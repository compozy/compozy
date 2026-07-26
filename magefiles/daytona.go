//go:build mage

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DaytonaSidecars regenerates embedded Linux launcher sidecar assets.
func DaytonaSidecars() error {
	for _, asset := range daytonaSidecarAssets {
		if err := buildDaytonaSidecarAsset(context.Background(), asset, asset.path); err != nil {
			return err
		}
	}
	return nil
}

// DaytonaSidecarsCheck verifies embedded launcher sidecar assets are current.
func DaytonaSidecarsCheck() error {
	tmpDir, err := os.MkdirTemp("", "agh-daytona-sidecar-check-")
	if err != nil {
		return fmt.Errorf("create Daytona sidecar check dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, asset := range daytonaSidecarAssets {
		tmpAssetPath := filepath.Join(tmpDir, filepath.Base(asset.path))
		if err := buildDaytonaSidecarAsset(context.Background(), asset, tmpAssetPath); err != nil {
			return err
		}
		generated, err := os.ReadFile(tmpAssetPath)
		if err != nil {
			return fmt.Errorf("read generated Daytona sidecar asset %q: %w", tmpAssetPath, err)
		}
		current, err := os.ReadFile(asset.path)
		if err != nil {
			return fmt.Errorf(
				"read Daytona sidecar asset %q: %w; run %s",
				asset.path,
				err,
				daytonaSidecarRegenHint,
			)
		}
		if !bytes.Equal(generated, current) {
			return fmt.Errorf(
				"Daytona sidecar asset %q is stale; run %s",
				asset.path,
				daytonaSidecarRegenHint,
			)
		}
	}
	return nil
}

func buildDaytonaSidecarAsset(ctx context.Context, asset daytonaSidecarAsset, outputPath string) error {
	tmpDir, err := os.MkdirTemp("", "agh-daytona-sidecar-build-")
	if err != nil {
		return fmt.Errorf("create Daytona sidecar build dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "agh-daytona-sidecar")
	if err := runCommandInDirWithEnv(
		ctx,
		".",
		map[string]string{
			"CGO_ENABLED":  "0",
			"GODEBUG":      "",
			"GOENV":        "off",
			"GOEXPERIMENT": "",
			"GOFLAGS":      "",
			"GOAMD64":      "v1",
			"GOARM64":      "v8.0",
			"GOOS":         "linux",
			"GOARCH":       asset.arch,
			"GOTOOLCHAIN":  "go" + daytonaSidecarToolchain,
		},
		"go",
		"build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags",
		"-s -w -buildid=",
		"-o",
		binaryPath,
		daytonaSidecarPackage,
	); err != nil {
		return fmt.Errorf("build Daytona launcher sidecar for linux/%s: %w", asset.arch, err)
	}
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read Daytona launcher sidecar for linux/%s: %w", asset.arch, err)
	}
	if len(binary) == 0 {
		return fmt.Errorf("Daytona launcher sidecar for linux/%s is empty", asset.arch)
	}
	if err := writeGzipAsset(outputPath, binary); err != nil {
		return fmt.Errorf("write Daytona sidecar asset %q: %w", outputPath, err)
	}
	return nil
}

func writeGzipAsset(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create gzip asset dir %q: %w", filepath.Dir(path), err)
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	if _, err := writer.Write(data); err != nil {
		closeErr := writer.Close()
		return errors.Join(fmt.Errorf("write gzip payload: %w", err), closeErr)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}
	return os.WriteFile(path, compressed.Bytes(), 0o644)
}
