package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteReplacesAndSyncs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "file.txt")
	if err := AtomicWrite(p, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "hello\n" {
		t.Fatalf("got %q", got)
	}
	if err := AtomicWrite(p, []byte("world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(p)
	if string(got) != "world\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJournalReplay(t *testing.T) {
	dir := t.TempDir()
	jp := filepath.Join(dir, "install.journal")
	j, err := OpenJournal(jp)
	if err != nil {
		t.Fatal(err)
	}
	j.Record(JournalEntry{Op: "write", Path: "a.md"})
	j.Record(JournalEntry{Op: "write", Path: "config.toml", Key: "mcp_servers.x"})
	j.Close()

	entries, committed, err := ReadJournal(jp)
	if err != nil {
		t.Fatal(err)
	}
	if committed {
		t.Fatal("journal must not be marked committed")
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}

	j2, _ := OpenJournal(jp)
	j2.Commit()
	j2.Close()
	_, committed2, _ := ReadJournal(jp)
	if !committed2 {
		t.Fatal("journal should now be committed")
	}
}
