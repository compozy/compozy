//go:build mage

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func WebAssetsCheck() error {
	if err := ensureWebDist(); err != nil {
		return err
	}
	moduleDir, err := webAssetsModuleDir(context.Background())
	if err != nil {
		return err
	}
	localDigest, err := directoryDigest(webDistDir)
	if err != nil {
		return fmt.Errorf("digest local web build: %w", err)
	}
	moduleDigest, err := directoryDigest(filepath.Join(moduleDir, webAssetsModuleDistDir))
	if err != nil {
		return fmt.Errorf("digest %s module assets: %w", webAssetsModulePath, err)
	}
	metadata, err := readWebAssetsMetadata(moduleDir)
	if err != nil {
		return err
	}
	if metadata.BuildDigest != "" && metadata.BuildDigest != moduleDigest {
		return fmt.Errorf(
			"%s metadata digest %s differs from module dist digest %s",
			webAssetsModulePath,
			metadata.BuildDigest,
			moduleDigest,
		)
	}
	if metadata.SourceRepository != "" && metadata.SourceRepository != webAssetsSourceRepository {
		return fmt.Errorf(
			"%s metadata source repository %q differs from %q",
			webAssetsModulePath,
			metadata.SourceRepository,
			webAssetsSourceRepository,
		)
	}
	if localDigest != moduleDigest {
		return fmt.Errorf(
			"%s is stale: local %s digest %s differs from module %s digest %s",
			webAssetsModulePath,
			webDistDir,
			localDigest,
			webAssetsModuleDistDir,
			moduleDigest,
		)
	}
	return nil
}

func WebAssetsDeterminismCheck() error {
	return webAssetsDeterminismCheck(
		WebBuild,
		func() error {
			return os.RemoveAll(webDistDir)
		},
		func() (string, error) {
			return directoryDigest(webDistDir)
		},
	)
}

func WebAssetsPublicCheck() error {
	ctx := context.Background()
	version, err := pinnedWebAssetsVersion(ctx)
	if err != nil {
		return err
	}
	return webAssetsPublicCheck(ctx, version)
}

func webAssetsModuleDir(ctx context.Context) (string, error) {
	if err := runCommandInDir(ctx, ".", "go", "mod", "download", webAssetsModulePath); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", webAssetsModulePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("locate %s module: %w\n%s", webAssetsModulePath, err, output)
	}
	moduleDir := strings.TrimSpace(string(output))
	if moduleDir == "" {
		return "", fmt.Errorf("locate %s module: go list returned an empty directory", webAssetsModulePath)
	}
	return moduleDir, nil
}

func parseWebAssetsMetadataSource(source string) webAssetsMetadata {
	return webAssetsMetadata{
		BuildDigest:      goStringConst(source, "BuildDigest"),
		SourceRepository: goStringConst(source, "SourceRepository"),
		SourceCommit:     goStringConst(source, "SourceCommit"),
	}
}

func directoryDigest(root string) (string, error) {
	paths := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q is a symlink; embedded web assets must be regular files", path)
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("%s contains no files", root)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", fmt.Errorf("resolve %q relative to %q: %w", path, root, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %q: %w", path, err)
		}
		hash.Write([]byte(filepath.ToSlash(rel)))
		hash.Write([]byte{0})
		hash.Write(data)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func prepareWebAssetsRepo(srcDistDir string, assetsRepoDir string, metadata webAssetsMetadata) error {
	if metadata.BuildDigest == "" {
		return errors.New("web assets build digest is required")
	}
	if metadata.SourceRepository == "" {
		return errors.New("web assets source repository is required")
	}
	if metadata.SourceCommit == "" {
		return errors.New("web assets source commit is required")
	}
	info, err := os.Stat(assetsRepoDir)
	if err != nil {
		return fmt.Errorf("stat assets repo dir %q: %w", assetsRepoDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("assets repo path %q is not a directory", assetsRepoDir)
	}
	destDistDir := filepath.Join(assetsRepoDir, webAssetsModuleDistDir)
	if err := os.RemoveAll(destDistDir); err != nil {
		return fmt.Errorf("remove existing assets dist %q: %w", destDistDir, err)
	}
	if err := copyWebAssetsDist(srcDistDir, destDistDir); err != nil {
		return err
	}
	return writeWebAssetsMetadata(assetsRepoDir, metadata)
}

func copyWebAssetsDist(srcDir string, destDir string) error {
	return filepath.WalkDir(srcDir, func(srcPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", srcPath, walkErr)
		}
		rel, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return fmt.Errorf("resolve %q relative to %q: %w", srcPath, srcDir, err)
		}
		destPath := filepath.Join(destDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", srcPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q is a symlink; web assets module dist must contain regular files", srcPath)
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read %q: %w", srcPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("create parent for %q: %w", destPath, err)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(destPath, data, mode); err != nil {
			return fmt.Errorf("write %q: %w", destPath, err)
		}
		return nil
	})
}

func writeWebAssetsMetadata(assetsRepoDir string, metadata webAssetsMetadata) error {
	content := strings.Join([]string{
		"// Package webassets embeds the production AGH web UI bundle.",
		"package webassets",
		"",
		"import \"embed\"",
		"",
		"// DistDir is the root directory embedded in DistFS.",
		"const DistDir = \"dist\"",
		"",
		"const (",
		"\tBuildDigest = " + strconv.Quote(metadata.BuildDigest),
		"\tSourceRepository = " + strconv.Quote(metadata.SourceRepository),
		"\tSourceCommit = " + strconv.Quote(metadata.SourceCommit),
		")",
		"",
		"// DistFS embeds the generated production AGH web UI bundle.",
		"//",
		"//go:embed all:dist",
		"var DistFS embed.FS",
		"",
	}, "\n")
	path := filepath.Join(assetsRepoDir, webAssetsMetadataFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write web assets metadata %q: %w", path, err)
	}
	return nil
}

func readWebAssetsMetadata(moduleDir string) (webAssetsMetadata, error) {
	data, err := os.ReadFile(filepath.Join(moduleDir, webAssetsMetadataFile))
	if err != nil {
		return webAssetsMetadata{}, fmt.Errorf("read %s metadata: %w", webAssetsModulePath, err)
	}
	return parseWebAssetsMetadataSource(string(data)), nil
}

func goStringConst(source string, name string) string {
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		left, right, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(left) != name {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(right))
		if len(fields) == 0 {
			return ""
		}
		value, err := strconv.Unquote(fields[0])
		if err != nil {
			return ""
		}
		return value
	}
	return ""
}

func nextWebAssetsTag(tags []string) (string, error) {
	var highest webAssetsSemver
	found := false
	for _, tag := range tags {
		version, ok := parseWebAssetsSemver(tag)
		if !ok {
			continue
		}
		if !found || compareWebAssetsSemver(version, highest) > 0 {
			highest = version
			found = true
		}
	}
	if !found {
		return "v0.0.1", nil
	}
	if highest.patch == int(^uint(0)>>1) {
		return "", fmt.Errorf("cannot increment patch version for %v", highest)
	}
	highest.patch++
	return fmt.Sprintf("v%d.%d.%d", highest.major, highest.minor, highest.patch), nil
}

func parseWebAssetsSemver(tag string) (webAssetsSemver, bool) {
	raw := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return webAssetsSemver{}, false
	}
	values := [3]int{}
	for idx, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return webAssetsSemver{}, false
		}
		values[idx] = value
	}
	return webAssetsSemver{major: values[0], minor: values[1], patch: values[2]}, true
}

func compareWebAssetsSemver(left webAssetsSemver, right webAssetsSemver) int {
	if left.major != right.major {
		return left.major - right.major
	}
	if left.minor != right.minor {
		return left.minor - right.minor
	}
	return left.patch - right.patch
}

func webAssetsDeterminismCheck(
	build func() error,
	clean func() error,
	digest func() (string, error),
) error {
	if err := clean(); err != nil {
		return fmt.Errorf("clean first web build: %w", err)
	}
	if err := build(); err != nil {
		return fmt.Errorf("first web build: %w", err)
	}
	firstDigest, err := digest()
	if err != nil {
		return fmt.Errorf("digest first web build: %w", err)
	}
	if err := clean(); err != nil {
		return fmt.Errorf("clean second web build: %w", err)
	}
	if err := build(); err != nil {
		return fmt.Errorf("second web build: %w", err)
	}
	secondDigest, err := digest()
	if err != nil {
		return fmt.Errorf("digest second web build: %w", err)
	}
	if firstDigest != secondDigest {
		return fmt.Errorf(
			"web build is not deterministic: first digest %s, second digest %s",
			firstDigest,
			secondDigest,
		)
	}
	return nil
}

func pinnedWebAssetsVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Version}}", webAssetsModulePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve pinned %s version: %w\n%s", webAssetsModulePath, err, output)
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("resolve pinned %s version: empty version", webAssetsModulePath)
	}
	return version, nil
}

func webAssetsPublicCheck(ctx context.Context, version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("web assets module version is required")
	}
	tmpDir, err := os.MkdirTemp("", "agh-web-assets-public-check-")
	if err != nil {
		return fmt.Errorf("create web assets public check dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	env := webAssetsPublicModuleEnv(tmpDir)
	moduleVersion := webAssetsModulePath + "@" + version
	if err := runCommandInDirWithEnv(ctx, tmpDir, env, "go", "list", "-m", moduleVersion); err != nil {
		return fmt.Errorf("public resolve %s: %w", moduleVersion, err)
	}
	return nil
}

func webAssetsPublicModuleEnv(tmpDir string) map[string]string {
	env := map[string]string{
		"GO111MODULE": "on",
		"GOFLAGS":     "-mod=mod",
		"GONOPROXY":   "",
		"GONOSUMDB":   "",
		"GOPRIVATE":   "",
		"GOPROXY":     "https://proxy.golang.org,direct",
		"GOSUMDB":     "sum.golang.org",
	}
	if tmpDir != "" {
		env["GOMODCACHE"] = filepath.Join(tmpDir, "mod")
		env["GOPATH"] = filepath.Join(tmpDir, "gopath")
	}
	return env
}
