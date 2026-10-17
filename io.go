package mibig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ReadFile reads and decodes the MIBiG entry in the named file.
func ReadFile(name string) (*Entry, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	entry, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return entry, nil
}

// WriteFile encodes the entry with Marshal and writes it to the named file.
// It writes a temporary file in the same directory, syncs it and renames it,
// so readers see either the old or the new file on Unix systems. An existing
// file keeps its permissions; new files are created with mode 0644.
func WriteFile(name string, e *Entry) error {
	data, err := Marshal(e)
	if err != nil {
		return err
	}

	mode := fs.FileMode(0o644)
	if info, err := os.Stat(name); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after the rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
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
	return os.Rename(tmp.Name(), name)
}
