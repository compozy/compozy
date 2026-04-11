package extensions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/core/provider"
)

var (
	sdkReviewExtensionBuildOnce sync.Once
	sdkReviewExtensionBinary    string
	sdkReviewExtensionBuildErr  error

	tsReviewProviderBuildOnce  sync.Once
	tsReviewProviderEntrypoint string
	tsReviewProviderBuildErr   error
)

func TestReviewProviderBridgeRunsGoSDKExtensionOverRealStdIO(t *testing.T) {
	workspaceRoot := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "go-review-records.jsonl")
	entry := goSDKReviewProviderEntry(t, workspaceRoot, recordPath, "")

	bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
	if err != nil {
		t.Fatalf("NewReviewProviderBridge() error = %v", err)
	}
	defer func() {
		if err := bridge.Close(); err != nil {
			t.Fatalf("bridge.Close() error = %v", err)
		}
	}()

	items, err := bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{
		PR:              "123",
		IncludeNitpicks: true,
	})
	if err != nil {
		t.Fatalf("FetchReviews() error = %v", err)
	}
	if len(items) != 1 || items[0].ProviderRef != "thread-go-1" {
		t.Fatalf("FetchReviews() = %#v, want Go SDK review item", items)
	}

	records := waitForRecords(t, recordPath, 1)
	fetchRecord := findRecord(t, records, "fetch_reviews")
	if got := fetchRecord.Payload["pr"]; got != "123" {
		t.Fatalf("fetch record pr = %#v, want %q", got, "123")
	}
	if got := fetchRecord.Payload["include_nitpicks"]; got != true {
		t.Fatalf("fetch record include_nitpicks = %#v, want true", got)
	}

	if err := bridge.ResolveIssues(context.Background(), entry.Name, "123", []provider.ResolvedIssue{{
		FilePath:    "issue_001.md",
		ProviderRef: "thread-go-1",
	}}); err != nil {
		t.Fatalf("ResolveIssues() error = %v", err)
	}

	records = waitForRecords(t, recordPath, 2)
	resolveRecord := findRecord(t, records, "resolve_issues")
	if got := resolveRecord.Payload["pr"]; got != "123" {
		t.Fatalf("resolve record pr = %#v, want %q", got, "123")
	}
}

func TestReviewProviderBridgeRejectsGoSDKMissingProviderRegistration(t *testing.T) {
	workspaceRoot := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "go-review-records.jsonl")
	entry := goSDKReviewProviderEntry(t, workspaceRoot, recordPath, "missing_registration")

	bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
	if err != nil {
		t.Fatalf("NewReviewProviderBridge() error = %v", err)
	}
	defer func() { _ = bridge.Close() }()

	_, err = bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{PR: "123"})
	if err == nil || !strings.Contains(err.Error(), "unsupported_review_provider_contract") {
		t.Fatalf("expected unsupported review provider contract error, got %v", err)
	}
}

func TestReviewProviderBridgeRejectsGoSDKMissingProvidersRegisterCapability(t *testing.T) {
	workspaceRoot := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "go-review-records.jsonl")
	entry := goSDKReviewProviderEntry(t, workspaceRoot, recordPath, "missing_capability")

	bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
	if err != nil {
		t.Fatalf("NewReviewProviderBridge() error = %v", err)
	}
	defer func() { _ = bridge.Close() }()

	_, err = bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{PR: "123"})
	if err == nil || !strings.Contains(err.Error(), "missing_provider_registration_capability") {
		t.Fatalf("expected missing provider registration capability error, got %v", err)
	}
}

func TestReviewProviderBridgeRunsTypeScriptExtensionOverRealStdIO(t *testing.T) {
	workspaceRoot := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "ts-review-records.jsonl")
	entry := typeScriptReviewProviderEntry(t, workspaceRoot, recordPath, "")

	bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
	if err != nil {
		t.Fatalf("NewReviewProviderBridge() error = %v", err)
	}
	defer func() {
		if err := bridge.Close(); err != nil {
			t.Fatalf("bridge.Close() error = %v", err)
		}
	}()

	items, err := bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{
		PR:              "789",
		IncludeNitpicks: true,
	})
	if err != nil {
		t.Fatalf("FetchReviews() error = %v", err)
	}
	if len(items) != 1 || items[0].ProviderRef != "thread-ts-1" {
		t.Fatalf("FetchReviews() = %#v, want TypeScript review item", items)
	}

	records := waitForTSRecords(t, recordPath, 1)
	fetchRecord := findTSRecord(t, records, "fetch_reviews")
	if got := fetchRecord.Payload["pr"]; got != "789" {
		t.Fatalf("fetch record pr = %#v, want %q", got, "789")
	}
	if got := fetchRecord.Payload["include_nitpicks"]; got != true {
		t.Fatalf("fetch record include_nitpicks = %#v, want true", got)
	}

	if err := bridge.ResolveIssues(context.Background(), entry.Name, "789", []provider.ResolvedIssue{{
		FilePath:    "issue_001.md",
		ProviderRef: "thread-ts-1",
	}}); err != nil {
		t.Fatalf("ResolveIssues() error = %v", err)
	}

	records = waitForTSRecords(t, recordPath, 2)
	resolveRecord := findTSRecord(t, records, "resolve_issues")
	if got := resolveRecord.Payload["pr"]; got != "789" {
		t.Fatalf("resolve record pr = %#v, want %q", got, "789")
	}
}

func TestReviewProviderBridgeRejectsTypeScriptMissingProviderRegistration(t *testing.T) {
	workspaceRoot := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "ts-review-records.jsonl")
	entry := typeScriptReviewProviderEntry(t, workspaceRoot, recordPath, "missing_registration")

	bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
	if err != nil {
		t.Fatalf("NewReviewProviderBridge() error = %v", err)
	}
	defer func() { _ = bridge.Close() }()

	_, err = bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{PR: "123"})
	if err == nil || !strings.Contains(err.Error(), "unsupported_review_provider_contract") {
		t.Fatalf("expected unsupported review provider contract error, got %v", err)
	}
}

func TestReviewProviderBridgeRejectsTypeScriptMissingProvidersRegisterCapability(t *testing.T) {
	workspaceRoot := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "ts-review-records.jsonl")
	entry := typeScriptReviewProviderEntry(t, workspaceRoot, recordPath, "missing_capability")

	bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
	if err != nil {
		t.Fatalf("NewReviewProviderBridge() error = %v", err)
	}
	defer func() { _ = bridge.Close() }()

	_, err = bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{PR: "123"})
	if err == nil || !strings.Contains(err.Error(), "missing_provider_registration_capability") {
		t.Fatalf("expected missing provider registration capability error, got %v", err)
	}
}

func goSDKReviewProviderEntry(
	t *testing.T,
	workspaceRoot string,
	recordPath string,
	mode string,
) DeclaredProvider {
	t.Helper()

	providerName := "sdk-review"
	binary := buildSDKReviewExtensionBinary(t)
	return declaredReviewProviderForTest(
		workspaceRoot,
		"sdk-review-ext",
		providerName,
		binary,
		map[string]string{
			"COMPOZY_SDK_RECORD_PATH":     recordPath,
			"COMPOZY_SDK_REVIEW_MODE":     mode,
			"COMPOZY_SDK_REVIEW_PROVIDER": providerName,
		},
	)
}

func typeScriptReviewProviderEntry(
	t *testing.T,
	workspaceRoot string,
	recordPath string,
	mode string,
) DeclaredProvider {
	t.Helper()

	providerName := "ts-review-review"
	entrypoint := buildTypeScriptReviewProviderEntrypoint(t)
	return declaredReviewProviderForTest(
		workspaceRoot,
		"ts-review",
		providerName,
		entrypoint,
		map[string]string{
			"COMPOZY_TS_RECORD_PATH": recordPath,
			"COMPOZY_TS_REVIEW_MODE": mode,
		},
	)
}

func declaredReviewProviderForTest(
	workspaceRoot string,
	extensionName string,
	providerName string,
	command string,
	env map[string]string,
) DeclaredProvider {
	manifest := &Manifest{
		Extension: ExtensionInfo{
			Name:              extensionName,
			Version:           "1.0.0",
			Description:       "Review provider fixture",
			MinCompozyVersion: "0.0.1",
		},
		Subprocess: &SubprocessConfig{
			Command: command,
			Env:     env,
		},
		Security: SecurityConfig{
			Capabilities: []Capability{CapabilityProvidersRegister},
		},
		Providers: ProvidersConfig{
			Review: []ProviderEntry{{
				Name: providerName,
				Kind: ProviderKindExtension,
			}},
		},
	}

	return DeclaredProvider{
		Extension: Ref{
			Name:          extensionName,
			Source:        SourceWorkspace,
			WorkspaceRoot: workspaceRoot,
		},
		ManifestPath: filepath.Join(filepath.Dir(command), ManifestFileNameTOML),
		ExtensionDir: filepath.Dir(command),
		Manifest:     manifest,
		ProviderEntry: ProviderEntry{
			Name: providerName,
			Kind: ProviderKindExtension,
		},
	}
}

func buildSDKReviewExtensionBinary(t *testing.T) string {
	t.Helper()

	sdkReviewExtensionBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "compozy-sdk-review-extension-*")
		if err != nil {
			sdkReviewExtensionBuildErr = err
			return
		}

		binary := filepath.Join(dir, "sdk-review-extension")
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", binary, "./testdata/sdk_review_extension")
		cmd.Dir = "."
		output, err := cmd.CombinedOutput()
		if err != nil {
			sdkReviewExtensionBuildErr = fmt.Errorf("go build sdk review extension: %w: %s", err, output)
			return
		}
		sdkReviewExtensionBinary = binary
	})

	if sdkReviewExtensionBuildErr != nil {
		t.Fatal(sdkReviewExtensionBuildErr)
	}
	return sdkReviewExtensionBinary
}

func buildTypeScriptReviewProviderEntrypoint(t *testing.T) string {
	t.Helper()

	tsReviewProviderBuildOnce.Do(func() {
		repoRoot := repoRootForTest(t)
		runCommandForTest(t, repoRoot, "npm", "run", "build", "--workspace", "@compozy/extension-sdk")
		targetDir := filepath.Join(os.TempDir(), "compozy-ts-review-provider")
		_ = os.RemoveAll(targetDir)
		copyDir(
			t,
			filepath.Join(repoRoot, "sdk", "extension-sdk-ts", "templates", "review-provider"),
			targetDir,
		)
		rewriteTemplateTokensForTest(t, targetDir, map[string]string{
			"__EXTENSION_NAME__":             "ts-review",
			"__EXTENSION_VERSION__":          "0.1.0",
			"__COMPOZY_MIN_VERSION__":        readSDKPackageVersion(t, repoRoot),
			"__COMPOZY_EXTENSION_SDK_SPEC__": "file:" + filepath.Join(repoRoot, "sdk", "extension-sdk-ts"),
			"__PACKAGE_NAME__":               "ts-review",
		})

		runCommandForTest(t, targetDir, "npm", "install")
		runCommandForTest(t, targetDir, "npm", "run", "build")

		nodePath, err := exec.LookPath("node")
		if err != nil {
			tsReviewProviderBuildErr = fmt.Errorf("look up node: %w", err)
			return
		}

		entrypoint := filepath.Join(targetDir, "run-extension.sh")
		script := fmt.Sprintf("#!/bin/sh\nexec %q %q\n", nodePath, filepath.Join(targetDir, "dist", "src", "index.js"))
		if err := os.WriteFile(entrypoint, []byte(script), 0o755); err != nil {
			tsReviewProviderBuildErr = fmt.Errorf("write wrapper script: %w", err)
			return
		}
		tsReviewProviderEntrypoint = entrypoint
	})

	if tsReviewProviderBuildErr != nil {
		t.Fatal(tsReviewProviderBuildErr)
	}
	return tsReviewProviderEntrypoint
}
