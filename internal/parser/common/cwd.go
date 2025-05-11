package common

import (
	"errors"
	"os"
	"path/filepath"
)

type CWD struct {
	Path string
}

func CWDFromPath(path string) (*CWD, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return &CWD{Path: cwd}, nil
	}
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		return &CWD{Path: absPath}, nil
	}
	return &CWD{Path: path}, nil
}

func (c *CWD) Set(path string) {
	c.Path = path
}

func (c *CWD) Get() string {
	return c.Path
}

func (c *CWD) Join(path string) string {
	return filepath.Join(c.Path, path)
}

func (c *CWD) Validate() error {
	if c.Path == "" {
		return errors.New("current working directory not set")
	}
	return nil
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
