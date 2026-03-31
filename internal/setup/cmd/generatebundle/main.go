package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "generate bundled skills mirror: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("resolve generator source location")
	}

	toolDir := filepath.Dir(file)
	setupDir := filepath.Clean(filepath.Join(toolDir, "..", ".."))
	repoRoot := filepath.Clean(filepath.Join(setupDir, "..", ".."))
	sourceDir := filepath.Join(repoRoot, "skills")
	destDir := filepath.Join(setupDir, "assets", "skills")

	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("reset destination %s: %w", destDir, err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create destination root %s: %w", destDir, err)
	}

	sourceRoot, err := os.OpenRoot(sourceDir)
	if err != nil {
		return fmt.Errorf("open source root %s: %w", sourceDir, err)
	}
	defer sourceRoot.Close()

	destRoot, err := os.OpenRoot(destDir)
	if err != nil {
		return fmt.Errorf("open destination root %s: %w", destDir, err)
	}
	defer destRoot.Close()

	return fs.WalkDir(sourceRoot.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		targetPath := filepath.FromSlash(path)
		if entry.IsDir() {
			return destRoot.MkdirAll(targetPath, 0o755)
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := destRoot.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create parent for %s: %w", targetPath, err)
		}

		content, err := sourceRoot.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := destRoot.WriteFile(targetPath, content, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		return nil
	})
}
