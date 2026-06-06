package install

import (
	"os"
	"path/filepath"
)

// DoctorReport summarizes installed-state health (spec §9.1 doctor).
type DoctorReport struct {
	OK       []string // path
	Broken   []string // path: reason
	Emulated []string // path (informational)
}

// Doctor verifies each manifest record still matches what stark wrote: files present,
// merged keys/sentinels still present. Missing/altered managed state is reported.
func Doctor(dest, manifestFile string) (*DoctorReport, error) {
	m, err := LoadManifest(manifestFile)
	if err != nil {
		return nil, err
	}
	rep := &DoctorReport{}
	for _, r := range m.Records {
		abs := filepath.Join(dest, filepath.FromSlash(r.Path))
		cur := readOrEmpty(abs)
		switch r.Action {
		case ActionWriteFile:
			if _, err := os.Stat(abs); err != nil {
				rep.Broken = append(rep.Broken, r.Path+": file missing")
			} else {
				rep.OK = append(rep.OK, r.Path)
			}
		case ActionMergeTOMLKey:
			if cur == nil || !tableExists(cur, r.Key) {
				rep.Broken = append(rep.Broken, r.Path+": ["+r.Key+"] missing")
			} else {
				rep.OK = append(rep.OK, r.Path+"#"+r.Key)
			}
		case ActionMergeJSONKey:
			if cur == nil || !jsonKeyExists(cur, r.Key) {
				rep.Broken = append(rep.Broken, r.Path+": "+r.Key+" missing")
			} else {
				rep.OK = append(rep.OK, r.Path+"#"+r.Key)
			}
		case ActionSentinelBlock:
			if cur == nil || !sentinelExists(cur, r.Sentinel) {
				rep.Broken = append(rep.Broken, r.Path+": sentinel "+r.Sentinel+" missing")
			} else {
				rep.OK = append(rep.OK, r.Path+"#"+r.Sentinel)
			}
		}
	}
	return rep, nil
}
