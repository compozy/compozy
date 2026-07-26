package modelcatalog

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProviderModelHardCutResidueGuard(t *testing.T) {
	t.Parallel()

	t.Run("Should find no old provider model config residue outside allowlisted surfaces", func(t *testing.T) {
		t.Parallel()

		repoRoot := testRepoRoot(t)
		fields := []string{
			"default_model",
			"supported_models",
			"supports_reasoning_effort",
		}
		var residues []string
		for _, target := range []string{"cmd", "internal", "web", "packages/site", "openapi", "config.toml"} {
			targetPath := filepath.Join(repoRoot, target)
			info, err := os.Stat(targetPath)
			if err != nil {
				t.Fatalf("os.Stat(%q) error = %v", targetPath, err)
			}
			if !info.IsDir() {
				residues = appendResiduesFromFile(t, residues, repoRoot, targetPath, fields)
				continue
			}
			err = filepath.WalkDir(targetPath, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					if skipResidueGuardDir(entry.Name()) {
						return filepath.SkipDir
					}
					return nil
				}
				residues = appendResiduesFromFile(t, residues, repoRoot, path, fields)
				return nil
			})
			if err != nil {
				t.Fatalf("WalkDir(%q) error = %v", targetPath, err)
			}
		}

		if len(residues) > 0 {
			t.Fatalf(
				"provider model hard-cut residue found in non-test surfaces:\n%s",
				strings.Join(residues, "\n"),
			)
		}
	})
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func appendResiduesFromFile(
	t *testing.T,
	residues []string,
	repoRoot string,
	path string,
	fields []string,
) []string {
	t.Helper()

	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		t.Fatalf("filepath.Rel(%q, %q) error = %v", repoRoot, path, err)
	}
	rel = filepath.ToSlash(rel)
	if skipResidueGuardFile(rel) {
		return residues
	}
	allowedRanges := removedProviderModelKeyOwnerRanges(t, path, rel)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Close(%q) error = %v", path, closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		for _, field := range fields {
			if !strings.Contains(line, field) {
				continue
			}
			if allowedProviderModelResidue(rel, line, lineNo, field, allowedRanges) {
				continue
			}
			residues = append(residues, fmt.Sprintf("%s:%d contains %s", rel, lineNo, field))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scan(%q) error = %v", path, err)
	}
	return residues
}

type sourceLineRange struct {
	start int
	end   int
}

func removedProviderModelKeyOwnerRanges(t *testing.T, path string, rel string) []sourceLineRange {
	t.Helper()

	if !strings.HasPrefix(rel, "internal/config/") || filepath.Ext(rel) != ".go" {
		return nil
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(%q) error = %v", path, err)
	}

	ranges := make([]sourceLineRange, 0, 1)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "rejectRemovedProviderModelKeys" {
			continue
		}
		ranges = append(ranges, sourceLineRange{
			start: fileSet.Position(function.Pos()).Line,
			end:   fileSet.Position(function.End()).Line,
		})
	}
	return ranges
}

func skipResidueGuardDir(name string) bool {
	switch name {
	case ".git", ".next", ".tmp", ".turbo", "coverage", "dist", "node_modules", "out", "storybook-static":
		return true
	default:
		return false
	}
}

func skipResidueGuardFile(rel string) bool {
	base := filepath.Base(rel)
	if strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasSuffix(base, ".snap") ||
		strings.HasSuffix(base, ".tsbuildinfo") {
		return true
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".avif", ".tsbuildinfo":
		return true
	}
	return strings.Contains(rel, "/__tests__/") || strings.Contains(rel, "/testdata/")
}

func allowedProviderModelResidue(
	rel string,
	line string,
	lineNo int,
	field string,
	allowedRanges []sourceLineRange,
) bool {
	if lineWithinRanges(lineNo, allowedRanges) {
		return strings.Contains(line, fmt.Sprintf("%q", field))
	}
	if rel == "packages/site/content/runtime/core/agents/providers.mdx" ||
		rel == "packages/site/content/runtime/core/configuration/config-toml.mdx" {
		return strings.Contains(line, "flat keys") || strings.Contains(line, "are no longer")
	}
	if field != "supported_models" {
		return false
	}
	switch rel {
	case "internal/api/contract/contract.go",
		"web/src/generated/agh-openapi.d.ts",
		"openapi/agh.json",
		"web/src/systems/session/mocks/fixtures.ts",
		"web/src/systems/network/mocks/fixtures.ts":
		return true
	default:
		return false
	}
}

func lineWithinRanges(line int, ranges []sourceLineRange) bool {
	for _, candidate := range ranges {
		if line >= candidate.start && line <= candidate.end {
			return true
		}
	}
	return false
}
