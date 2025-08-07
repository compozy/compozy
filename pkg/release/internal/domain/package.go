package domain

// Package represents an NPM package to be updated.

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Private bool   `json:"private"`
	Path    string `json:"-"` // Path is not part of package.json
}
