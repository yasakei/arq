package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func writeFiles(dir string, files map[string]string) (err error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := checkDirectory(root); err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if !filepath.IsLocal(name) {
			return fmt.Errorf("invalid scaffold path %q", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := checkDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	var createdFiles, createdDirs []string
	defer func() {
		if err != nil {
			for i := len(createdFiles) - 1; i >= 0; i-- {
				_ = os.Remove(createdFiles[i])
			}
			for i := len(createdDirs) - 1; i >= 0; i-- {
				_ = os.Remove(createdDirs[i])
			}
		}
	}()
	for _, name := range names {
		path := filepath.Join(root, name)
		if err = makeDirectory(filepath.Dir(path), &createdDirs); err != nil {
			return err
		}
		var f *os.File
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		createdFiles = append(createdFiles, path)
		_, err = f.WriteString(files[name])
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func checkDirectory(path string) error {
	parent := filepath.Dir(path)
	if parent != path {
		if err := checkDirectory(parent); err != nil {
			return err
		}
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory or is a symlink", path)
	}
	return nil
}

func makeDirectory(path string, created *[]string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory or is a symlink", path)
		}
		return checkDirectory(path)
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := makeDirectory(filepath.Dir(path), created); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0755); err != nil {
		return err
	}
	*created = append(*created, path)
	return nil
}
