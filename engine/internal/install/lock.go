package install

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
)

// Lock is an advisory file lock held while mutating a shared config file (spec §9.2).
type Lock struct{ fl *flock.Flock }

// AcquireLock takes an exclusive advisory lock for the dest-relative target `rel`. The lock
// file lives under <dest>/.stark/locks/ (stark's managed dir), NOT as a sibling of the user's
// config file — so we never truncate user content and never litter their runtime config tree
// (the whole .stark/ dir is stark-owned and removable).
func AcquireLock(dest, rel string) (*Lock, error) {
	lockDir := filepath.Join(starkDir(dest), "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, err
	}
	name := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(rel) + ".lock"
	fl := flock.New(filepath.Join(lockDir, name))
	if err := fl.Lock(); err != nil {
		return nil, err
	}
	return &Lock{fl: fl}, nil
}

func (l *Lock) Release() error { return l.fl.Unlock() }
