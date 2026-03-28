//go:build mage

package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const golangciLintVersion = "v1.64.8"

var Default = Verify

func Deps() error {
	return sh.RunV("go", "mod", "tidy")
}

func Fmt() error {
	files, err := goFiles(".")
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"-w"}, files...)
	return sh.RunV("gofmt", args...)
}

func Lint() error {
	return sh.RunV(
		"go",
		"run",
		"github.com/golangci/golangci-lint/cmd/golangci-lint@"+golangciLintVersion,
		"run",
		"./...",
	)
}

func Test() error {
	return sh.RunV("go", "test", "-race", "./...")
}

func Build() error {
	return sh.RunV("go", "build", "./...")
}

func Verify() {
	mg.SerialDeps(Fmt, Lint, Test, Build)
}

func goFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}
