package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type FileScanner struct {
	Err error
}

func (s *FileScanner) Scan(root string, match func(string) bool) map[string]fs.DirEntry {
	targets := make(map[string]fs.DirEntry)

	info, err := os.Lstat(root)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.Err = errors.Join(s.Err, err)
		}

		return targets
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return targets
	}

	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			s.Err = errors.Join(s.Err, err)

			return nil
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			s.Err = errors.Join(s.Err, err)

			return nil
		}

		if match == nil || match(filepath.ToSlash(relative)) {
			targets[path] = entry
		}

		return nil
	})

	return targets
}
