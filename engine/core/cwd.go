package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type CWD struct {
	path string
}

func CWDFromPath(path string) (*CWD, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return &CWD{path: cwd}, nil
	}

	var absPath string
	if !filepath.IsAbs(path) {
		var err error
		absPath, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
	} else {
		absPath = path
	}

	fileInfo, err := os.Stat(absPath)
	if err == nil && !fileInfo.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	return &CWD{path: absPath}, nil
}

func (c *CWD) Set(path string) error {
	if path == "" {
		return errors.New("path is required")
	}
	if c == nil {
		return errors.New("cwd is nil")
	}
	normalizedPath, err := CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	c.path = normalizedPath.path
	return nil
}

func (c *CWD) PathStr() string {
	if c == nil {
		return ""
	}
	return c.path
}

func (c *CWD) JoinAndCheck(path string) (string, error) {
	if c == nil {
		return "", errors.New("cwd is nil")
	}
	if c.path == "" {
		return "", errors.New("cwd is not set")
	}
	filename := filepath.Join(c.path, path)
	filename, err := filepath.Abs(filename)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	_, err = os.Stat(filename)
	if err != nil {
		return "", fmt.Errorf("file not found or inaccessible: %w", err)
	}
	return filename, nil
}

func (c *CWD) Validate() error {
	if c.path == "" {
		return errors.New("current working directory not set")
	}
	return nil
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
