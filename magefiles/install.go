//go:build mage

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/magefile/mage/sh"
)

func InstallerCheck() error {
	installer := filepath.Join("packages", "site", "public", "install.sh")
	if err := sh.RunV("sh", "-n", installer); err != nil {
		return err
	}
	return sh.RunV("sh", installer, "--dry-run", "--skip-bootstrap")
}

func ReleaseInstallCheck() error {
	return runMageSteps([]mageStep{
		{name: "WebBuild", run: WebBuild},
		{name: "WebAssetsDeterminismCheck", run: WebAssetsDeterminismCheck},
		{name: "WebAssetsCheck", run: WebAssetsCheck},
		{name: "WebAssetsPublicCheck", run: WebAssetsPublicCheck},
		{name: "SourceInstallCheck", run: SourceInstallCheck},
	})
}

// SourceInstallCheck verifies the public root go install path from source-visible files.
func SourceInstallCheck() error {
	tmpRoot, err := os.MkdirTemp("", "agh-source-install-check-")
	if err != nil {
		return fmt.Errorf("create source install check dir: %w", err)
	}
	defer os.RemoveAll(tmpRoot)

	sourceDir := filepath.Join(tmpRoot, "src")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return fmt.Errorf("create source install check source dir: %w", err)
	}
	if err := copySourceInstallFiles(sourceDir); err != nil {
		return err
	}

	binDir := filepath.Join(tmpRoot, "bin")
	env := map[string]string{
		"CGO_ENABLED":    "0",
		"GOBIN":          binDir,
		"GOMODCACHE":     filepath.Join(tmpRoot, "mod"),
		"GOPATH":         filepath.Join(tmpRoot, "gopath"),
		webDistDirEnvVar: "",
	}
	if err := runCommandInDirWithEnv(context.Background(), sourceDir, env, "go", "install", "."); err != nil {
		return fmt.Errorf("source install go install .: %w", err)
	}
	binary := filepath.Join(binDir, cliBinary)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if err := runCommandInDirWithEnv(context.Background(), sourceDir, env, binary, "version"); err != nil {
		return fmt.Errorf("source install agh version: %w", err)
	}
	if err := runCommandInDirWithEnv(
		context.Background(),
		sourceDir,
		env,
		"go",
		"test",
		"./internal/api/httpapi",
		"-run",
		"TestStaticRoutesServeEmbedded(IndexForRootAndDeepLinks|Assets)$",
		"-count=1",
	); err != nil {
		return fmt.Errorf("source install embedded web asset test: %w", err)
	}
	return nil
}

func copySourceInstallFiles(destRoot string) error {
	files, err := gitSourceInstallFiles()
	if err != nil {
		return err
	}
	for _, rel := range files {
		info, err := os.Lstat(rel)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat source install file %q: %w", rel, err)
		}
		if info.IsDir() {
			continue
		}
		dest := filepath.Join(destRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create source install parent for %q: %w", rel, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(rel)
			if err != nil {
				return fmt.Errorf("read source install symlink %q: %w", rel, err)
			}
			if err := os.Symlink(target, dest); err != nil {
				return fmt.Errorf("write source install symlink %q: %w", rel, err)
			}
			continue
		}
		data, err := os.ReadFile(rel)
		if err != nil {
			return fmt.Errorf("read source install file %q: %w", rel, err)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(dest, data, mode); err != nil {
			return fmt.Errorf("write source install file %q: %w", rel, err)
		}
	}
	return nil
}

func gitSourceInstallFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z", "-c", "-o", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list source install files: %w", err)
	}
	parts := bytes.Split(out, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(part)))
	}
	return files, nil
}
