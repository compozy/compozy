package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	runtimeevents "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestManagerStartRunsTypeScriptLifecycleTemplateOverRealStdIO(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "ts-records.jsonl")
	entrypoint := materializeTypeScriptLifecycleObserver(t)

	harness := newManagerHarness(t, managerHarnessSpec{
		Name:         "ts-lifecycle",
		Binary:       entrypoint,
		Capabilities: []Capability{CapabilityRunMutate},
		Hooks:        []HookDeclaration{{Event: HookRunPostShutdown}},
		Env: map[string]string{
			"COMPOZY_TS_RECORD_PATH": recordPath,
		},
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	loaded := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionLoaded)
	ready := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionReady)

	if loaded.Kind != runtimeevents.EventKindExtensionLoaded {
		t.Fatalf("loaded event kind = %q, want %q", loaded.Kind, runtimeevents.EventKindExtensionLoaded)
	}

	var readyPayload kinds.ExtensionReadyPayload
	decodeEventPayload(t, ready, &readyPayload)
	if readyPayload.Extension != "ts-lifecycle" {
		t.Fatalf("ready payload extension = %q, want %q", readyPayload.Extension, "ts-lifecycle")
	}

	harness.manager.DispatchObserver(context.Background(), HookRunPostShutdown, map[string]any{
		"run_id": "run-ts-001",
		"reason": "run_completed",
		"summary": map[string]any{
			"status":     "succeeded",
			"jobs_total": 1,
		},
	})

	records := waitForTSRecords(t, recordPath, 1)
	record := findTSRecord(t, records, "run.post_shutdown")
	if got := record.Payload["run_id"]; got != "run-ts-001" {
		t.Fatalf("run.post_shutdown run_id = %#v, want %q", got, "run-ts-001")
	}
	if got := record.Payload["reason"]; got != "run_completed" {
		t.Fatalf("run.post_shutdown reason = %#v, want %q", got, "run_completed")
	}
	if got := record.Payload["status"]; got != "succeeded" {
		t.Fatalf("run.post_shutdown status = %#v, want %q", got, "succeeded")
	}

	if err := harness.manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func materializeTypeScriptLifecycleObserver(t *testing.T) string {
	t.Helper()

	repoRoot := repoRootForTest(t)
	sdkDir := prepareTypeScriptSDKForLocalInstall(t, repoRoot)
	targetDir := filepath.Join(t.TempDir(), "ts-lifecycle")
	copyDir(
		t,
		filepath.Join(repoRoot, "sdk", "extension-sdk-ts", "templates", "lifecycle-observer"),
		targetDir,
	)
	rewriteTemplateTokensForTest(t, targetDir, map[string]string{
		"__EXTENSION_NAME__":             "ts-lifecycle",
		"__EXTENSION_VERSION__":          "0.1.0",
		"__COMPOZY_MIN_VERSION__":        readSDKPackageVersion(t, repoRoot),
		"__COMPOZY_EXTENSION_SDK_SPEC__": "file:" + sdkDir,
		"__PACKAGE_NAME__":               "ts-lifecycle",
	})

	runCommandForTest(t, targetDir, "npm", "install")
	runCommandForTest(t, targetDir, "npm", "run", "build")

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("look up node: %v", err)
	}

	entrypoint := filepath.Join(targetDir, "run-extension.sh")
	script := fmt.Sprintf("#!/bin/sh\nexec %q %q\n", nodePath, filepath.Join(targetDir, "dist", "src", "index.js"))
	if err := os.WriteFile(entrypoint, []byte(script), 0o755); err != nil {
		t.Fatalf("write wrapper script: %v", err)
	}
	return entrypoint
}

func prepareTypeScriptSDKForLocalInstall(t *testing.T, repoRoot string) string {
	t.Helper()

	sourceDir := filepath.Join(repoRoot, "sdk", "extension-sdk-ts")
	stageRoot := t.TempDir()
	targetDir := filepath.Join(stageRoot, "sdk", "extension-sdk-ts")
	typeScriptSpec := readWorkspaceDevDependencySpec(t, repoRoot, "typescript")
	nodeTypesSpec := readWorkspaceDevDependencySpec(t, repoRoot, "@types/node")

	copyFile(t, filepath.Join(repoRoot, "tsconfig.base.json"), filepath.Join(stageRoot, "tsconfig.base.json"))
	copyFile(t, filepath.Join(sourceDir, "package.json"), filepath.Join(targetDir, "package.json"))
	copyFile(t, filepath.Join(sourceDir, "tsconfig.json"), filepath.Join(targetDir, "tsconfig.json"))
	copyDir(t, filepath.Join(sourceDir, "src"), filepath.Join(targetDir, "src"))

	runCommandForTest(
		t,
		targetDir,
		"npm",
		"install",
		"--no-package-lock",
		"--no-save",
		"typescript@"+typeScriptSpec,
		"@types/node@"+nodeTypesSpec,
	)
	runCommandForTest(t, targetDir, "npm", "run", "build")

	return targetDir
}

type tsRecord struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}

func copyDir(t *testing.T, source string, target string) {
	t.Helper()

	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", target, err)
	}

	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}

		targetPath := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, content, info.Mode())
	}); err != nil {
		t.Fatalf("copy template %s -> %s: %v", source, target, err)
	}
}

func copyFile(t *testing.T, source string, target string) {
	t.Helper()

	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read file %s: %v", source, err)
	}

	info, err := os.Stat(source)
	if err != nil {
		t.Fatalf("stat file %s: %v", source, err)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
	}

	if err := os.WriteFile(target, content, info.Mode()); err != nil {
		t.Fatalf("write file %s: %v", target, err)
	}
}

func rewriteTemplateTokensForTest(t *testing.T, root string, replacements map[string]string) {
	t.Helper()

	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rewritten := string(content)
		for token, value := range replacements {
			rewritten = strings.ReplaceAll(rewritten, token, value)
		}
		return os.WriteFile(path, []byte(rewritten), 0o600)
	}); err != nil {
		t.Fatalf("rewrite template tokens in %s: %v", root, err)
	}
}

func readSDKPackageVersion(t *testing.T, repoRoot string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRoot, "sdk", "extension-sdk-ts", "package.json"))
	if err != nil {
		t.Fatalf("read sdk package.json: %v", err)
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatalf("unmarshal sdk package.json: %v", err)
	}
	if strings.TrimSpace(pkg.Version) == "" {
		t.Fatal("sdk package.json version is empty")
	}
	return pkg.Version
}

func readWorkspaceDevDependencySpec(t *testing.T, repoRoot string, name string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		t.Fatalf("read workspace package.json: %v", err)
	}

	var pkg struct {
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatalf("unmarshal workspace package.json: %v", err)
	}

	spec := strings.TrimSpace(pkg.DevDependencies[name])
	if spec == "" {
		t.Fatalf("workspace package.json devDependency %q is empty", name)
	}
	return spec
}

func repoRootForTest(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	info, statErr := os.Stat(filepath.Join(repoRoot, "go.mod"))
	if statErr != nil || info.IsDir() {
		t.Fatalf("resolve repo root from %s: stat go.mod: %v", wd, statErr)
	}
	return repoRoot
}

func runCommandForTest(t *testing.T, cwd string, name string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

func readTSRecords(t *testing.T, path string) []tsRecord {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("open ts record file: %v", err)
	}
	defer file.Close()

	records := make([]tsRecord, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record tsRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("unmarshal ts record: %v", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan ts records: %v", err)
	}
	return records
}

func waitForTSRecords(t *testing.T, path string, minRecords int) []tsRecord {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		records := readTSRecords(t, path)
		if len(records) >= minRecords {
			return records
		}
		time.Sleep(10 * time.Millisecond)
	}
	return readTSRecords(t, path)
}

func findTSRecord(t *testing.T, records []tsRecord, kind string) tsRecord {
	t.Helper()

	for _, record := range records {
		if record.Kind == kind {
			return record
		}
	}
	t.Fatalf("ts record %q not found in %#v", kind, records)
	return tsRecord{}
}
