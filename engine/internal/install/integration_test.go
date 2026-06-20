package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/21-Stark-AI/stark-marketplace/engine/internal/indexio"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/installplan"
	"github.com/21-Stark-AI/stark-marketplace/engine/internal/model"
)

func planFor(t *testing.T, rt model.Runtime) *installplan.Plan {
	t.Helper()
	idx, err := indexio.LoadIndex("testdata/index.json")
	if err != nil {
		t.Fatal(err)
	}
	fa := installplan.NewFakeAdapter(map[string]string{
		"config.toml#mcp_servers.srv":  "command = \"node\"\nargs = [\"srv.js\"]\n",
		".mcp.json#mcpServers.srv":     `{"command":"node","args":["srv.js"]}`,
		"settings.json#mcpServers.srv": `{"command":"node","args":["srv.js"]}`,
		"GEMINI.md#":                   "emulated agent role block\n",
	})
	p, err := installplan.Compute(idx, "testdata/bundles", fa, "multi", "", model.TypeCommand, rt)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInstallVerifyRemoveAllRuntimes(t *testing.T) {
	for _, rt := range model.AllRuntimes() {
		rt := rt
		t.Run(string(rt), func(t *testing.T) {
			dest := t.TempDir()
			// seed a pre-existing user file per runtime to prove preservation
			switch rt {
			case model.RuntimeCodex:
				os.WriteFile(filepath.Join(dest, "config.toml"), []byte("# user\nlog=\"x\"\n"), 0o644)
			case model.RuntimeGemini:
				os.WriteFile(filepath.Join(dest, "GEMINI.md"), []byte("# user gemini\n\nintro\n"), 0o644)
				os.WriteFile(filepath.Join(dest, "settings.json"), []byte(`{"theme":"dark"}`), 0o644)
			case model.RuntimeClaude:
				os.WriteFile(filepath.Join(dest, ".mcp.json"), []byte(`{"existing":true}`), 0o644)
			}
			p := planFor(t, rt)
			res, err := Install(dest, p, Options{})
			if err != nil {
				t.Fatalf("install %s: %v", rt, err)
			}
			rep, _ := Doctor(dest, res.ManifestPath)
			if len(rep.Broken) != 0 {
				t.Fatalf("%s: doctor broken right after install: %+v", rt, rep.Broken)
			}
			// preservation checks
			assertUserContent(t, rt, dest)
			// managed sentinel block is actually written on the emulated-agent gemini path
			if rt == model.RuntimeGemini {
				g, _ := os.ReadFile(filepath.Join(dest, "GEMINI.md"))
				if !has(string(g), "<!-- stark:begin multi/agentmd -->") {
					t.Fatalf("gemini: sentinel block not written on install:\n%s", g)
				}
			}

			if err := Remove(dest, res.ManifestPath); err != nil {
				t.Fatalf("remove %s: %v", rt, err)
			}
			assertUserContent(t, rt, dest) // still intact after removal
			// and the managed sentinel block is excised on remove (not orphaned)
			if rt == model.RuntimeGemini {
				g, _ := os.ReadFile(filepath.Join(dest, "GEMINI.md"))
				if has(string(g), "stark:begin multi/agentmd") {
					t.Fatalf("gemini: sentinel block not excised on remove:\n%s", g)
				}
			}
			if _, err := os.Stat(res.ManifestPath); !os.IsNotExist(err) {
				t.Fatalf("%s: manifest should be gone after remove", rt)
			}
		})
	}
}

func assertUserContent(t *testing.T, rt model.Runtime, dest string) {
	t.Helper()
	read := func(p string) string { b, _ := os.ReadFile(filepath.Join(dest, p)); return string(b) }
	switch rt {
	case model.RuntimeCodex:
		if !has(read("config.toml"), "# user") {
			t.Fatalf("codex: user toml comment lost:\n%s", read("config.toml"))
		}
	case model.RuntimeGemini:
		if !has(read("GEMINI.md"), "# user gemini") {
			t.Fatalf("gemini: user md lost:\n%s", read("GEMINI.md"))
		}
		if !has(read("settings.json"), "dark") {
			t.Fatalf("gemini: settings sibling lost")
		}
	case model.RuntimeClaude:
		if !has(read(".mcp.json"), "existing") {
			t.Fatalf("claude: mcp sibling lost")
		}
	}
}

func has(s, sub string) bool { return indexOf(s, sub) >= 0 }
