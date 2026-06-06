package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/GetEvinced/stark-marketplace/engine/internal/installplan"
	"github.com/GetEvinced/stark-marketplace/engine/internal/model"
)

// Options controls executor behavior.
type Options struct {
	Force bool // overwrite unmanaged collisions
}

// Result reports what Install did.
type Result struct {
	ManifestPath string
	Written      int
	Merged       int
}

// ConflictError is exit code 4 at the CLI boundary (spec §9.8).
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

func starkDir(dest string) string    { return filepath.Join(dest, ".stark") }
func journalPath(dest string) string { return filepath.Join(starkDir(dest), "install.journal") }
func manifestPath(dest, rt string) string {
	return filepath.Join(starkDir(dest), "manifest-"+rt+".json")
}

// Install applies a plan to dest atomically with a write-ahead journal.
func Install(dest string, p *installplan.Plan, o Options) (*Result, error) {
	if err := os.MkdirAll(starkDir(dest), 0o755); err != nil {
		return nil, err
	}
	// 1. recover any crashed prior run BEFORE truncating its journal. A re-run is the natural
	// reaction to a failed install; without this, OpenJournal's O_TRUNC would erase the
	// uncommitted journal and orphan the prior run's partial mutations forever (§9.4).
	if err := Repair(dest); err != nil {
		return nil, fmt.Errorf("recover prior incomplete install: %w", err)
	}

	mPath := manifestPath(dest, string(p.Runtime))
	manifest := &Manifest{SchemaVersion: 1, Runtime: p.Runtime}
	if prev, err := LoadManifest(mPath); err == nil {
		manifest = prev // keep prior records; merge actions are idempotent, file actions overwrite
	}

	// 2. collision pre-check (refuse before mutating anything)
	if !o.Force {
		if err := preflightCollisions(dest, p, manifest); err != nil {
			return nil, err
		}
	}

	// 3. open journal (write-ahead)
	j, err := OpenJournal(journalPath(dest))
	if err != nil {
		return nil, err
	}
	defer j.Close()

	res := &Result{ManifestPath: mPath}
	for _, step := range p.Steps {
		for _, f := range step.Files {
			rec, err := applyFile(dest, j, step, f)
			if err != nil {
				return nil, err
			}
			upsertRecord(manifest, rec)
			if f.Kind == "file" {
				res.Written++
			} else {
				res.Merged++
			}
		}
	}

	// 4. mark journal committed FIRST, then persist the manifest. If we crash between the two,
	// Repair sees a committed journal and leaves the (intact) files in place — never a manifest
	// that claims content a rollback already removed (§9.4 self-consistency).
	if err := j.Commit(); err != nil {
		return nil, err
	}
	if err := SaveManifest(mPath, manifest); err != nil {
		return nil, err
	}
	_ = os.Remove(journalPath(dest)) // clean finish: drop the journal
	return res, nil
}

// applyFile writes/merges one AdaptedFile and returns its manifest Record.
func applyFile(dest string, j *Journal, step installplan.Step, f installplan.AdaptedFile) (Record, error) {
	abs := filepath.Join(dest, filepath.FromSlash(f.Path))
	rec := Record{Bundle: step.Bundle, Name: step.Name, Type: step.Type, Path: f.Path, Key: f.Key, Sentinel: f.Sentinel}

	switch f.Kind {
	case "file":
		if err := j.Record(JournalEntry{Op: "write", Path: f.Path}); err != nil {
			return rec, err
		}
		if err := writeVerified(abs, []byte(f.Payload)); err != nil {
			return rec, err
		}
		rec.Action = ActionWriteFile
		rec.Digest = Digest([]byte(f.Payload))

	case "mergeTOMLKey":
		lk, err := AcquireLock(dest, f.Path)
		if err != nil {
			return rec, err
		}
		defer lk.Release()
		if err := j.Record(JournalEntry{Op: "mergeTOML", Path: f.Path, Key: f.Key}); err != nil {
			return rec, err
		}
		cur := readOrEmpty(abs)
		out, _, err := MergeTOMLKey(cur, f.Key, f.Payload)
		if err != nil {
			return rec, err
		}
		if err := writeVerified(abs, out); err != nil {
			return rec, err
		}
		rec.Action = ActionMergeTOMLKey
		rec.Digest = Digest([]byte(f.Payload))

	case "mergeJSONKey":
		lk, err := AcquireLock(dest, f.Path)
		if err != nil {
			return rec, err
		}
		defer lk.Release()
		if err := j.Record(JournalEntry{Op: "mergeJSON", Path: f.Path, Key: f.Key}); err != nil {
			return rec, err
		}
		cur := readOrEmpty(abs)
		var val any
		if err := json.Unmarshal([]byte(f.Payload), &val); err != nil {
			return rec, fmt.Errorf("mergeJSONKey payload not JSON: %w", err)
		}
		out, _, err := MergeJSONKey(cur, f.Key, val)
		if err != nil {
			return rec, err
		}
		if err := writeVerified(abs, out); err != nil {
			return rec, err
		}
		rec.Action = ActionMergeJSONKey
		rec.Digest = Digest([]byte(f.Payload))

	case "sentinel":
		lk, err := AcquireLock(dest, f.Path)
		if err != nil {
			return rec, err
		}
		defer lk.Release()
		if err := j.Record(JournalEntry{Op: "sentinel", Path: f.Path, Sentinel: f.Sentinel}); err != nil {
			return rec, err
		}
		cur := readOrEmpty(abs)
		out, _, err := MergeSentinel(cur, f.Sentinel, f.Payload)
		if err != nil {
			return rec, err
		}
		if err := writeVerified(abs, out); err != nil {
			return rec, err
		}
		rec.Action = ActionSentinelBlock
		rec.Digest = Digest([]byte(f.Payload))

	default:
		return rec, fmt.Errorf("unknown adapted-file kind %q", f.Kind)
	}
	return rec, nil
}

// writeVerified atomically writes data, then reads it back and verifies the on-disk bytes match
// (spec §9.4 integrity). A mismatch returns a *DigestError → exit 3 at the CLI boundary.
func writeVerified(abs string, data []byte) error {
	if err := AtomicWrite(abs, data, 0o644); err != nil {
		return err
	}
	got, err := readBack(abs)
	if err != nil {
		return err
	}
	return PreValidateDigest(got, Digest(data))
}

// preflightCollisions refuses if a managed target collides with UNMANAGED pre-existing
// content not recorded in our manifest (spec §9.2).
func preflightCollisions(dest string, p *installplan.Plan, manifest *Manifest) error {
	known := map[string]bool{} // path|key|sentinel already ours
	for _, r := range manifest.Records {
		known[r.Path+"|"+r.Key+"|"+r.Sentinel] = true
	}
	for _, step := range p.Steps {
		for _, f := range step.Files {
			abs := filepath.Join(dest, filepath.FromSlash(f.Path))
			cur := readOrEmpty(abs)
			id := f.Path + "|" + f.Key + "|" + f.Sentinel
			if known[id] {
				continue // we own it; replace is fine
			}
			switch f.Kind {
			case "mergeTOMLKey":
				if tableExists(cur, f.Key) {
					return &ConflictError{fmt.Sprintf("%s already has unmanaged [%s] — use --force", f.Path, f.Key)}
				}
			case "mergeJSONKey":
				if jsonKeyExists(cur, f.Key) {
					return &ConflictError{fmt.Sprintf("%s already has unmanaged %s — use --force", f.Path, f.Key)}
				}
			case "sentinel":
				if sentinelExists(cur, f.Sentinel) {
					return &ConflictError{fmt.Sprintf("%s already has block %s — use --force", f.Path, f.Sentinel)}
				}
			case "file":
				if _, err := os.Stat(abs); err == nil {
					return &ConflictError{fmt.Sprintf("%s already exists — use --force", f.Path)}
				}
			}
		}
	}
	return nil
}

func upsertRecord(m *Manifest, r Record) {
	for i := range m.Records {
		if m.Records[i].Path == r.Path && m.Records[i].Key == r.Key && m.Records[i].Sentinel == r.Sentinel {
			m.Records[i] = r
			return
		}
	}
	m.Add(r)
}

func readOrEmpty(abs string) []byte {
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	return b
}

// Remove excises exactly what a manifest installed (spec §9.2).
func Remove(dest, manifestFile string) error {
	m, err := LoadManifest(manifestFile)
	if err != nil {
		return err
	}
	recs := append([]Record(nil), m.Records...)
	sort.Slice(recs, func(i, j int) bool { return recs[i].Path > recs[j].Path })
	for _, r := range recs {
		abs := filepath.Join(dest, filepath.FromSlash(r.Path))
		switch r.Action {
		case ActionWriteFile:
			_ = os.Remove(abs)
		case ActionMergeTOMLKey:
			if cur := readOrEmpty(abs); cur != nil {
				_ = AtomicWrite(abs, removeTOMLTable(cur, r.Key), 0o644)
			}
		case ActionMergeJSONKey:
			if cur := readOrEmpty(abs); cur != nil {
				_ = AtomicWrite(abs, removeJSONKey(cur, r.Key), 0o644)
			}
		case ActionSentinelBlock:
			if cur := readOrEmpty(abs); cur != nil {
				_ = AtomicWrite(abs, removeSentinel(cur, r.Sentinel), 0o644)
			}
		}
	}
	return os.Remove(manifestFile)
}

// Repair recovers a crashed/partial install using the uncommitted journal (spec §9.4):
// any journaled mutation is rolled back to a clean state, then the journal is cleared.
func Repair(dest string) error {
	jp := journalPath(dest)
	entries, committed, err := ReadJournal(jp)
	if os.IsNotExist(err) {
		return nil // nothing to repair
	}
	if err != nil {
		return err
	}
	if committed {
		return os.Remove(jp) // install finished; just drop the journal
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		abs := filepath.Join(dest, filepath.FromSlash(e.Path))
		switch e.Op {
		case "write":
			_ = os.Remove(abs)
		case "mergeTOML":
			if cur := readOrEmpty(abs); cur != nil {
				_ = AtomicWrite(abs, removeTOMLTable(cur, e.Key), 0o644)
			}
		case "mergeJSON":
			if cur := readOrEmpty(abs); cur != nil {
				_ = AtomicWrite(abs, removeJSONKey(cur, e.Key), 0o644)
			}
		case "sentinel":
			if cur := readOrEmpty(abs); cur != nil {
				_ = AtomicWrite(abs, removeSentinel(cur, e.Sentinel), 0o644)
			}
		}
	}
	return os.Remove(jp)
}

var _ = model.RuntimeCodex // model import anchor
