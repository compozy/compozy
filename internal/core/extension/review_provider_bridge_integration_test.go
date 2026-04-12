package extensions

import (
	"context"
	"errors"
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

type reviewProviderEntryBuilder func(
	t *testing.T,
	workspaceRoot string,
	recordPath string,
	mode string,
) DeclaredProvider

func TestReviewProviderBridgeFetchAndResolveOverRealStdIO(t *testing.T) {
	cases := []struct {
		name            string
		mode            string
		recordFile      string
		wantProviderRef string
		buildEntry      reviewProviderEntryBuilder
		assertRecords   func(t *testing.T, path string, pr string)
	}{
		{
			name:            "ShouldBridgeGoSDKReviewProvidersOverRealStdIO",
			recordFile:      "go-review-records.jsonl",
			wantProviderRef: "thread-go-1",
			buildEntry:      goSDKReviewProviderEntry,
			assertRecords:   assertGoReviewProviderRecords,
		},
		{
			name:            "ShouldBridgeTypeScriptReviewProvidersOverRealStdIO",
			recordFile:      "ts-review-records.jsonl",
			wantProviderRef: "thread-ts-1",
			buildEntry:      typeScriptReviewProviderEntry,
			assertRecords:   assertTypeScriptReviewProviderRecords,
		},
		{
			name:            "ShouldBridgeTypeScriptReviewProvidersThatAutoDeclareRegisterCapability",
			mode:            "missing_capability",
			recordFile:      "ts-review-autocap-records.jsonl",
			wantProviderRef: "thread-ts-1",
			buildEntry:      typeScriptReviewProviderEntry,
			assertRecords:   assertTypeScriptReviewProviderRecords,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			recordPath := filepath.Join(t.TempDir(), tc.recordFile)
			entry := tc.buildEntry(t, workspaceRoot, recordPath, tc.mode)

			bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
			if err != nil {
				t.Fatalf("NewReviewProviderBridge() error = %v", err)
			}
			defer func() {
				if err := bridge.Close(); err != nil {
					t.Fatalf("bridge.Close() error = %v", err)
				}
			}()

			const pr = "123"
			items, err := bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{
				PR:              pr,
				IncludeNitpicks: true,
			})
			if err != nil {
				t.Fatalf("FetchReviews() error = %v", err)
			}
			if len(items) != 1 || items[0].ProviderRef != tc.wantProviderRef {
				t.Fatalf("FetchReviews() = %#v, want provider_ref %q", items, tc.wantProviderRef)
			}

			if err := bridge.ResolveIssues(context.Background(), entry.Name, pr, []provider.ResolvedIssue{{
				FilePath:    "issue_001.md",
				ProviderRef: tc.wantProviderRef,
			}}); err != nil {
				t.Fatalf("ResolveIssues() error = %v", err)
			}

			tc.assertRecords(t, recordPath, pr)
		})
	}
}

func TestReviewProviderBridgeRejectsInvalidExtensionContracts(t *testing.T) {
	cases := []struct {
		name       string
		recordFile string
		mode       string
		wantErr    string
		buildEntry reviewProviderEntryBuilder
	}{
		{
			name:       "ShouldRejectGoSDKProvidersMissingRegistration",
			recordFile: "go-review-records.jsonl",
			mode:       "missing_registration",
			wantErr:    "unsupported_review_provider_contract",
			buildEntry: goSDKReviewProviderEntry,
		},
		{
			name:       "ShouldRejectGoSDKProvidersMissingRegisterCapability",
			recordFile: "go-review-records.jsonl",
			mode:       "missing_capability",
			wantErr:    "missing_provider_registration_capability",
			buildEntry: goSDKReviewProviderEntry,
		},
		{
			name:       "ShouldRejectTypeScriptProvidersMissingRegistration",
			recordFile: "ts-review-records.jsonl",
			mode:       "missing_registration",
			wantErr:    "unsupported_review_provider_contract",
			buildEntry: typeScriptReviewProviderEntry,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			recordPath := filepath.Join(t.TempDir(), tc.recordFile)
			entry := tc.buildEntry(t, workspaceRoot, recordPath, tc.mode)

			bridge, err := NewReviewProviderBridge(entry, workspaceRoot, "fetch-reviews")
			if err != nil {
				t.Fatalf("NewReviewProviderBridge() error = %v", err)
			}
			defer func() { _ = bridge.Close() }()

			_, err = bridge.FetchReviews(context.Background(), entry.Name, provider.FetchRequest{PR: "123"})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestLookupActiveExtensionSessionFailsClosedWhenMatchesAreAmbiguous(t *testing.T) {
	workspaceRoot := t.TempDir()

	t.Run("ShouldReturnTheUniqueNormalizedReadyMatch", func(t *testing.T) {
		session := readySessionForTest(" ext-review ")
		setActiveManagersForTest(t, &Manager{
			workspaceRoot: workspaceRoot,
			sessions: map[string]*extensionSession{
				normalizeSessionKey(session.runtime.normalizedName()): session,
			},
		})

		got := lookupActiveExtensionSession(workspaceRoot, "ext-review")
		if got != session {
			t.Fatalf("lookupActiveExtensionSession() = %#v, want %#v", got, session)
		}
	})

	t.Run("ShouldReturnNilWhenMultipleManagersMatchTheSameWorkspaceAndExtension", func(t *testing.T) {
		first := readySessionForTest("ext-review")
		second := readySessionForTest("ext-review")
		setActiveManagersForTest(t,
			&Manager{
				workspaceRoot: workspaceRoot,
				sessions: map[string]*extensionSession{
					normalizeSessionKey(first.runtime.normalizedName()): first,
				},
			},
			&Manager{
				workspaceRoot: workspaceRoot,
				sessions: map[string]*extensionSession{
					normalizeSessionKey(second.runtime.normalizedName()): second,
				},
			},
		)

		if got := lookupActiveExtensionSession(workspaceRoot, "ext-review"); got != nil {
			t.Fatalf("lookupActiveExtensionSession() = %#v, want nil for ambiguous match", got)
		}
	})
}

func TestRuntimeExtensionFromDeclaredProviderInitializesLoadedRuntimeState(t *testing.T) {
	t.Parallel()

	timeout := 12 * defaultExtensionShutdown
	entry := declaredReviewProviderForTest(
		t.TempDir(),
		"declared-review-ext",
		"declared-review",
		"/bin/declared-review",
		nil,
	)
	entry.Manifest.Subprocess.ShutdownTimeout = timeout

	runtimeExtension, err := runtimeExtensionFromDeclaredProvider(entry)
	if err != nil {
		t.Fatalf("runtimeExtensionFromDeclaredProvider() error = %v", err)
	}
	if got := runtimeExtension.State(); got != ExtensionStateLoaded {
		t.Fatalf("runtimeExtension.State() = %q, want %q", got, ExtensionStateLoaded)
	}
	if got := runtimeExtension.ShutdownDeadline(); got != timeout {
		t.Fatalf("runtimeExtension.ShutdownDeadline() = %s, want %s", got, timeout)
	}
}

func TestMissingRegisteredSessionErrorPreservesRegistrationFailureAsPrimaryCause(t *testing.T) {
	t.Parallel()

	cleanupErr := errors.New("cleanup failed")
	err := missingRegisteredSessionError("ext-review", cleanupErr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `session was not registered`) {
		t.Fatalf("expected registration failure in error text, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), `shutdown during cleanup: cleanup failed`) {
		t.Fatalf("expected cleanup failure context in error text, got %q", err.Error())
	}

	cause := errors.Unwrap(err)
	if cause == nil || !strings.Contains(cause.Error(), `session was not registered`) {
		t.Fatalf("expected wrapped cause to remain the registration failure, got %#v", cause)
	}
}

func readySessionForTest(name string) *extensionSession {
	runtime := &RuntimeExtension{Name: name}
	runtime.SetState(ExtensionStateReady)
	return &extensionSession{runtime: runtime}
}

func setActiveManagersForTest(t *testing.T, managers ...*Manager) {
	t.Helper()

	replacement := make(map[*Manager]struct{}, len(managers))
	for _, manager := range managers {
		replacement[manager] = struct{}{}
	}

	activeManagers.mu.Lock()
	original := activeManagers.managers
	activeManagers.managers = replacement
	activeManagers.mu.Unlock()

	t.Cleanup(func() {
		activeManagers.mu.Lock()
		activeManagers.managers = original
		activeManagers.mu.Unlock()
	})
}

func assertGoReviewProviderRecords(t *testing.T, path string, wantPR string) {
	t.Helper()

	records := waitForRecords(t, path, 2)
	fetchRecord := findRecord(t, records, "fetch_reviews")
	if got := fetchRecord.Payload["pr"]; got != wantPR {
		t.Fatalf("fetch record pr = %#v, want %q", got, wantPR)
	}
	if got := fetchRecord.Payload["include_nitpicks"]; got != true {
		t.Fatalf("fetch record include_nitpicks = %#v, want true", got)
	}

	resolveRecord := findRecord(t, records, "resolve_issues")
	if got := resolveRecord.Payload["pr"]; got != wantPR {
		t.Fatalf("resolve record pr = %#v, want %q", got, wantPR)
	}
}

func assertTypeScriptReviewProviderRecords(t *testing.T, path string, wantPR string) {
	t.Helper()

	records := waitForTSRecords(t, path, 2)
	fetchRecord := findTSRecord(t, records, "fetch_reviews")
	if got := fetchRecord.Payload["pr"]; got != wantPR {
		t.Fatalf("fetch record pr = %#v, want %q", got, wantPR)
	}
	if got := fetchRecord.Payload["include_nitpicks"]; got != true {
		t.Fatalf("fetch record include_nitpicks = %#v, want true", got)
	}

	resolveRecord := findTSRecord(t, records, "resolve_issues")
	if got := resolveRecord.Payload["pr"]; got != wantPR {
		t.Fatalf("resolve record pr = %#v, want %q", got, wantPR)
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
		sdkDir := stageTypeScriptSDKForReviewProvider(t, repoRoot)
		targetDir, err := os.MkdirTemp("", "compozy-ts-review-provider-*")
		if err != nil {
			tsReviewProviderBuildErr = fmt.Errorf("create ts review provider dir: %w", err)
			return
		}
		copyDir(
			t,
			filepath.Join(repoRoot, "sdk", "extension-sdk-ts", "templates", "review-provider"),
			targetDir,
		)
		rewriteTemplateTokensForTest(t, targetDir, map[string]string{
			"__EXTENSION_NAME__":             "ts-review",
			"__EXTENSION_VERSION__":          "0.1.0",
			"__COMPOZY_MIN_VERSION__":        readSDKPackageVersion(t, repoRoot),
			"__COMPOZY_EXTENSION_SDK_SPEC__": "file:" + sdkDir,
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

func stageTypeScriptSDKForReviewProvider(t *testing.T, repoRoot string) string {
	t.Helper()

	sourceDir := filepath.Join(repoRoot, "sdk", "extension-sdk-ts")
	stageRoot, err := os.MkdirTemp("", "compozy-ts-review-sdk-*")
	if err != nil {
		t.Fatalf("create staged ts sdk root: %v", err)
	}
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
