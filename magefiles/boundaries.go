//go:build mage

package main

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const aghModulePath = "github.com/compozy/agh/"

type dependencyClosureRule struct {
	root              string
	allowedPrefixes   []string
	forbiddenExact    []string
	forbiddenPrefixes []string
}

type dependencyClosureViolation struct {
	root       string
	dependency string
}

var bridgeDependencyClosureRules = func() []dependencyClosureRule {
	allowedPrefixes := []string{aghModulePath + "internal/bridges/contract"}
	forbiddenPrefixes := []string{
		aghModulePath + "internal/bridges",
		aghModulePath + "internal/store",
		aghModulePath + "internal/resources",
		aghModulePath + "internal/task",
		aghModulePath + "internal/session",
		aghModulePath + "internal/automation",
		aghModulePath + "internal/observe",
		aghModulePath + "internal/extension",
		aghModulePath + "internal/daemon",
	}
	roots := []string{
		"./internal/bridgesdk",
		"./internal/subprocess",
		"./extensions/bridges/slack",
		"./extensions/bridges/telegram",
		"./extensions/bridges/discord",
		"./extensions/bridges/teams",
		"./extensions/bridges/gchat",
		"./extensions/bridges/whatsapp",
		"./extensions/bridges/github",
		"./extensions/bridges/linear",
		"./sdk/examples/telegram-reference",
	}
	rules := make([]dependencyClosureRule, 0, len(roots))
	for _, root := range roots {
		rules = append(rules, dependencyClosureRule{
			allowedPrefixes:   allowedPrefixes,
			root:              root,
			forbiddenPrefixes: forbiddenPrefixes,
		})
	}
	return rules
}()

// Boundaries verifies direct package rules, strict leaf imports, and bridge dependency closures.
func Boundaries() error {
	forbidden := []struct {
		importer string
		imported string
	}{
		{"internal/config", "internal/daemon"},
		{"internal/acp", "internal/daemon"},
		{"internal/session", "internal/daemon"},
		{"internal/store", "internal/daemon"},
		{"internal/observe", "internal/daemon"},
		{"internal/events", "internal/daemon"},
		{"internal/doctor", "internal/daemon"},
		{"internal/providers", "internal/daemon"},
		{"internal/loop", "internal/daemon"},
		{"internal/diagnosticcontract", "internal/daemon"},
		{"internal/config", "internal/api/httpapi"},
		{"internal/acp", "internal/api/httpapi"},
		{"internal/session", "internal/api/httpapi"},
		{"internal/store", "internal/api/httpapi"},
		{"internal/observe", "internal/api/httpapi"},
		{"internal/events", "internal/api/httpapi"},
		{"internal/doctor", "internal/api/httpapi"},
		{"internal/providers", "internal/api/httpapi"},
		{"internal/loop", "internal/api/httpapi"},
		{"internal/diagnosticcontract", "internal/api/httpapi"},
		{"internal/config", "internal/api/udsapi"},
		{"internal/acp", "internal/api/udsapi"},
		{"internal/session", "internal/api/udsapi"},
		{"internal/store", "internal/api/udsapi"},
		{"internal/observe", "internal/api/udsapi"},
		{"internal/events", "internal/api/udsapi"},
		{"internal/doctor", "internal/api/udsapi"},
		{"internal/providers", "internal/api/udsapi"},
		{"internal/loop", "internal/api/udsapi"},
		{"internal/diagnosticcontract", "internal/api/udsapi"},
		{"internal/config", "internal/cli"},
		{"internal/acp", "internal/cli"},
		{"internal/session", "internal/cli"},
		{"internal/store", "internal/cli"},
		{"internal/observe", "internal/cli"},
		{"internal/events", "internal/cli"},
		{"internal/doctor", "internal/cli"},
		{"internal/providers", "internal/cli"},
		{"internal/loop", "internal/cli"},
		{"internal/diagnosticcontract", "internal/cli"},
		{"internal/providers", "internal/session"},
		{"internal/providers", "internal/acp"},
		{"internal/providers", "internal/api/core"},
		{"internal/api/contract", "internal/daemon"},
		{"internal/api/contract", "internal/api/httpapi"},
		{"internal/api/contract", "internal/api/udsapi"},
		{"internal/api/contract", "internal/cli"},
		{"internal/diagnosticcontract", "internal/api/contract"},
		{"internal/diagnosticcontract", "internal/api/core"},
		{"internal/events", "internal/api/contract"},
		{"internal/events", "internal/api/core"},
		{"internal/loop", "internal/api/contract"},
		{"internal/loop", "internal/api/core"},
		{"internal/loop", "internal/loop/goal"},
		{"internal/automation", "internal/loop"},
		{"internal/automation", "internal/loop/dsl"},
		{"internal/api/core", "internal/daemon"},
		{"internal/api/core", "internal/api/httpapi"},
		{"internal/api/core", "internal/api/udsapi"},
		{"internal/api/core", "internal/cli"},
		{"internal/api/httpapi", "internal/daemon"},
		{"internal/api/httpapi", "internal/api/udsapi"},
		{"internal/api/httpapi", "internal/cli"},
		{"internal/api/udsapi", "internal/daemon"},
		{"internal/api/udsapi", "internal/api/httpapi"},
		{"internal/api/udsapi", "internal/cli"},
		{"internal/modelcatalog", "internal/daemon"},
		{"internal/modelcatalog", "internal/api/contract"},
		{"internal/modelcatalog", "internal/api/core"},
		{"internal/modelcatalog", "internal/api/httpapi"},
		{"internal/modelcatalog", "internal/api/udsapi"},
		{"internal/modelcatalog", "internal/cli"},
		{"internal/marketplace", "internal/daemon"},
		{"internal/marketplace", "internal/api/contract"},
		{"internal/marketplace", "internal/api/core"},
		{"internal/marketplace", "internal/api/httpapi"},
		{"internal/marketplace", "internal/api/udsapi"},
		{"internal/marketplace", "internal/cli"},
		{"internal/marketplace", "internal/skills"},
		{"internal/marketplace", "internal/extension"},
		{"internal/marketplace", "internal/bundles"},
		{"internal/marketplace", "internal/settings"},
		{"internal/marketplace", "internal/mcp"},
		{"internal/memory/contract", "internal/memory/controller"},
		{"internal/memory/contract", "internal/memory/recall"},
		{"internal/memory/contract", "internal/memory/extractor"},
		{"internal/memory/contract", "internal/memory/provider/local"},
		{"internal/memory/contract", "internal/store/workspacedb"},
		{"internal/memory/controller", "internal/daemon"},
		{"internal/memory/controller", "internal/api/httpapi"},
		{"internal/memory/controller", "internal/api/udsapi"},
		{"internal/memory/controller", "internal/cli"},
		{"internal/memory/recall", "internal/daemon"},
		{"internal/memory/recall", "internal/api/httpapi"},
		{"internal/memory/recall", "internal/api/udsapi"},
		{"internal/memory/recall", "internal/cli"},
		{"internal/memory/extractor", "internal/daemon"},
		{"internal/memory/extractor", "internal/api/httpapi"},
		{"internal/memory/extractor", "internal/api/udsapi"},
		{"internal/memory/extractor", "internal/cli"},
		{"internal/memory/provider/local", "internal/daemon"},
		{"internal/memory/provider/local", "internal/api/httpapi"},
		{"internal/memory/provider/local", "internal/api/udsapi"},
		{"internal/memory/provider/local", "internal/cli"},
		{"internal/sessions/ledger", "internal/daemon"},
		{"internal/sessions/ledger", "internal/api/httpapi"},
		{"internal/sessions/ledger", "internal/api/udsapi"},
		{"internal/sessions/ledger", "internal/cli"},
		{"internal/store/workspacedb", "internal/daemon"},
		{"internal/store/workspacedb", "internal/api/httpapi"},
		{"internal/store/workspacedb", "internal/api/udsapi"},
		{"internal/store/workspacedb", "internal/cli"},
	}

	violations := 0
	for _, rule := range forbidden {
		importerDir := rule.importer
		if _, err := os.Stat(importerDir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("inspect boundary importer %q: %w", importerDir, err)
		}
		importPath := "github.com/compozy/agh/" + rule.imported
		files, err := filesImporting(importerDir, importPath)
		if err != nil {
			return fmt.Errorf("check whether %q imports %q: %w", importerDir, importPath, err)
		}
		if len(files) > 0 {
			fmt.Printf("VIOLATION: %s imports %s\n", rule.importer, rule.imported)
			for _, file := range files {
				fmt.Printf("  %s\n", file)
			}
			violations++
		}
	}

	prefixForbidden := []struct {
		importer string
		imported string
	}{
		// In-tree platform implementations live under extensions/bridges. The
		// daemon bridge domain must not depend on either that exact package or
		// any provider sibling below it; provider composition points inward.
		{"internal/bridges", "extensions/bridges"},
		{"internal/bridges", "internal/bridgesdk"},
		{"internal/bridges", "internal/extension"},
		{"internal/bridges", "internal/daemon"},
		{"internal/bridgesdk", "internal/extension"},
		{"internal/bridgesdk", "internal/daemon"},
		{"internal/bridgesdk", "internal/store"},
		{"internal/bridgesdk", "internal/session"},
	}
	for _, rule := range prefixForbidden {
		importerDir := rule.importer
		if _, err := os.Stat(importerDir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("inspect boundary importer %q: %w", importerDir, err)
		}
		importPath := "github.com/compozy/agh/" + rule.imported
		files, err := filesImportingPrefix(importerDir, importPath)
		if err != nil {
			return fmt.Errorf("check whether %q imports %q or a subpackage: %w", importerDir, importPath, err)
		}
		if len(files) == 0 {
			continue
		}
		fmt.Printf("VIOLATION: %s imports %s or a subpackage\n", rule.importer, rule.imported)
		for _, file := range files {
			fmt.Printf("  %s\n", file)
		}
		violations++
	}

	leafRules := []struct {
		importer string
		allowed  map[string]struct{}
	}{
		{importer: "internal/redact", allowed: map[string]struct{}{}},
		{importer: "internal/extensionprotocol", allowed: map[string]struct{}{}},
		{importer: "internal/network/participation", allowed: map[string]struct{}{}},
		{
			importer: "internal/toolmeta",
			allowed: map[string]struct{}{
				"github.com/compozy/agh/internal/redact": {},
			},
		},
		{
			importer: "internal/bridges/contract",
			allowed: map[string]struct{}{
				"github.com/compozy/agh/internal/redact": {},
			},
		},
	}
	for _, rule := range leafRules {
		files, err := productionFilesImportingOutsideLeaf(rule.importer, rule.allowed)
		if err != nil {
			return fmt.Errorf("inspect leaf boundary importer %q: %w", rule.importer, err)
		}
		if len(files) == 0 {
			continue
		}
		fmt.Printf("VIOLATION: %s imports a non-leaf dependency\n", rule.importer)
		for _, file := range files {
			fmt.Printf("  %s\n", file)
		}
		violations++
	}

	closureViolations, err := inspectDependencyClosures(
		bridgeDependencyClosureRules,
		listPackageDependencies,
	)
	if err != nil {
		return fmt.Errorf("inspect bridge dependency closures: %w", err)
	}
	for _, violation := range closureViolations {
		fmt.Printf(
			"VIOLATION: %s transitively depends on %s\n",
			violation.root,
			violation.dependency,
		)
		violations++
	}

	sourceSizeViolations, err := inspectProductionSourceLineLimit(".")
	if err != nil {
		return fmt.Errorf("inspect production source line limit: %w", err)
	}
	for _, violation := range sourceSizeViolations {
		fmt.Printf(
			"VIOLATION: production source exceeds %d lines: %s (%d lines)\n",
			maxProductionSourceLines,
			violation.path,
			violation.lines,
		)
		violations++
	}

	if violations > 0 {
		return fmt.Errorf("found %d boundary violations", violations)
	}
	fmt.Println("OK: all package boundaries respected")
	return nil
}

func productionFilesImportingOutsideLeaf(
	root string,
	allowed map[string]struct{},
) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	fset := token.NewFileSet()
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse Go imports in %q: %w", path, err)
		}
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("decode Go import in %q: %w", path, err)
			}
			if isStandardLibraryImport(importPath) {
				continue
			}
			if _, ok := allowed[importPath]; ok {
				continue
			}
			files = append(files, path)
			break
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func isStandardLibraryImport(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

func inspectDependencyClosures(
	rules []dependencyClosureRule,
	listDependencies func(string) ([]string, error),
) ([]dependencyClosureViolation, error) {
	violations := make([]dependencyClosureViolation, 0)
	for _, rule := range rules {
		dependencies, err := listDependencies(rule.root)
		if err != nil {
			return nil, fmt.Errorf("list dependencies for %q: %w", rule.root, err)
		}
		for _, dependency := range dependencies {
			if !dependencyForbiddenByClosureRule(dependency, rule) {
				continue
			}
			violations = append(violations, dependencyClosureViolation{
				root: rule.root, dependency: dependency,
			})
		}
	}
	sort.Slice(violations, func(left int, right int) bool {
		if violations[left].root == violations[right].root {
			return violations[left].dependency < violations[right].dependency
		}
		return violations[left].root < violations[right].root
	})
	return violations, nil
}

func dependencyForbiddenByClosureRule(dependency string, rule dependencyClosureRule) bool {
	for _, prefix := range rule.allowedPrefixes {
		if dependency == prefix || strings.HasPrefix(dependency, prefix+"/") {
			return false
		}
	}
	for _, exact := range rule.forbiddenExact {
		if dependency == exact {
			return true
		}
	}
	for _, prefix := range rule.forbiddenPrefixes {
		if dependency == prefix || strings.HasPrefix(dependency, prefix+"/") {
			return true
		}
	}
	return false
}

func listPackageDependencies(root string) ([]string, error) {
	command := exec.Command("go", "list", "-deps", root)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go list -deps %s: %w: %s", root, err, strings.TrimSpace(string(output)))
	}
	dependencies := strings.Fields(string(output))
	sort.Strings(dependencies)
	return dependencies, nil
}

func filesImporting(root string, target string) ([]string, error) {
	return filesImportingMatching(root, func(importPath string) bool {
		return importPath == target
	})
}

func filesImportingPrefix(root string, target string) ([]string, error) {
	return filesImportingMatching(root, func(importPath string) bool {
		return importPath == target || strings.HasPrefix(importPath, target+"/")
	})
}

func filesImportingMatching(root string, matches func(string) bool) ([]string, error) {
	fset := token.NewFileSet()
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse Go imports in %q: %w", path, err)
		}
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("decode Go import in %q: %w", path, err)
			}
			if matches(importPath) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
