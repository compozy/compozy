//go:build mage

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Daytona sidecar check stamp: DaytonaSidecarsCheck cross-compiles two Linux
// sidecars on every CodegenCheck. When every build input is unchanged since the
// last passing check on this machine, the stamp lets the check skip. Fresh
// environments (CI) have no stamp and always run the full check;
// AGH_DAYTONA_CHECK=full forces it locally.

const (
	daytonaCheckModeEnvVar = "AGH_DAYTONA_CHECK"
	daytonaStampFileName   = "daytona-check.stamp"
	daytonaSidecarPkgPath  = "./internal/sandbox/daytona/cmd/agh-daytona-sidecar"
	repoModulePath         = "github.com/compozy/agh"
)

func daytonaSidecarsCheckStamped() error {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(daytonaCheckModeEnvVar)), "full") {
		return DaytonaSidecarsCheck()
	}
	digest, err := daytonaCheckInputsDigest(context.Background())
	if err != nil {
		fmt.Printf("Warning: daytona check stamp unavailable (%v); running full check\n", err)
		return DaytonaSidecarsCheck()
	}
	stampPath, err := daytonaStampPath()
	if err != nil {
		fmt.Printf("Warning: daytona check stamp unavailable (%v); running full check\n", err)
		return DaytonaSidecarsCheck()
	}
	if recorded, readErr := os.ReadFile(stampPath); readErr == nil && strings.TrimSpace(string(recorded)) == digest {
		fmt.Printf(
			"Daytona sidecar check: inputs unchanged since last passing check — skipped (%s=full to force)\n",
			daytonaCheckModeEnvVar,
		)
		return nil
	}
	if err := DaytonaSidecarsCheck(); err != nil {
		return err
	}
	if err := os.WriteFile(stampPath, []byte(digest+"\n"), 0o644); err != nil {
		fmt.Printf("Warning: write daytona check stamp %q: %v\n", stampPath, err)
	}
	return nil
}

func daytonaStampPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	stampDir := filepath.Join(cacheDir, verifyLockDirName)
	if err := os.MkdirAll(stampDir, 0o755); err != nil {
		return "", fmt.Errorf("create stamp dir %q: %w", stampDir, err)
	}
	return filepath.Join(stampDir, daytonaStampFileName), nil
}

// daytonaCheckInputsDigest hashes everything that determines the check outcome:
// the module-local source files (plus embeds) of the sidecar package and its
// dependency closure resolved for each target GOOS/GOARCH, go.mod/go.sum, the
// committed sidecar assets, and the build parameters. The build parameter
// string must stay in sync with buildDaytonaSidecarAsset — when in doubt the
// stamp over-invalidates, never under.
func daytonaCheckInputsDigest(ctx context.Context) (string, error) {
	digest := sha256.New()
	fmt.Fprintf(
		digest,
		"toolchain=go%s;flags=-trimpath -buildvcs=false;ldflags=-s -w -buildid=;goamd64=v1;goarm64=v8.0;gzip=best;",
		daytonaSidecarToolchain,
	)
	assetPaths := make([]string, 0, len(daytonaSidecarAssets))
	for _, asset := range daytonaSidecarAssets {
		fmt.Fprintf(digest, "arch=%s;", asset.arch)
		assetPaths = append(assetPaths, asset.path)
	}
	files := map[string]struct{}{"go.mod": {}, "go.sum": {}}
	for _, path := range assetPaths {
		files[path] = struct{}{}
	}
	for _, asset := range daytonaSidecarAssets {
		depFiles, err := daytonaDepSourceFiles(ctx, asset.arch)
		if err != nil {
			return "", err
		}
		for _, file := range depFiles {
			files[file] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(files))
	for file := range files {
		ordered = append(ordered, file)
	}
	sort.Strings(ordered)
	if err := hashFilesInto(digest, ordered); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func daytonaDepSourceFiles(ctx context.Context, arch string) ([]string, error) {
	cmd := exec.CommandContext(
		ctx,
		"go", "list", "-deps",
		"-f", `{{.Dir}}|{{if .Module}}{{.Module.Path}}{{end}}|{{join .GoFiles ","}}|{{join .EmbedFiles ","}}`,
		daytonaSidecarPkgPath,
	)
	cmd.Env = mergeEnvOverrides(os.Environ(), map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "linux",
		"GOARCH":      arch,
	})
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list daytona sidecar deps for linux/%s: %w", arch, err)
	}
	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		dir, modPath, names, ok := parseGoListDepLine(line)
		if !ok || modPath != repoModulePath {
			continue
		}
		for _, name := range names {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

func parseGoListDepLine(line string) (dir string, modPath string, files []string, ok bool) {
	parts := strings.Split(strings.TrimSpace(line), "|")
	if len(parts) != 4 || parts[0] == "" {
		return "", "", nil, false
	}
	for _, field := range []string{parts[2], parts[3]} {
		for _, name := range strings.Split(field, ",") {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				files = append(files, trimmed)
			}
		}
	}
	return parts[0], parts[1], files, true
}

func hashFilesInto(digest hash.Hash, paths []string) error {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("hash daytona input %q: %w", path, err)
		}
		digest.Write([]byte(path))
		digest.Write([]byte{0})
		digest.Write(data)
		digest.Write([]byte{0})
	}
	return nil
}
