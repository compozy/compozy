package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type PathCWD struct {
	Path string `json:"path" yaml:"path" mapstructure:"path"`
}

func CWDFromPath(path string) (*PathCWD, error) {
	if path == "" {
		CWD, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return &PathCWD{Path: CWD}, nil
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

	return &PathCWD{Path: absPath}, nil
}

func (c *PathCWD) Set(path string) error {
	if path == "" {
		return errors.New("path is required")
	}
	if c == nil {
		return errors.New("CWD is nil")
	}
	normalizedPath, err := CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	c.Path = normalizedPath.Path
	return nil
}

func (c *PathCWD) PathStr() string {
	if c == nil {
		return ""
	}
	return c.Path
}

func (c *PathCWD) JoinAndCheck(path string) (string, error) {
	if c == nil {
		return "", errors.New("CWD is nil")
	}
	if c.Path == "" {
		return "", errors.New("CWD is not set")
	}
	filename := filepath.Join(c.Path, path)
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

func (c *PathCWD) Validate() error {
	if c.Path == "" {
		return errors.New("current working directory not set")
	}
	return nil
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
