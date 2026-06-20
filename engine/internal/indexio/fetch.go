package indexio

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Fetcher pulls the committed index/detail from the PRIVATE marketplace repo via the
// authenticated GitHub Contents API. There is NO anonymous raw-URL path: the repo is private,
// so every request carries a token. APIBase defaults to https://api.github.com when empty.
type Fetcher struct {
	APIBase  string // e.g. https://api.github.com (overridden by tests)
	Owner    string // 21-Stark-AI
	Repo     string // stark-marketplace
	Ref      string // branch/tag/sha, e.g. "main"
	BasePath string // path to the published index dir, e.g. "dist/claude"
	Token    string // GitHub token (see resolveToken)
	HTTP     *http.Client
}

// DefaultFetcher builds a Fetcher for the published claude index on main, resolving the token
// from gh/env. Returns an error if no token is available (private repo → token required).
func DefaultFetcher() (*Fetcher, error) {
	tok := resolveToken(ghAuthToken)
	if tok == "" {
		return nil, fmt.Errorf("no GitHub token: set GITHUB_TOKEN or run `gh auth login` (the marketplace repo is private)")
	}
	return &Fetcher{
		APIBase:  "https://api.github.com",
		Owner:    "21-Stark-AI",
		Repo:     "stark-marketplace",
		Ref:      "main",
		BasePath: "dist/claude",
		Token:    tok,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// resolveToken prefers GITHUB_TOKEN; falls back to `gh auth token` via the injected getter.
func resolveToken(ghToken func() (string, error)) string {
	if v := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); v != "" {
		return v
	}
	if ghToken != nil {
		if v, err := ghToken(); err == nil {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ghAuthToken shells out to `gh auth token` (the user's existing GitHub CLI auth).
func ghAuthToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (f *Fetcher) apiBase() string {
	if f.APIBase != "" {
		return strings.TrimRight(f.APIBase, "/")
	}
	return "https://api.github.com"
}

func (f *Fetcher) httpc() *http.Client {
	if f.HTTP != nil {
		return f.HTTP
	}
	return http.DefaultClient
}

// fetchRaw GETs one repo file's raw bytes through the Contents API.
func (f *Fetcher) fetchRaw(repoPath string) ([]byte, error) {
	u := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		f.apiBase(), f.Owner, f.Repo, repoPath, url.QueryEscape(f.Ref))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+f.Token)
	req.Header.Set("Accept", "application/vnd.github.raw")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := f.httpc().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d: %s", repoPath, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// FetchIndex fetches + asserts the remote index.json (same parse/assert path as LoadIndex).
func (f *Fetcher) FetchIndex() (*Index, error) {
	data, err := f.fetchRaw(f.BasePath + "/index.json")
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse remote index: %w", err)
	}
	if err := AssertSchemaVersion(idx.SchemaVersion); err != nil {
		return nil, err
	}
	return &idx, nil
}

// FetchBundleDetail fetches + asserts a remote bundles/<name>.json.
func (f *Fetcher) FetchBundleDetail(name string) (*BundleDetail, error) {
	data, err := f.fetchRaw(f.BasePath + "/bundles/" + name + ".json")
	if err != nil {
		return nil, err
	}
	var d BundleDetail
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse remote bundle detail %s: %w", name, err)
	}
	if err := AssertSchemaVersion(d.SchemaVersion); err != nil {
		return nil, err
	}
	return &d, nil
}
