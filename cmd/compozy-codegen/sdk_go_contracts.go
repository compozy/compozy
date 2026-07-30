package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/codegen/sdkgo"
	"github.com/compozy/compozy/internal/fileutil"
)

const defaultSDKGoContractsPath = "sdk/go/contracts"

func sdkGoContractsPathFor(typeScriptPath string) string {
	if filepath.Clean(typeScriptPath) == filepath.Clean(defaultSDKContractsPath) {
		return defaultSDKGoContractsPath
	}
	return filepath.Join(filepath.Dir(typeScriptPath), "go-contracts")
}

func generateSDKGoContracts() (sdkgo.Result, error) {
	result, err := sdkgo.Generate()
	if err != nil {
		return sdkgo.Result{}, fmt.Errorf("generate Go SDK contracts: %w", err)
	}
	return result, nil
}

func writeSDKGoContracts(path string) error {
	result, err := generateSDKGoContracts()
	if err != nil {
		return err
	}
	if err := publishGeneratedTree(path, result.Files); err != nil {
		return fmt.Errorf("write Go SDK contracts to %q: %w", path, err)
	}
	return nil
}

func checkSDKGoContracts(path string) error {
	return checkSDKGoContractsWith(path, generateSDKGoContracts)
}

func checkSDKGoContractsWith(path string, generate func() (sdkgo.Result, error)) error {
	result, err := generate()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s is missing; run codegen", path)
		}
		return fmt.Errorf("read generated Go SDK contracts %q: %w", path, err)
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf(
				"%s: %w; generated contract tree contains directory %s",
				path,
				ErrStaleGeneratedFile,
				entry.Name(),
			)
		}
		actual = append(actual, entry.Name())
	}
	sort.Strings(actual)
	if expected := result.FileNames(); !equalStrings(actual, expected) {
		return fmt.Errorf("%s: %w; file set is %v, want %v", path, ErrStaleGeneratedFile, actual, expected)
	}
	for _, name := range actual {
		if err := checkFile(filepath.Join(path, name), result.Files[name]); err != nil {
			return err
		}
	}
	return nil
}

func publishGeneratedTree(path string, files map[string][]byte) error {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "." || cleanPath == "" {
		return fmt.Errorf("generated tree path is required")
	}
	parent := filepath.Dir(cleanPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create generated tree parent %q: %w", parent, err)
	}
	temporary, err := stageGeneratedTree(parent, files)
	if err != nil {
		return err
	}
	backup, err := backupGeneratedTree(cleanPath, parent)
	if err != nil {
		return cleanupGeneratedTree(temporary, err)
	}

	if err := os.Rename(temporary, cleanPath); err != nil {
		restoreErr := restoreGeneratedTree(cleanPath, backup)
		publishErr := fmt.Errorf("publish generated tree %q: %w; restore: %v", cleanPath, err, restoreErr)
		return cleanupGeneratedTree(temporary, publishErr)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove generated tree backup %q: %w", backup, err)
		}
	}
	return nil
}

func stageGeneratedTree(parent string, files map[string][]byte) (string, error) {
	temporary, err := os.MkdirTemp(parent, ".sdk-go-contracts-")
	if err != nil {
		return "", fmt.Errorf("create generated tree staging directory: %w", err)
	}
	for name, content := range files {
		if filepath.Base(name) != name {
			return "", cleanupGeneratedTree(
				temporary,
				fmt.Errorf("generated file name %q must be a base name", name),
			)
		}
		if err := fileutil.AtomicWriteFile(filepath.Join(temporary, name), content, 0o600); err != nil {
			return "", cleanupGeneratedTree(
				temporary,
				fmt.Errorf("stage generated file %q: %w", name, err),
			)
		}
	}
	return temporary, nil
}

func backupGeneratedTree(path string, parent string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("inspect generated tree %q: %w", path, err)
	}
	backup, err := reserveBackupPath(parent)
	if err != nil {
		return "", err
	}
	if err := os.Rename(path, backup); err != nil {
		return "", fmt.Errorf("backup generated tree %q: %w", path, err)
	}
	return backup, nil
}

func cleanupGeneratedTree(path string, cause error) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("%w; clean generated tree %q: %v", cause, path, err)
	}
	return cause
}

func reserveBackupPath(parent string) (string, error) {
	path, err := os.MkdirTemp(parent, ".sdk-go-contracts-backup-")
	if err != nil {
		return "", fmt.Errorf("create generated tree backup placeholder: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("remove generated tree backup placeholder %q: %w", path, err)
	}
	return path, nil
}

func restoreGeneratedTree(path string, backup string) error {
	if backup == "" {
		return nil
	}
	if err := os.Rename(backup, path); err != nil {
		return fmt.Errorf("restore generated tree %q: %w", path, err)
	}
	return nil
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
