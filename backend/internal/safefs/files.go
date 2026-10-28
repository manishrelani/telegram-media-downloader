// Package safefs confines local file operations to a selected directory.
package safefs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Relative(root, path string) (string, error) {
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("path is outside the selected folder")
	}
	return rel, nil
}

// Check rejects links rather than trusting a lexical prefix. Root operations
// provide the confinement even if a component is replaced after this check.
func Check(root *os.Root, rel string) error {
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("unsafe relative path")
	}
	current := "."
	for _, part := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in download paths")
		}
	}
	return nil
}
func Open(root, path string) (*os.Root, string, error) {
	rel, err := Relative(root, path)
	if err != nil {
		return nil, "", err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, "", err
	}
	if err := Check(r, rel); err != nil {
		r.Close()
		return nil, "", err
	}
	return r, rel, nil
}
func Ensure(root, path string) error {
	r, rel, err := Open(root, path)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.MkdirAll(rel, 0700); err != nil {
		return err
	}
	return Check(r, rel)
}
func WriteFile(root, path string, data []byte) error {
	r, rel, err := Open(root, path)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := r.OpenFile(rel, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
