package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to a temp file in the same dir, fsyncs it, then renames over the
// target (atomic on POSIX). Parent dirs are created.
func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".stark-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Digest returns the canonical content digest used for pre-validation (spec §9.4).
func Digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// PreValidateDigest asserts staged content matches an expected digest before commit.
func PreValidateDigest(content []byte, expected string) error {
	if expected == "" {
		return nil
	}
	if got := Digest(content); got != expected {
		return fmt.Errorf("digest mismatch: got %s want %s", got, expected)
	}
	return nil
}
