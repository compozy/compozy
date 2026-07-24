// Suite: public event documentation contract
// Invariant: Every exported EventKind has a contract section in docs/events.md.
// Boundary IN: pkg/compozy/events declarations and docs/events.md.
// Boundary OUT: Event behavior and payload serialization, covered by package tests.
package events

import (
	"go/ast"
	"go/build"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEventsDocumentationEnumeratesAllPublicKinds(t *testing.T) {
	t.Parallel()

	content := readEventsDocumentation(t)
	for _, kind := range readPublicEventKinds(t) {
		want := "### `" + string(kind) + "`"
		if !strings.Contains(content, want) {
			t.Fatalf("expected docs/events.md to define a contract section for %s", kind)
		}
	}
}

func readPublicEventKinds(t *testing.T) []EventKind {
	t.Helper()

	packageDir := currentPackageDir(t)
	fileSet := token.NewFileSet()
	buildPackage, err := build.Default.ImportDir(packageDir, 0)
	if err != nil {
		t.Fatalf("resolve build files in %s: %v", packageDir, err)
	}
	sourceFiles := make([]string, 0, len(buildPackage.GoFiles)+len(buildPackage.CgoFiles))
	sourceFiles = append(sourceFiles, buildPackage.GoFiles...)
	sourceFiles = append(sourceFiles, buildPackage.CgoFiles...)
	files := make([]*ast.File, 0, len(sourceFiles))
	for _, name := range sourceFiles {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(packageDir, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		files = append(files, file)
	}
	typedPackage, err := (&types.Config{Importer: importer.Default()}).Check(
		"github.com/compozy/compozy/pkg/compozy/events",
		fileSet,
		files,
		nil,
	)
	if err != nil {
		t.Fatalf("type-check public event kinds: %v", err)
	}
	eventKindObject := typedPackage.Scope().Lookup("EventKind")
	if eventKindObject == nil {
		t.Fatal("EventKind type not found")
	}

	var kinds []EventKind
	for _, name := range typedPackage.Scope().Names() {
		object := typedPackage.Scope().Lookup(name)
		kindConstant, ok := object.(*types.Const)
		if !ok || !object.Exported() || !types.Identical(kindConstant.Type(), eventKindObject.Type()) {
			continue
		}
		if kindConstant.Val().Kind() != constant.String {
			t.Fatalf("public event kind %s is not a string constant", name)
		}
		kinds = append(kinds, EventKind(constant.StringVal(kindConstant.Val())))
	}
	if len(kinds) == 0 {
		t.Fatal("no public event kinds found")
	}
	return kinds
}

func readEventsDocumentation(t *testing.T) string {
	t.Helper()

	docsPath := filepath.Join(currentPackageDir(t), "..", "..", "..", "docs", "events.md")
	content, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read %s: %v", docsPath, err)
	}
	return string(content)
}

func currentPackageDir(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve docs test file path")
	}
	return filepath.Dir(currentFile)
}
