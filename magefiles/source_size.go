//go:build mage

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const maxProductionSourceLines = 500

type productionSourceSizeViolation struct {
	path  string
	lines int
}

var productionSourceExtensions = map[string]struct{}{
	".c": {}, ".cc": {}, ".cjs": {}, ".cpp": {}, ".css": {}, ".go": {},
	".h": {}, ".hpp": {}, ".java": {}, ".js": {}, ".jsx": {}, ".kt": {},
	".mjs": {}, ".py": {}, ".rb": {}, ".rs": {}, ".scss": {}, ".sh": {},
	".swift": {}, ".ts": {}, ".tsx": {},
}

var productionSourceRoots = []string{
	"cmd/",
	"extensions/",
	"internal/",
	"lint-plugins/",
	"magefiles/",
	"packages/",
	"scripts/",
	"sdk/",
	"skills/",
	"web/",
}

var excludedProductionSourceSegments = map[string]struct{}{
	"__tests__":  {},
	"e2e":        {},
	"examples":   {},
	"fixtures":   {},
	"generated":  {},
	"migrations": {},
	"mocks":      {},
	"sqlcgen":    {},
	"testdata":   {},
	"tests":      {},
}

func inspectProductionSourceLineLimit(root string) ([]productionSourceSizeViolation, error) {
	trackedFiles, err := listTrackedFiles(root)
	if err != nil {
		return nil, err
	}
	return inspectTrackedProductionSources(root, trackedFiles)
}

func listTrackedFiles(root string) ([]string, error) {
	command := exec.Command(
		"git",
		"-C",
		root,
		"ls-files",
		"--cached",
		"--others",
		"--exclude-standard",
		"-z",
	)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}

	parts := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		files = append(files, string(part))
	}
	return files, nil
}

func inspectTrackedProductionSources(
	root string,
	trackedFiles []string,
) ([]productionSourceSizeViolation, error) {
	violations := make([]productionSourceSizeViolation, 0)
	for _, trackedPath := range trackedFiles {
		path := filepath.ToSlash(filepath.Clean(trackedPath))
		if !isProductionSourcePath(path) {
			continue
		}

		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read tracked production source %q: %w", path, err)
		}
		if isGeneratedSource(contents) {
			continue
		}

		lines := physicalLineCount(contents)
		if lines <= maxProductionSourceLines {
			continue
		}
		violations = append(violations, productionSourceSizeViolation{path: path, lines: lines})
	}

	sort.Slice(violations, func(left int, right int) bool {
		if violations[left].lines == violations[right].lines {
			return violations[left].path < violations[right].path
		}
		return violations[left].lines > violations[right].lines
	})
	return violations, nil
}

func isProductionSourcePath(path string) bool {
	if path != "main.go" && !hasProductionSourceRoot(path) {
		return false
	}
	if strings.HasPrefix(path, "packages/slides/") || path == "packages/ui/src/tokens.css" {
		return false
	}
	if path == "web/src/routeTree.gen.ts" {
		return false
	}

	name := filepath.Base(path)
	if isTestSourceName(name) || isGeneratedSourceName(name) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if _, excluded := excludedProductionSourceSegments[segment]; excluded {
			return false
		}
	}
	_, supported := productionSourceExtensions[strings.ToLower(filepath.Ext(name))]
	return supported
}

func hasProductionSourceRoot(path string) bool {
	for _, root := range productionSourceRoots {
		if strings.HasPrefix(path, root) {
			return true
		}
	}
	return false
}

func isTestSourceName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, "_test.go") ||
		strings.HasPrefix(lower, "test_") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.")
}

func isGeneratedSourceName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "zz_generated") ||
		strings.Contains(lower, ".generated.") ||
		strings.Contains(lower, ".gen.") ||
		strings.HasSuffix(lower, "_generated.go")
}

func isGeneratedSource(contents []byte) bool {
	const generatedHeaderLimit = 4096
	prefix := contents
	if len(prefix) > generatedHeaderLimit {
		prefix = prefix[:generatedHeaderLimit]
	}
	return bytes.Contains(prefix, []byte("Code generated")) &&
		bytes.Contains(prefix, []byte("DO NOT EDIT"))
}

func physicalLineCount(contents []byte) int {
	if len(contents) == 0 {
		return 0
	}
	lines := bytes.Count(contents, []byte{'\n'})
	if contents[len(contents)-1] != '\n' {
		lines++
	}
	return lines
}
