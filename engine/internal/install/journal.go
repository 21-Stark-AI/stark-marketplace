package install

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// JournalEntry is one intended mutation, written BEFORE it happens (spec §9.4).
type JournalEntry struct {
	Op       string `json:"op"`   // write | mergeJSON | mergeTOML | sentinel
	Path     string `json:"path"` // dest-relative
	Key      string `json:"key,omitempty"`
	Sentinel string `json:"sentinel,omitempty"`
}

// Journal is an append-only, fsynced write-ahead log.
type Journal struct {
	f *os.File
	w *bufio.Writer
}

const journalCommitMarker = "__COMMIT__"

func OpenJournal(path string) (*Journal, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	return &Journal{f: f, w: bufio.NewWriter(f)}, nil
}

// Record appends an entry and fsyncs immediately (write-ahead guarantee).
func (j *Journal) Record(e JournalEntry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := j.w.Write(append(b, '\n')); err != nil {
		return err
	}
	if err := j.w.Flush(); err != nil {
		return err
	}
	return j.f.Sync()
}

// Commit writes the commit marker + fsyncs, signalling a clean finish.
func (j *Journal) Commit() error {
	if _, err := j.w.WriteString(journalCommitMarker + "\n"); err != nil {
		return err
	}
	if err := j.w.Flush(); err != nil {
		return err
	}
	return j.f.Sync()
}

func (j *Journal) Close() error {
	ferr := j.w.Flush()
	cerr := j.f.Close()
	if ferr != nil {
		return ferr
	}
	return cerr
}

// ReadJournal returns recorded entries and whether the run committed cleanly.
func ReadJournal(path string) (entries []JournalEntry, committed bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if line == journalCommitMarker {
			committed = true
			continue
		}
		var e JournalEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, false, err
		}
		entries = append(entries, e)
	}
	return entries, committed, sc.Err()
}
