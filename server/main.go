// Command server is the static origin for the stark-marketplace web registry.
//
// It serves the Vite SPA shell plus the engine-generated index.json and
// bundles/*.json from WEBROOT. It is sized to run on Cloud Run behind the
// platform load balancer (marketplace.21stark.com): the LB terminates TLS at the
// edge, so this process does no auth — it only serves files.
//
// The SPA uses HashRouter with relative data fetches (./index.json,
// ./bundles/<name>.json), so the document is always served at "/". There is no
// server-side route rewriting to do; unknown paths fall back to the app shell
// purely as a defensive measure.
//
// Caching mirrors the spec §10 atomic-unit model: content-hashed assets/ are
// immutable and long-cached; the shell, index.json and bundles/ are no-cache so
// a new deploy is picked up immediately.
//
// Every response carries a baseline of security headers (HSTS, CSP, frame
// blockers, nosniff, Referrer-Policy, Permissions-Policy). The platform LB
// fronts identity at the edge; these headers are the app-layer defense the
// proxy doesn't add.
package main

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func main() {
	root := env("WEBROOT", "./public")
	port := env("PORT", "8080")

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler(os.DirFS(root)),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("stark-marketplace static server: root=%s port=%s", root, port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// CSP locked to first-party assets. 'unsafe-inline' on style-src is the
// minimum Vite needs for its FOUC-guard inline <style>; scripts stay strict
// 'self' because Vite emits hashed bundles, not inline scripts. frame-ancestors
// 'none' + X-Frame-Options DENY block embedding; base-uri 'self' prevents
// <base> hijacks; form-action 'self' bounds outbound posts; connect-src 'self'
// restricts fetch/XHR to same-origin (the SPA only fetches ./index.json +
// ./bundles/*.json).
const contentSecurityPolicy = "default-src 'self'; " +
	"img-src 'self' data:; " +
	"style-src 'self' 'unsafe-inline'; " +
	"script-src 'self'; " +
	"connect-src 'self'; " +
	"font-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

// securityHeaders sets the response baseline before the file handler can write
// anything. Applied to every response (including method-not-allowed, /healthz,
// and the SPA-shell fallback) — these headers are independent of cache or auth
// state.
func securityHeaders(h http.Header) {
	// HSTS: 2 years, include subdomains, preload-eligible. The Cloud Run +
	// platform LB front this origin behind a managed *.21stark.com cert, so
	// every real request is TLS — clients can safely cache HSTS.
	h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
	h.Set("Content-Security-Policy", contentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	h.Set("X-Frame-Options", "DENY") // defense-in-depth alongside CSP frame-ancestors 'none'
}

// handler serves files from fsys with cache headers, a /healthz probe, and an
// app-shell fallback for unknown paths.
func handler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w.Header())

		if r.URL.Path == "/healthz" {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// path.Clean on a rooted path collapses any ../ so a request can never
		// escape fsys; os.DirFS rejects the rest.
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
			w.Header().Set("Cache-Control", cacheControl(name))
			http.ServeFileFS(w, r, fsys, name)
			return
		}

		// Unknown path → hash-routed app shell. Never cache the shell.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, fsys, "index.html")
	})
}

// cacheControl long-caches content-hashed assets and leaves everything else
// (the shell, index.json, bundles/) revalidated on every load.
func cacheControl(name string) string {
	if strings.HasPrefix(name, "assets/") {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}
