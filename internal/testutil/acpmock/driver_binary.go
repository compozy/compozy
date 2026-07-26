package acpmock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const driverBinaryEnvVar = "AGH_TEST_ACPMOCK_DRIVER_BIN"

const defaultDriverBuildTimeout = 45 * time.Second

var (
	driverBinaryMu   sync.Mutex
	driverBinaryPath string
)

// DefaultDriverPath resolves or builds the test-only ACP mock driver binary.
func DefaultDriverPath() (string, error) {
	if trimmed := strings.TrimSpace(os.Getenv(driverBinaryEnvVar)); trimmed != "" {
		return resolveExplicitDriverPath(trimmed)
	}

	driverBinaryMu.Lock()
	defer driverBinaryMu.Unlock()

	if strings.TrimSpace(driverBinaryPath) != "" {
		return driverBinaryPath, nil
	}

	repoRoot, err := repoRootFromCaller()
	if err != nil {
		return "", err
	}

	outputDir, err := os.MkdirTemp("", "agh-acpmock-driver-")
	if err != nil {
		return "", fmt.Errorf("acpmock: create driver build directory: %w", err)
	}
	outputPath := filepath.Join(outputDir, driverBinaryName())

	buildCtx, cancel := context.WithTimeout(context.Background(), defaultDriverBuildTimeout)
	defer cancel()
	if err := buildDriverBinary(buildCtx, repoRoot, outputPath); err != nil {
		return "", err
	}

	driverBinaryPath = outputPath
	return outputPath, nil
}

func buildDriverBinary(ctx context.Context, repoRoot string, outputPath string) error {
	cmd := exec.CommandContext(
		ctx,
		"go",
		"build",
		"-o",
		outputPath,
		"./internal/testutil/acpmock/cmd/acpmock-driver",
	)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"acpmock: build driver binary: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}
	return nil
}

func resolveDriverPath(override string) (string, error) {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return resolveExplicitDriverPath(trimmed)
	}
	if trimmed := strings.TrimSpace(os.Getenv(driverBinaryEnvVar)); trimmed != "" {
		return resolveExplicitDriverPath(trimmed)
	}
	return DefaultDriverPath()
}

func resolveExplicitDriverPath(path string) (string, error) {
	resolved, err := aghconfig.ResolvePath(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved) == "" {
		return "", errors.New("acpmock: driver path is required")
	}
	return resolved, nil
}

func driverBinaryName() string {
	if runtime.GOOS == "windows" {
		return "acpmock-driver.exe"
	}
	return "acpmock-driver"
}

func repoRootFromCaller() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("acpmock: runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}
