package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func provisionBundledRuntime(sourcePath string, targetPath string) (returnErr error) {
	sourcePath = filepath.Clean(sourcePath)
	targetPath = filepath.Clean(targetPath)
	if sourcePath == targetPath {
		return nil
	}
	if !regularFileExists(sourcePath) {
		return fmt.Errorf("cli: bundled runtime payload %q is missing or is not a regular file", sourcePath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return fmt.Errorf("cli: create runtime bin directory: %w", err)
	}
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".compozy-bootstrap-*")
	if err != nil {
		return fmt.Errorf("cli: create runtime provision temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempOpen := true
	defer func() {
		var closeErr error
		if tempOpen {
			closeErr = tempFile.Close()
		}
		removeErr := os.Remove(tempPath)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		returnErr = errors.Join(returnErr, closeErr, removeErr)
	}()
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("cli: open bundled runtime payload: %w", err)
	}
	if _, err := io.Copy(tempFile, source); err != nil {
		closeErr := source.Close()
		return errors.Join(fmt.Errorf("cli: copy bundled runtime payload: %w", err), closeErr)
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("cli: close bundled runtime payload: %w", err)
	}
	if err := tempFile.Chmod(0o700); err != nil {
		return fmt.Errorf("cli: make provisioned runtime executable: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("cli: sync provisioned runtime: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("cli: close provisioned runtime: %w", err)
	}
	tempOpen = false
	if err := os.Link(tempPath, targetPath); err != nil {
		if errors.Is(err, os.ErrExist) && regularFileExists(targetPath) {
			return nil
		}
		return fmt.Errorf("cli: publish provisioned runtime: %w", err)
	}
	dir, err := os.Open(filepath.Dir(targetPath))
	if err != nil {
		return fmt.Errorf("cli: open runtime bin directory: %w", err)
	}
	if err := dir.Sync(); err != nil {
		closeErr := dir.Close()
		return errors.Join(fmt.Errorf("cli: sync runtime bin directory: %w", err), closeErr)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("cli: close runtime bin directory: %w", err)
	}
	return nil
}

func regularFileExists(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}
