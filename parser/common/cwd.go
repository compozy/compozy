package common

import (
	"errors"
	"path/filepath"
)

var ErrCWDNotSet = errors.New("current working directory not set")

// CWD represents a common working directory structure
type CWD struct {
	path string
}

// NewCWD creates a new CWD instance
func NewCWD(path string) *CWD {
	return &CWD{
		path: path,
	}
}

// Set sets the current working directory
func (c *CWD) Set(path string) {
	c.path = path
}

// Get returns the current working directory
func (c *CWD) Get() string {
	return c.path
}

// Join joins the current working directory with the given path
func (c *CWD) Join(path string) string {
	return filepath.Join(c.path, path)
}

// Validate checks if the working directory is set
func (c *CWD) Validate() error {
	if c.path == "" {
		return ErrCWDNotSet
	}
	return nil
}
