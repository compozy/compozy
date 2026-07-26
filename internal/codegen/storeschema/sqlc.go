package storeschema

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

var sqlcPackagePaths = []string{
	"internal/store/globaldb",
	"internal/store/sessiondb",
}

// GenerateSQLC regenerates every store query package with the pinned Go tool.
func GenerateSQLC(ctx context.Context, root string) error {
	for _, packagePath := range sqlcPackagePaths {
		configPath := filepath.Join(root, packagePath, "sqlc.yaml")
		outputPath := filepath.Join(root, packagePath, "sqlcgen")
		if err := runSQLC(ctx, root, configPath, outputPath); err != nil {
			return err
		}
	}
	return nil
}

func runSQLC(ctx context.Context, toolRoot, configPath, outputPath string) error {
	command := exec.CommandContext(ctx, "go", "tool", "sqlc", "generate", "-f", configPath)
	command.Dir = toolRoot
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"codegen: generate sqlc package %q: %w\n%s",
			configPath,
			err,
			strings.TrimSpace(string(output)),
		)
	}
	return modernizeSQLCGenerated(outputPath)
}

// CheckSQLC regenerates into a temporary tree and rejects generated-code or import-ownership drift.
func CheckSQLC(ctx context.Context, root string) (err error) {
	tempRoot, err := os.MkdirTemp("", "agh-sqlc-check-")
	if err != nil {
		return fmt.Errorf("codegen: create sqlc check directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tempRoot))
	}()
	for _, packagePath := range sqlcPackagePaths {
		if err := checkSQLCPackage(ctx, root, tempRoot, packagePath); err != nil {
			return err
		}
	}
	return checkSQLCImportOwnership(root)
}

func checkSQLCPackage(ctx context.Context, root string, tempRoot string, packagePath string) (err error) {
	sourceRoot := filepath.Join(root, packagePath)
	tempPackage := filepath.Join(tempRoot, packagePath)
	if err := os.MkdirAll(tempPackage, 0o755); err != nil {
		return fmt.Errorf("codegen: create temporary sqlc package %q: %w", packagePath, err)
	}
	source, err := os.OpenRoot(sourceRoot)
	if err != nil {
		return fmt.Errorf("codegen: open sqlc source root %q: %w", packagePath, err)
	}
	defer func() {
		err = errors.Join(err, source.Close())
	}()
	destination, err := os.OpenRoot(tempPackage)
	if err != nil {
		return fmt.Errorf("codegen: open temporary sqlc root %q: %w", packagePath, err)
	}
	defer func() {
		err = errors.Join(err, destination.Close())
	}()
	if err := copyFile(source, destination, "sqlc.yaml"); err != nil {
		return err
	}
	if err := copySQLSchemaFiles(source, destination); err != nil {
		return err
	}
	if err := copySQLFiles(source, destination); err != nil {
		return err
	}
	tempOutput := filepath.Join(tempPackage, "sqlcgen")
	if err := runSQLC(ctx, root, filepath.Join(tempPackage, "sqlc.yaml"), tempOutput); err != nil {
		return err
	}
	generatedPath := filepath.Join(packagePath, "sqlcgen")
	if err := compareDirectories(
		filepath.Join(root, generatedPath),
		filepath.Join(tempRoot, generatedPath),
	); err != nil {
		return fmt.Errorf("codegen: sqlc drift in %s: %w; run make codegen", packagePath, err)
	}
	return nil
}

func modernizeSQLCGenerated(outputPath string) (err error) {
	generated, err := os.OpenRoot(outputPath)
	if err != nil {
		return fmt.Errorf("codegen: open generated sqlc package: %w", err)
	}
	defer func() {
		err = errors.Join(err, generated.Close())
	}()
	generatedFS := generated.FS()
	if err := fs.WalkDir(generatedFS, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || path.Ext(name) != ".go" {
			return nil
		}
		contents, err := fs.ReadFile(generatedFS, name)
		if err != nil {
			return fmt.Errorf("read generated file %q: %w", name, err)
		}
		modernized := bytes.ReplaceAll(contents, []byte("interface{}"), []byte("any"))
		formatted, err := format.Source(modernized)
		if err != nil {
			return fmt.Errorf("format modernized generated file %q: %w", name, err)
		}
		if bytes.Equal(contents, formatted) {
			return nil
		}
		if err := generated.WriteFile(name, formatted, 0o600); err != nil {
			return fmt.Errorf("modernize generated file %q: %w", name, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("codegen: modernize sqlc package: %w", err)
	}
	return nil
}

func copySQLFiles(source, destination *os.Root) error {
	entries, err := fs.ReadDir(source.FS(), "queries")
	if err != nil {
		return fmt.Errorf("codegen: read sqlc queries: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != sqlFileExtension {
			continue
		}
		if err := copyFile(source, destination, path.Join("queries", entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copySQLSchemaFiles(source, destination *os.Root) error {
	if err := fs.WalkDir(source.FS(), "schema", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if name == "schema/migrations" {
				return filepath.SkipDir
			}
			return nil
		}
		if path.Ext(name) != sqlFileExtension {
			return nil
		}
		if err := copyFile(source, destination, name); err != nil {
			return fmt.Errorf("copy declarative schema: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk declarative schema: %w", err)
	}
	return nil
}

func copyFile(source, destination *os.Root, name string) error {
	contents, err := source.ReadFile(name)
	if err != nil {
		return fmt.Errorf("codegen: read %q: %w", name, err)
	}
	if err := destination.MkdirAll(path.Dir(name), 0o755); err != nil {
		return fmt.Errorf("codegen: create directory for %q: %w", name, err)
	}
	if err := destination.WriteFile(name, contents, 0o600); err != nil {
		return fmt.Errorf("codegen: write %q: %w", name, err)
	}
	return nil
}

func compareDirectories(expectedDir, actualDir string) error {
	expectedFS := os.DirFS(expectedDir)
	actualFS := os.DirFS(actualDir)
	expected, err := directoryFiles(expectedFS)
	if err != nil {
		return err
	}
	actual, err := directoryFiles(actualFS)
	if err != nil {
		return err
	}
	if !slices.Equal(expected, actual) {
		return fmt.Errorf("generated file set = %v, want %v", expected, actual)
	}
	for _, name := range expected {
		expectedContents, err := fs.ReadFile(expectedFS, name)
		if err != nil {
			return fmt.Errorf("read expected generated file %q: %w", name, err)
		}
		actualContents, err := fs.ReadFile(actualFS, name)
		if err != nil {
			return fmt.Errorf("read actual generated file %q: %w", name, err)
		}
		if !slices.Equal(expectedContents, actualContents) {
			return fmt.Errorf("generated file %q differs", name)
		}
	}
	return nil
}

func directoryFiles(root fs.FS) ([]string, error) {
	files := make([]string, 0)
	err := fs.WalkDir(root, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk generated directory: %w", err)
	}
	slices.Sort(files)
	return files, nil
}

func checkSQLCImportOwnership(root string) error {
	generatedImports := make(map[string]string, len(sqlcPackagePaths))
	for _, packagePath := range sqlcPackagePaths {
		generatedImports[strconv.Quote("github.com/compozy/agh/"+packagePath+"/sqlcgen")] = packagePath
	}
	rootFS := os.DirFS(root)
	return fs.WalkDir(rootFS, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if path.Ext(name) != ".go" {
			return nil
		}
		contents, err := fs.ReadFile(rootFS, name)
		if err != nil {
			return fmt.Errorf("codegen: read Go source %q: %w", name, err)
		}
		for generatedImport, ownerPath := range generatedImports {
			if !strings.Contains(string(contents), generatedImport) {
				continue
			}
			if path.Dir(name) != ownerPath {
				return fmt.Errorf("codegen: sqlcgen import outside owning package %s: %s", ownerPath, name)
			}
		}
		return nil
	})
}
