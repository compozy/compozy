package mcp

import (
	"context"

	"errors"

	"os"
	"path/filepath"

	"strings"
)

func constantHashEqual(left string, right string) bool {
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for i := range left {
		diff |= left[i] ^ right[i]
	}
	return diff == 0
}

func normalizeExecutablePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("executable path is required")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	evaluated, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(evaluated), nil
}

func sameExecutablePath(expected string, actual string) (bool, error) {
	expectedPath, err := normalizeExecutablePath(expected)
	if err != nil {
		return false, err
	}
	actualPath, err := normalizeExecutablePath(actual)
	if err != nil {
		return false, err
	}
	if actualPath == expectedPath {
		return true, nil
	}
	expectedInfo, err := os.Stat(expectedPath)
	if err != nil {
		return false, err
	}
	actualInfo, err := os.Stat(actualPath)
	if err != nil {
		return false, err
	}
	return os.SameFile(expectedInfo, actualInfo), nil
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return errors.New("mcp: context is required")
	}
	return ctx.Err()
}
