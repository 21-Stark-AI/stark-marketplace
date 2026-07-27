package digest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
)

// Files returns a deterministic content digest over a set of repo-relative files.
//
// This is the version-bump gate for a bundle's VENDORED PLUGIN ASSETS
// (`vendor/plugins/<bundle>/**`). Those are not artifacts, so they carry no
// `digest.Source` and never appeared in the index — meaning a change confined to a
// plugin's `tools/**` changed no artifact digest, `check-bumps` reported clean, and the
// bundle shipped modified content under an unchanged version. Consumers already on that
// version never re-fetch, so the change was invisible on every installed machine.
// (Hit live 2026-07-27: a stark-gh `lib/git.ts` fix shipped un-bumped at 0.1.10.)
//
// Content is normalized to LF exactly as `build` normalizes it before writing to
// `dist/`, so the digest tracks what actually ships rather than the checkout's line
// endings. Paths are hashed alongside content, so a pure rename is a change.
func Files(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		// Length-prefix both fields so no concatenation of (path, content) pairs can
		// collide with a different set that happens to share the same byte stream.
		writeField(h, []byte(filepath.ToSlash(name)))
		writeField(h, toLF(files[name]))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// Dir walks a directory and returns `Files` over its regular files, keyed by
// slash-separated path relative to root. A missing directory is not an error — it
// yields the empty-set digest, so a bundle that has no vendored plugin assets stays
// stable instead of erroring the gate.
func Dir(root string) (string, error) {
	files := map[string][]byte{}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return Files(files), nil
		}
		return "", err
	}
	if !info.IsDir() {
		return "", &os.PathError{Op: "digest.Dir", Path: root, Err: os.ErrInvalid}
	}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return "", err
	}
	return Files(files), nil
}

func writeField(h interface{ Write([]byte) (int, error) }, b []byte) {
	var lenBuf [8]byte
	n := uint64(len(b))
	for i := 0; i < 8; i++ {
		lenBuf[i] = byte(n >> (8 * (7 - i)))
	}
	_, _ = h.Write(lenBuf[:])
	_, _ = h.Write(b)
}

// toLF mirrors build.toLF: dist content is LF-normalized before it is written, so the
// gate must hash the same normalized bytes.
func toLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
}
