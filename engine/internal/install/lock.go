package install

import "github.com/gofrs/flock"

// Lock is an advisory file lock held on a shared config file during mutation (spec §9.2).
type Lock struct{ fl *flock.Flock }

// AcquireLock takes an exclusive advisory lock on <path>.stark.lock (sibling, never the
// file itself, so we never truncate user content).
func AcquireLock(path string) (*Lock, error) {
	fl := flock.New(path + ".stark.lock")
	if err := fl.Lock(); err != nil {
		return nil, err
	}
	return &Lock{fl: fl}, nil
}

func (l *Lock) Release() error { return l.fl.Unlock() }
