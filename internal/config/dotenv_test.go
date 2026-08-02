package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/fileutil"
)

func TestDotEnvParserSanitizesAndRepairsStructuredEntries(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".env")
	contents := strings.Join([]string{
		"# keep comments",
		"COMPOZY_HOME=/tmp/compozy-home",
		"OPENAI_API_KEY=sk-live\u200b ANTHROPIC_API_KEY=anthropic\u2011key",
		`PLAIN_VALUE="hello world"`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("os.WriteFile(.env) error = %v", err)
	}

	report, err := InspectDotEnvFile(path)
	if err != nil {
		t.Fatalf("InspectDotEnvFile() error = %v", err)
	}
	if report.Status != DotEnvStatusRepairable {
		t.Fatalf("InspectDotEnvFile() Status = %q, want %q", report.Status, DotEnvStatusRepairable)
	}
	if len(report.Diagnostics) != 3 {
		t.Fatalf("InspectDotEnvFile() diagnostics = %#v, want multi-key plus two sanitizers", report.Diagnostics)
	}

	repair, err := RepairDotEnvFile(path)
	if err != nil {
		t.Fatalf("RepairDotEnvFile() error = %v", err)
	}
	if repair.Status != DotEnvStatusRepaired || !repair.Repaired {
		t.Fatalf("RepairDotEnvFile() = %#v, want repaired status", repair)
	}

	repaired, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(repaired .env) error = %v", err)
	}
	repairedText := string(repaired)
	for _, want := range []string{
		"# keep comments",
		"COMPOZY_HOME=/tmp/compozy-home",
		"OPENAI_API_KEY=sk-live",
		"ANTHROPIC_API_KEY=anthropickey",
		`PLAIN_VALUE="hello world"`,
	} {
		if !strings.Contains(repairedText, want) {
			t.Fatalf("repaired .env missing %q:\n%s", want, repairedText)
		}
	}
	if strings.Contains(repairedText, "\u200b") || strings.Contains(repairedText, "\u2011") {
		t.Fatalf("repaired .env retained non-ASCII secret characters:\n%s", repairedText)
	}

	parsed := parseDotEnvDocument(repairedText)
	if parsed.unsupported || parsed.needsRepair {
		t.Fatalf("parseDotEnvDocument(repaired) = %#v, want clean parse", parsed)
	}
	wantValues := map[string]string{
		"COMPOZY_HOME":      "/tmp/compozy-home",
		"OPENAI_API_KEY":    "sk-live",
		"ANTHROPIC_API_KEY": "anthropickey",
		"PLAIN_VALUE":       "hello world",
	}
	if !reflect.DeepEqual(parsed.values, wantValues) {
		t.Fatalf("parsed values = %#v, want %#v", parsed.values, wantValues)
	}
}

func TestDotEnvFileTreatsMissingPathAsOptional(t *testing.T) {
	t.Parallel()

	t.Run("Should treat a missing dotenv path as optional", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), ".env")
		report, err := InspectDotEnvFile(path)
		if err != nil {
			t.Fatalf("InspectDotEnvFile(missing) error = %v", err)
		}
		if report.Status != DotEnvStatusMissing {
			t.Fatalf("InspectDotEnvFile(missing) status = %q, want %q", report.Status, DotEnvStatusMissing)
		}

		repair, err := RepairDotEnvFile(path)
		if err != nil {
			t.Fatalf("RepairDotEnvFile(missing) error = %v", err)
		}
		if repair.Status != DotEnvStatusMissing {
			t.Fatalf("RepairDotEnvFile(missing) status = %q, want %q", repair.Status, DotEnvStatusMissing)
		}
	})
}

func TestRepairDotEnvFileTightensRepairedFilePermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should tighten a repaired dotenv file to owner read write", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), ".env")
		contents := "OPENAI_API_KEY=sk-live\u200b ANTHROPIC_API_KEY=anthropic\u2011key\n"
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("os.WriteFile(.env) error = %v", err)
		}

		report, err := RepairDotEnvFile(path)
		if err != nil {
			t.Fatalf("RepairDotEnvFile() error = %v", err)
		}
		if report.Status != DotEnvStatusRepaired || !report.Repaired {
			t.Fatalf("RepairDotEnvFile() = %#v, want repaired status", report)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat(.env) error = %v", err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Fatalf(".env mode = %#o, want %#o", got, want)
		}
	})
}

func TestRepairDotEnvFileRejectsUnsupportedContentWithoutWriting(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".env")
	before := "VALID=value\nsource ./secrets.env\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatalf("os.WriteFile(.env) error = %v", err)
	}

	report, err := RepairDotEnvFile(path)
	if err == nil {
		t.Fatal("RepairDotEnvFile() error = nil, want unsupported content error")
	}
	if !errors.Is(err, ErrDotEnvUnsupported) {
		t.Fatalf("RepairDotEnvFile() error = %v, want ErrDotEnvUnsupported", err)
	}
	if report.Status != DotEnvStatusUnsupported {
		t.Fatalf("RepairDotEnvFile() report = %#v, want unsupported status", report)
	}
	if strings.Contains(err.Error(), "VALID=value") {
		t.Fatalf("RepairDotEnvFile() error leaked .env value: %v", err)
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("os.ReadFile(.env after repair) error = %v", readErr)
	}
	if string(after) != before {
		t.Fatalf(".env changed after unsupported repair\nbefore:\n%s\nafter:\n%s", before, string(after))
	}
}

func TestRepairDotEnvFileRejectsSymlinkWithoutReadingTarget(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "actual.env")
	before := "COMPOZY_TASK09_API_KEY=secret\u200b-value\n"
	if err := os.WriteFile(targetPath, []byte(before), 0o600); err != nil {
		t.Fatalf("os.WriteFile(target .env) error = %v", err)
	}
	linkPath := filepath.Join(dir, ".env")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("os.Symlink(.env) error = %v", err)
	}

	report, err := RepairDotEnvFile(linkPath)
	if err == nil {
		t.Fatal("RepairDotEnvFile(symlink) error = nil, want unsupported symlink")
	}
	if !errors.Is(err, ErrDotEnvUnsupported) {
		t.Fatalf("RepairDotEnvFile(symlink) error = %v, want ErrDotEnvUnsupported", err)
	}
	if report.Status != DotEnvStatusUnsupported {
		t.Fatalf("RepairDotEnvFile(symlink) report = %#v, want unsupported status", report)
	}

	after, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("os.ReadFile(target .env after repair) error = %v", readErr)
	}
	if string(after) != before {
		t.Fatalf("symlink repair changed target .env\nbefore:\n%s\nafter:\n%s", before, string(after))
	}
}

func TestRepairDotEnvFileWritesThroughTheHeldParentAfterPathReplacement(t *testing.T) {
	t.Parallel()
	t.Run("Should repair only the file below the opened dotenv parent", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		root := t.TempDir()
		liveParent := filepath.Join(root, "live")
		archivedParent := filepath.Join(root, "archived")
		externalParent := filepath.Join(root, "external")
		livePath := filepath.Join(liveParent, ".env")
		externalPath := filepath.Join(externalParent, ".env")
		if err := os.MkdirAll(liveParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(live parent) error = %v", err)
		}
		if err := os.MkdirAll(externalParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(external parent) error = %v", err)
		}
		writeFile(t, livePath, "OPENAI_API_KEY=sk-live\u200b ANTHROPIC_API_KEY=anthropic\u2011key\n")
		sentinel := "EXTERNAL_DOTENV_SENTINEL=preserve\n"
		writeFile(t, externalPath, sentinel)

		directory, err := fileutil.OpenDirectory(liveParent)
		if err != nil {
			t.Fatalf("fileutil.OpenDirectory(live parent) error = %v", err)
		}
		defer func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		}()
		if err := os.Rename(liveParent, archivedParent); err != nil {
			t.Fatalf("os.Rename(live parent) error = %v", err)
		}
		if err := os.Symlink(externalParent, liveParent); err != nil {
			t.Fatalf("os.Symlink(external parent) error = %v", err)
		}

		report, err := repairDotEnvFileInDirectory(directory, ".env", livePath)
		if err != nil {
			t.Fatalf("repairDotEnvFileInDirectory() error = %v", err)
		}
		if report.Status != DotEnvStatusRepaired || !report.Repaired {
			t.Fatalf("repairDotEnvFileInDirectory() report = %#v, want repaired", report)
		}
		assertConfigSentinelUnchanged(t, externalPath, sentinel)
		archived, err := os.ReadFile(filepath.Join(archivedParent, ".env"))
		if err != nil {
			t.Fatalf("os.ReadFile(archived dotenv) error = %v", err)
		}
		if strings.Contains(string(archived), "\u200b") || strings.Contains(string(archived), "\u2011") {
			t.Fatalf("archived dotenv remains un-repaired: %q", archived)
		}
	})
}

func TestLoadDotEnvLookupRejectsSymlinkedParentWithoutReadingTarget(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a symlinked dotenv parent without reading the target", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		dir := t.TempDir()
		targetDir := filepath.Join(dir, "actual-workspace")
		targetPath := filepath.Join(targetDir, ".env")
		before := "COMPOZY_DOTENV_PARENT_SECRET=secret\n"
		if err := os.Mkdir(targetDir, 0o755); err != nil {
			t.Fatalf("os.Mkdir(target workspace) error = %v", err)
		}
		if err := os.WriteFile(targetPath, []byte(before), 0o600); err != nil {
			t.Fatalf("os.WriteFile(target .env) error = %v", err)
		}
		linkDir := filepath.Join(dir, "linked-workspace")
		if err := os.Symlink(targetDir, linkDir); err != nil {
			t.Fatalf("os.Symlink(workspace parent) error = %v", err)
		}

		_, err := loadDotEnvLookup(linkDir)
		if err == nil {
			t.Fatal("loadDotEnvLookup(symlinked parent) error = nil, want unsupported symlink")
		}
		if !errors.Is(err, ErrDotEnvUnsupported) {
			t.Fatalf("loadDotEnvLookup(symlinked parent) error = %v, want ErrDotEnvUnsupported", err)
		}
		if strings.Contains(err.Error(), "COMPOZY_DOTENV_PARENT_SECRET") {
			t.Fatalf("loadDotEnvLookup(symlinked parent) error leaked target content: %v", err)
		}

		after, readErr := os.ReadFile(targetPath)
		if readErr != nil {
			t.Fatalf("os.ReadFile(target .env after lookup) error = %v", readErr)
		}
		if string(after) != before {
			t.Fatalf("symlinked parent lookup changed target .env\nbefore:\n%s\nafter:\n%s", before, string(after))
		}
	})
}

func TestLoadDotEnvLookupUsesSanitizedInMemoryValuesWithoutMutatingFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, ".env")
	before := "COMPOZY_CONFIG_TASK09_TOKEN=tok\u200ben OTHER=value\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatalf("os.WriteFile(.env) error = %v", err)
	}

	lookup, err := loadDotEnvLookup(workspace)
	if err != nil {
		t.Fatalf("loadDotEnvLookup() error = %v", err)
	}
	value, ok := lookup("COMPOZY_CONFIG_TASK09_TOKEN")
	if !ok || value != "token" {
		t.Fatalf("lookup(token) = %q, %t; want sanitized token", value, ok)
	}
	other, ok := lookup("OTHER")
	if !ok || other != "value" {
		t.Fatalf("lookup(OTHER) = %q, %t; want value", other, ok)
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("os.ReadFile(.env after load) error = %v", readErr)
	}
	if string(after) != before {
		t.Fatalf("loadDotEnvLookup mutated .env\nbefore:\n%s\nafter:\n%s", before, string(after))
	}
}
