package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func TestExtensionSearchCommandUsesDaemonClient(t *testing.T) {
	t.Parallel()

	called := false
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		searchExtensionsFn: func(
			_ context.Context,
			request ExtensionSearchRequest,
		) (ExtensionSearchRecord, error) {
			called = true
			if request.Query != "utility" || request.Limit != 7 {
				t.Fatalf("SearchExtensions(%#v), want utility limit 7", request)
			}
			if request.Cursor != "next" {
				t.Fatalf("SearchExtensions cursor = %q, want next", request.Cursor)
			}
			if !reflect.DeepEqual(request.Sources, []string{"curated", "github"}) {
				t.Fatalf("SearchExtensions sources = %#v, want curated/github", request.Sources)
			}
			return ExtensionSearchRecord{
				Items: []contract.ExtensionSearchItem{{
					Slug:        "acme/utility-ext",
					Name:        "utility-ext",
					Description: "Utility extension",
					Author:      "acme",
					Version:     "1.0.0",
					Source:      "curated",
					Tier:        "official",
					Integrity:   "catalog_digest",
				}},
				NextCursor:      "page-two",
				SourcesDegraded: []string{"github"},
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"search",
		"utility",
		"--limit",
		"7",
		"--cursor",
		"next",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension search error = %v", err)
	}
	if !called {
		t.Fatal("SearchExtensions was not called")
	}
	var payload ExtensionSearchRecord
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(search) error = %v; stdout=%s", err, stdout)
	}
	if len(payload.Items) != 1 || payload.Items[0].Slug != "acme/utility-ext" ||
		payload.NextCursor != "page-two" || !reflect.DeepEqual(payload.SourcesDegraded, []string{"github"}) {
		t.Fatalf("search payload = %#v, want utility-ext", payload)
	}
}

func TestExtensionSearchCommandRejectsNonPositiveLimit(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})
	markExtensionDaemonRunning(&deps)
	_, _, err := executeRootCommand(t, deps, "extension", "search", "utility", "--limit", "0")
	if err == nil || !strings.Contains(err.Error(), "search limit must be positive") {
		t.Fatalf("extension search --limit=0 error = %v, want limit validation", err)
	}
}

func TestExtensionInstallCommandUsesDaemonForMarketplaceInstalls(t *testing.T) {
	t.Parallel()

	var captured InstallExtensionRequest
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		installExtensionFn: func(_ context.Context, request InstallExtensionRequest) (ExtensionRecord, error) {
			captured = request
			return ExtensionRecord{
				Name:          "install-ext",
				Version:       "1.2.0",
				Source:        extensionpkg.SourceMarketplace.String(),
				State:         "active",
				DaemonRunning: true,
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"install",
		"github:acme/install-ext",
		"--version",
		"1.2.0",
		"--asset",
		"darwin-arm64",
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension install marketplace error = %v", err)
	}
	want := InstallExtensionRequest{
		Source:          contract.InstallExtensionSourceGitHub,
		Ref:             "acme/install-ext",
		Version:         "1.2.0",
		Asset:           "darwin-arm64",
		AllowUnverified: true,
	}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("InstallExtension request = %#v, want %#v", captured, want)
	}
	var payload ExtensionRecord
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(install) error = %v; stdout=%s", err, stdout)
	}
	if payload.Name != "install-ext" || payload.Source != extensionpkg.SourceMarketplace.String() {
		t.Fatalf("install payload = %#v, want marketplace extension", payload)
	}
}

func TestExtensionInstallUsesReachableDaemonWhenProcessTimestampMetadataLags(t *testing.T) {
	t.Parallel()

	installCalled := false
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		previewExtensionInstallFn: func(context.Context, InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
			return ExtensionInstallPreviewRecord{Name: "late-boot-ext", DigestSHA256: strings.Repeat("a", 64)}, nil
		},
		daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			return DaemonStatus{Status: "running", PID: 999}, nil
		},
		installExtensionFn: func(_ context.Context, request InstallExtensionRequest) (ExtensionRecord, error) {
			installCalled = true
			return ExtensionRecord{
				Name:          "late-boot-ext",
				Version:       request.Version,
				Source:        extensionpkg.SourceMarketplace.String(),
				State:         "active",
				DaemonRunning: true,
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)
	deps.processMatchesStartTime = func(int, time.Time) bool { return false }

	_, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"install",
		"acme/late-boot-ext",
		"--version",
		"1.0.0",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension install marketplace error = %v", err)
	}
	if !installCalled {
		t.Fatal("InstallExtension was not called through the reachable daemon")
	}
}

// Invariant: the CLI pins the inspected acquisition and preserves manifest types and vault references.
// Owner: CLI command boundary. Canonical suite: extension_marketplace_test.go.
func TestExtensionInstallCommandInputs(t *testing.T) {
	t.Parallel()
	t.Run("Should pin the inspected digest and merge typed flags over input file values", func(t *testing.T) {
		t.Parallel()
		inputFile := filepath.Join(t.TempDir(), "inputs.json")
		if err := os.WriteFile(
			inputFile,
			[]byte(
				`{"label":{"value":"from file"},"token":{"vault_ref":"vault:extensions/global/pf-default/install-ext/env/TOKEN"}}`,
			),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		digest := strings.Repeat("a", 64)
		var captured InstallExtensionRequest
		previewCalls := 0
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			previewExtensionInstallFn: func(_ context.Context, req InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
				previewCalls++
				if len(req.Inputs) != 0 || req.ExpectedDigest != "" {
					t.Fatal("metadata request should not send input values or an unreviewed pin")
				}
				return ExtensionInstallPreviewRecord{
					Name:         "typed",
					DigestSHA256: digest,
					Inputs: []contract.MarketplaceInputPayload{
						{ID: "label", Type: "string"},
						{ID: "enabled", Type: "boolean"},
					},
				}, nil
			},
			installExtensionFn: func(_ context.Context, req InstallExtensionRequest) (ExtensionRecord, error) {
				captured = req
				return ExtensionRecord{Name: "typed"}, nil
			},
		})
		markExtensionDaemonRunning(&deps)
		_, stderr, err := executeRootCommand(
			t,
			deps,
			"extension",
			"install",
			"compozy/typed",
			"--input-file",
			inputFile,
			"--input",
			"label=true",
			"--input",
			"enabled=false",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatal(err)
		}
		if previewCalls != 1 || stderr != "" || captured.ExpectedDigest != digest ||
			captured.Source != contract.InstallExtensionSourceCurated {
			t.Fatalf(
				"preview calls=%d, stderr=%q, digest=%q, source=%q",
				previewCalls,
				stderr,
				captured.ExpectedDigest,
				captured.Source,
			)
		}
		if string(captured.Inputs["label"].Value) != `"true"` || string(captured.Inputs["enabled"].Value) != "false" {
			t.Fatal("CLI did not preserve string and boolean types")
		}
		if ref := captured.Inputs["token"].VaultRef; ref == nil ||
			*ref != "vault:extensions/global/pf-default/install-ext/env/TOKEN" {
			t.Fatal("CLI did not preserve the vault reference")
		}
	})
	t.Run("Should keep canonical catalog lookup failures on the curated source", func(t *testing.T) {
		t.Parallel()
		calls := 0
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			previewExtensionInstallFn: func(_ context.Context, req InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
				calls++
				if req.Source != contract.InstallExtensionSourceCurated {
					t.Fatal("canonical request fell through to GitHub")
				}
				return ExtensionInstallPreviewRecord{}, &daemonAPIError{
					statusCode: 404,
					status:     "missing catalog entry",
				}
			},
			installExtensionFn: func(context.Context, InstallExtensionRequest) (ExtensionRecord, error) {
				t.Fatal("missing canonical entry dispatched installation")
				return ExtensionRecord{}, nil
			},
		})
		markExtensionDaemonRunning(&deps)
		_, _, err := executeRootCommand(t, deps, "extension", "install", "compozy/missing", "-o", "json")
		if err == nil || calls != 1 {
			t.Fatalf("canonical lookup: calls=%d error=%v", calls, err)
		}
	})
	t.Run("Should reject the retired runtime-name flag before contacting the daemon", func(t *testing.T) {
		t.Parallel()
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			previewExtensionInstallFn: func(context.Context, InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
				t.Fatal("retired flag dispatched inspection")
				return ExtensionInstallPreviewRecord{}, nil
			},
			installExtensionFn: func(context.Context, InstallExtensionRequest) (ExtensionRecord, error) {
				t.Fatal("retired flag dispatched installation")
				return ExtensionRecord{}, nil
			},
		})
		markExtensionDaemonRunning(&deps)
		_, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"install",
			"compozy/pinned",
			"--runtime-name",
			"pinned-server",
			"-o",
			"json",
		)
		if err == nil || !strings.Contains(err.Error(), "unknown flag: --runtime-name") {
			t.Fatalf("retired flag: %v", err)
		}
	})
	t.Run("Should preserve an explicit digest pin during inspection and installation", func(t *testing.T) {
		t.Parallel()
		digest := strings.Repeat("b", 64)
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			previewExtensionInstallFn: func(_ context.Context, req InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
				if req.ExpectedDigest != digest {
					t.Fatal("inspection lost the reviewed digest")
				}
				return ExtensionInstallPreviewRecord{DigestSHA256: digest}, nil
			},
			installExtensionFn: func(_ context.Context, req InstallExtensionRequest) (ExtensionRecord, error) {
				if req.ExpectedDigest != digest {
					t.Fatal("installation lost the reviewed digest")
				}
				return ExtensionRecord{Name: "pinned"}, nil
			},
		})
		markExtensionDaemonRunning(&deps)
		if _, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"install",
			"compozy/pinned",
			"--expected-digest",
			digest,
			"-o",
			"json",
		); err != nil {
			t.Fatal(err)
		}
	})
}

// Invariant: an extension_inputs_required refusal prompts on a terminal and retries once with the merged
// values; without a terminal or with structured output it keeps the --input guidance and exit code 2.
// Owner: CLI install boundary (T-12). Canonical suite: extension_marketplace_test.go.
func TestExtensionInstallPromptsForRequiredInputs(t *testing.T) {
	t.Parallel()
	refusal := func() error {
		return &extensionOperationAPIError{
			statusCode: 422,
			status:     "422 Unprocessable Entity",
			payload: contract.ExtensionOperationErrorPayload{
				Error:  "extension: inputs required",
				Code:   diagnosticcontract.CodeExtensionInputsRequired,
				Inputs: []string{"token"},
				InputDefinitions: []contract.MarketplaceInputPayload{
					{ID: "label", Prompt: "Label", Type: "string"},
					{ID: "token", Prompt: "API token", Type: "secret", Required: true},
				},
			},
		}
	}
	newDeps := func(t *testing.T, terminal bool, calls *[]InstallExtensionRequest) commandDeps {
		t.Helper()
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			previewExtensionInstallFn: func(context.Context, InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
				return ExtensionInstallPreviewRecord{
					Name:         "typed",
					DigestSHA256: strings.Repeat("c", 64),
					Inputs: []contract.MarketplaceInputPayload{
						{ID: "label", Type: "string"},
						{ID: "token", Type: "secret", Required: true},
					},
				}, nil
			},
			installExtensionFn: func(_ context.Context, req InstallExtensionRequest) (ExtensionRecord, error) {
				*calls = append(*calls, req)
				if len(*calls) == 1 {
					return ExtensionRecord{}, refusal()
				}
				return ExtensionRecord{Name: "typed"}, nil
			},
		})
		markExtensionDaemonRunning(&deps)
		deps.inputIsTerminal = func(io.Reader) bool { return terminal }
		deps.outputIsTerminal = func(io.Writer) bool { return terminal }
		return deps
	}

	t.Run("Should prompt for the listed inputs and retry with the merged values", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		deps := newDeps(t, true, &calls)
		_, stderr, err := executeRootCommandWithInput(
			t, deps, "s3cret-token\n", "extension", "install", "compozy/typed", "--input", "label=team",
		)
		if err != nil {
			t.Fatalf("install with prompt error = %v", err)
		}
		if len(calls) != 2 {
			t.Fatalf("install calls = %d, want a refusal and one retry", len(calls))
		}
		retry := calls[1].Inputs
		if string(retry["token"].Value) != `"s3cret-token"` || string(retry["label"].Value) != `"team"` {
			t.Fatalf("retry inputs = %#v, want prompted token merged with --input label", retry)
		}
		if !strings.Contains(stderr, "API token (secret): ") || strings.Contains(stderr, "Label") {
			t.Fatalf("prompt output = %q, want only the listed secret input", stderr)
		}
		if strings.Contains(stderr, "s3cret-token") {
			t.Fatal("prompt output echoed the secret value")
		}
	})

	for _, testCase := range []struct {
		name     string
		terminal bool
		args     []string
	}{
		{name: "without a terminal", terminal: false},
		{name: "with JSON output", terminal: true, args: []string{"-o", "json"}},
	} {
		t.Run("Should keep the input guidance and exit code "+testCase.name, func(t *testing.T) {
			t.Parallel()
			var calls []InstallExtensionRequest
			deps := newDeps(t, testCase.terminal, &calls)
			args := append([]string{"extension", "install", "compozy/typed"}, testCase.args...)
			_, stderr, err := executeRootCommandWithInput(t, deps, "unused\n", args...)
			if err == nil || len(calls) != 1 || strings.Contains(stderr, "(secret)") {
				t.Fatalf("calls=%d stderr=%q err=%v, want one refused call without prompt", len(calls), stderr, err)
			}
			if code := cliExitCodeForError(err); code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
			want := "extension inputs required: token. Pass --input id=value or --input-file inputs.json."
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %q, want unchanged guidance", err)
			}
		})
	}

	t.Run("Should not retry when the prompt is aborted", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		deps := newDeps(t, true, &calls)
		_, _, err := executeRootCommandWithInput(t, deps, "", "extension", "install", "compozy/typed")
		if !errors.Is(err, errExtensionInputPromptAborted) || len(calls) != 1 {
			t.Fatalf("aborted prompt: calls=%d err=%v, want abort without retry", len(calls), err)
		}
	})
}

func TestExtensionMarketplaceInstallRequiresDaemon(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})
	_, _, err := executeRootCommand(t, deps, "extension", "install", "acme/install-ext", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "running daemon") {
		t.Fatalf("extension install marketplace offline error = %v, want running daemon requirement", err)
	}
}

func TestExtensionAllowUnverifiedStructuredOutputRequiresYes(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})
	dir := writeExtensionFixture(t, "trust-ext", extensionFixtureOptions{})
	_, _, err := executeRootCommand(t, deps, "extension", "install", dir, "--allow-unverified", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("extension install --allow-unverified without --yes error = %v, want --yes requirement", err)
	}
}

func TestExtensionRemoveCommandUsesDaemonClient(t *testing.T) {
	t.Parallel()

	var removedName string
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		removeExtensionFn: func(_ context.Context, name string) (ManagedExtensionRemoveRecord, error) {
			removedName = name
			return ManagedExtensionRemoveRecord{Name: name, Path: "/tmp/remove-ext", Status: "removed"}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"remove",
		"remove-ext",
		"--global",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension remove error = %v", err)
	}
	if removedName != "remove-ext" {
		t.Fatalf("RemoveExtension name = %q, want remove-ext", removedName)
	}
	var payload ManagedExtensionRemoveRecord
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(remove) error = %v; stdout=%s", err, stdout)
	}
	if payload.Status != "removed" {
		t.Fatalf("remove payload = %#v, want removed", payload)
	}
}

func TestExtensionUpdateCommandUsesDaemonClient(t *testing.T) {
	t.Parallel()
	// Invariant: both update entry points forward the selected workspace/profile without changing selectors.
	// Owner: CLI request construction; canonical suite: TestExtensionUpdateCommandUsesDaemonClient.
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should forward workspace and profile selectors with all=%t", all), func(t *testing.T) {
			t.Parallel()
			called := false
			assertScope := func(scope, workspaceID, profile string) {
				t.Helper()
				called = true
				if scope != "workspace" || workspaceID != "ws-alpha" || profile != "marketing" {
					t.Fatalf("update scope = %s/%s/%s", scope, workspaceID, profile)
				}
			}
			deps, _ := newExtensionLocalDeps(t, &stubClient{
				getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
					if ref != "alpha" {
						t.Fatalf("workspace lookup = %q", ref)
					}
					return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
				},
				listProfilesFn: func(context.Context) ([]contract.Profile, error) {
					return []contract.Profile{{ID: "marketing-id", Name: "marketing", State: "active"}}, nil
				},
				updateExtensionFn: func(_ context.Context, name string, request UpdateExtensionRequest) (ExtensionUpdateRecord, error) {
					assertScope(request.Scope, request.WorkspaceID, request.Profile)
					return ExtensionUpdateRecord{Name: name}, nil
				},
				updateExtensionsFn: func(_ context.Context, request UpdateExtensionsRequest) ([]ExtensionUpdateRecord, error) {
					assertScope(request.Scope, request.WorkspaceID, request.Profile)
					if !request.All {
						t.Fatal("batch must preserve all selector")
					}
					return []ExtensionUpdateRecord{}, nil
				},
			})
			markExtensionDaemonRunning(&deps)
			args := []string{
				"extension",
				"update",
				"--check",
				"--workspace",
				"alpha",
				"--profile",
				"marketing",
				"-o",
				"json",
			}
			if all {
				args = append(args, "--all")
			} else {
				args = append(args, "tool-ext")
			}
			if _, _, err := executeRootCommand(t, deps, args...); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("update request was not dispatched")
			}
		})
	}

	t.Run("Should update one extension through the daemon client", func(t *testing.T) {
		t.Parallel()

		var capturedName string
		var capturedRequest UpdateExtensionRequest
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			updateExtensionFn: func(
				_ context.Context,
				name string,
				request UpdateExtensionRequest,
			) (ExtensionUpdateRecord, error) {
				capturedName = name
				capturedRequest = request
				return ExtensionUpdateRecord{
					Name:           name,
					Slug:           "acme/" + name,
					Registry:       "github",
					CurrentVersion: "1.0.0",
					LatestVersion:  "1.2.0",
					Path:           "/tmp/" + name,
					Status:         extensionpkg.MarketplaceUpdateStatusUpdated,
					Warnings: []contract.DiagnosticItem{{
						Code: contract.CodeExtensionUpdateCleanupFailed,
					}},
				}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"update",
			"update-ext",
			"--version",
			"1.2.0",
			"--allow-unverified",
			"--yes",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension update error = %v", err)
		}
		if capturedName != "update-ext" {
			t.Fatalf("UpdateExtension name = %q, want update-ext", capturedName)
		}
		want := UpdateExtensionRequest{Version: "1.2.0", AllowUnverified: true}
		if !reflect.DeepEqual(capturedRequest, want) {
			t.Fatalf("UpdateExtension request = %#v, want %#v", capturedRequest, want)
		}
		var payload []extensionUpdateItem
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(update) error = %v; stdout=%s", err, stdout)
		}
		if len(payload) != 1 || payload[0].Status != extensionpkg.MarketplaceUpdateStatusUpdated {
			t.Fatalf("update payload = %#v, want updated", payload)
		}
		if len(payload[0].Warnings) != 1 || payload[0].Warnings[0].Code != contract.CodeExtensionUpdateCleanupFailed {
			t.Fatalf("update payload warnings = %#v, want cleanup warning", payload[0].Warnings)
		}
		humanOut, _, err := executeRootCommand(t, deps, "extension", "update", "update-ext")
		if err != nil {
			t.Fatalf("extension update human output error = %v", err)
		}
		if !strings.Contains(humanOut, contract.CodeExtensionUpdateCleanupFailed) {
			t.Fatalf("extension update human output = %q, want cleanup warning code", humanOut)
		}
	})
}

func TestExtensionUpdateCheckDoesNotRequireAllowUnverified(t *testing.T) {
	t.Parallel()
	t.Run("Should keep check-only updates free of unverified-install consent", func(t *testing.T) {
		t.Parallel()

		var capturedName string
		var captured UpdateExtensionRequest
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			updateExtensionFn: func(
				_ context.Context,
				name string,
				request UpdateExtensionRequest,
			) (ExtensionUpdateRecord, error) {
				capturedName = name
				captured = request
				return ExtensionUpdateRecord{
					Name:           name,
					Slug:           "acme/" + name,
					Registry:       "github",
					CurrentVersion: "1.0.0",
					LatestVersion:  "1.2.0",
					Status:         extensionpkg.MarketplaceUpdateStatusAvailable,
				}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "extension", "update", "update-ext", "--check", "-o", "json")
		if err != nil {
			t.Fatalf("extension update --check error = %v", err)
		}
		if capturedName != "update-ext" {
			t.Fatalf("UpdateExtension name = %q, want update-ext", capturedName)
		}
		want := UpdateExtensionRequest{CheckOnly: true}
		if !reflect.DeepEqual(captured, want) {
			t.Fatalf("UpdateExtension check request = %#v, want %#v", captured, want)
		}
		var payload []extensionUpdateItem
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(update check) error = %v; stdout=%s", err, stdout)
		}
		if len(payload) != 1 || payload[0].Status != extensionpkg.MarketplaceUpdateStatusAvailable {
			t.Fatalf("update check payload = %#v, want available", payload)
		}
	})
}

func TestExtensionUpdateAllUsesOneDaemonBatch(t *testing.T) {
	t.Parallel()

	calls := 0
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		updateExtensionsFn: func(
			_ context.Context,
			request UpdateExtensionsRequest,
		) ([]ExtensionUpdateRecord, error) {
			calls++
			if !request.All || !request.CheckOnly || len(request.Names) != 0 {
				t.Fatalf("UpdateExtensions request = %#v, want all check-only", request)
			}
			return []ExtensionUpdateRecord{{
				Name:           "market-ext",
				Slug:           "acme/market-ext",
				Registry:       "github",
				CurrentVersion: "1.0.0",
				LatestVersion:  "1.2.0",
				Status:         extensionpkg.MarketplaceUpdateStatusAvailable,
			}, {
				Name: "failed-ext", Status: extensionpkg.MarketplaceUpdateStatusFailed,
			}}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	if _, _, err := executeRootCommand(t, deps, "extension", "update", "--all", "--check", "-o", "json"); err != nil {
		t.Fatalf("extension update --all --check error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("UpdateExtensions calls = %d, want one batch", calls)
	}
}

func markExtensionDaemonRunning(deps *commandDeps) {
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 999, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(int) bool { return true }
}

// Invariant: named plugin sources cannot take over curated references or activate GitHub fallback.
// Owner: CLI acquisition selection; canonical extension marketplace suite.
func TestPluginSourceInstallSelection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, ref  string
		curated    bool
		wantSource contract.InstallExtensionSource
		wantErr    bool
	}{
		{"Should preserve curated reference priority", "team/tool", true, contract.InstallExtensionSourceCurated, false},
		{"Should explicitly select a plugin", "marketplace:team/tool", false, contract.InstallExtensionSourceMarketplace, false},
		{"Should reject an unknown explicit source", "marketplace:missing/tool", false, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &stubClient{
				listMarketplaceSourcesFn: func(context.Context) (contract.MarketplaceSourcesResponse, error) {
					return contract.MarketplaceSourcesResponse{Sources: []contract.MarketplaceSourcePayload{
						{Name: "team", Source: "github:acme/plugins"},
					}}, nil
				},
				previewExtensionInstallFn: func(_ context.Context, request InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
					if !tc.curated || request.Source != contract.InstallExtensionSourceCurated {
						t.Fatal("unexpected curated preview")
					}
					return ExtensionInstallPreviewRecord{DigestSHA256: strings.Repeat("a", 64)}, nil
				},
				marketplaceInfoFn: func(_ context.Context, entry, source, _ string, _ MarketplaceReadScope) (MarketplaceEntryRecord, error) {
					if tc.curated || entry != "tool" || source != "team" {
						t.Fatal("unexpected plugin lookup")
					}
					return MarketplaceEntryRecord{
						Entry: contract.MarketplaceListingPayload{DigestSHA256: strings.Repeat("b", 64)},
					}, nil
				},
			}
			deps, _ := newExtensionLocalDeps(t, client)
			plan, err := parseExtensionInstallPlan(tc.ref, "", "", true)
			if err != nil {
				t.Fatal(err)
			}
			selected, err := resolvePluginInstallPlan(t.Context(), deps, plan)
			if tc.wantErr {
				if err == nil {
					t.Fatal("unknown source accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(selected.Attempts) != 1 || selected.Attempts[0].Source != tc.wantSource ||
				selected.Attempts[0].ExpectedDigest == "" {
				t.Fatalf("selection = %+v", selected)
			}
		})
	}
}

// Invariant: the experimental dependency flow consents per artifact: a TTY shows one consolidated preview and each
// dependency's own trust gates, a non-TTY needs --with-dependencies, bare --allow-unverified never consents for a
// dependency, and remove refuses while dependents need the extension unless --force is passed [T-19].
// Owner: CLI extension install/remove commands. Canonical suite: extension_marketplace_test.go.
func TestExtensionInstallDependencyFlow(t *testing.T) {
	t.Parallel()
	gateway := strings.Repeat("d", 64)
	core := contract.ExtensionDependencyPayload{
		Name: "acme-core", Source: "curated", Ref: "acme/core", RequiredBy: []string{"typed"}, Depth: 1,
		State: "missing", Version: "0.5.1", DigestSHA256: strings.Repeat("a", 64), ConsentRequired: true,
		GatewayRequirementDigest: gateway,
	}
	other := contract.ExtensionDependencyPayload{
		Name: "acme-other", Source: "curated", Ref: "acme/other", RequiredBy: []string{"typed"}, Depth: 1,
		State: "missing", Version: "1.0.0", DigestSHA256: strings.Repeat("b", 64), ConsentRequired: true,
	}
	required := func(items ...contract.ExtensionDependencyPayload) error {
		return &extensionOperationAPIError{statusCode: 422, status: "422 Unprocessable Entity",
			payload: contract.ExtensionOperationErrorPayload{
				Error: "extension: dependencies required", Code: diagnosticcontract.CodeExtensionDependenciesRequired,
				Dependencies: items,
			}}
	}
	newDeps := func(
		t *testing.T, terminal bool, calls *[]InstallExtensionRequest, first error,
	) commandDeps {
		t.Helper()
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			previewExtensionInstallFn: func(context.Context, InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
				return ExtensionInstallPreviewRecord{
					Name: "typed", DigestSHA256: strings.Repeat("c", 64),
					Dependencies: []contract.ExtensionDependencyPayload{core, other},
				}, nil
			},
			installExtensionFn: func(_ context.Context, req InstallExtensionRequest) (ExtensionRecord, error) {
				*calls = append(*calls, req)
				if len(*calls) == 1 && first != nil {
					return ExtensionRecord{}, first
				}
				return ExtensionRecord{Name: "typed", InstallDependenciesInstalled: []string{"acme-core"}}, nil
			},
		})
		markExtensionDaemonRunning(&deps)
		deps.inputIsTerminal = func(io.Reader) bool { return terminal }
		deps.outputIsTerminal = func(io.Writer) bool { return terminal }
		return deps
	}

	t.Run("Should show one consolidated preview and run each dependency trust gate on a TTY", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		deps := newDeps(t, true, &calls, required(core))
		stdout, stderr, err := executeRootCommandWithInput(
			t,
			deps,
			"y\ny\ny\n",
			"extension",
			"install",
			"compozy/typed",
		)
		if err != nil || len(calls) != 2 {
			t.Fatalf("install calls = %d, err = %v; want a refusal and one consented retry", len(calls), err)
		}
		want := contract.ExtensionDependencyConsent{
			ExpectedDigest: core.DigestSHA256, AllowUnverified: true, ConfirmGatewayDigest: gateway,
		}
		if retry := calls[1]; !retry.WithDependencies || retry.Dependencies["acme-core"] != want {
			t.Fatalf("retry = %+v, want with_dependencies and acme-core consent %+v", retry, want)
		}
		for _, text := range []string{"Dependencies (experimental):", "acme-core 0.5.1 from curated acme/core",
			"Install these dependencies first", "acme-core is not registry-verified", "gateway requirement " + gateway} {
			if !strings.Contains(stderr, text) {
				t.Fatalf("stderr = %q, want %q", stderr, text)
			}
		}
		if !strings.Contains(stdout, "Installed dependencies first: acme-core.") {
			t.Fatalf("stdout = %q, want the installed dependencies", stdout)
		}
	})

	t.Run("Should install nothing more when the consolidated consent is declined", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		deps := newDeps(t, true, &calls, required(core))
		_, _, err := executeRootCommandWithInput(t, deps, "n\n", "extension", "install", "compozy/typed")
		if !errors.Is(err, errExtensionDependenciesDeclined) || len(calls) != 1 {
			t.Fatalf("declined consent: calls = %d, err = %v", len(calls), err)
		}
	})

	t.Run("Should require --with-dependencies without a TTY", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		deps := newDeps(t, false, &calls, required(core))
		_, stderr, err := executeRootCommandWithInput(t, deps, "y\n", "extension", "install", "compozy/typed")
		if err == nil || len(calls) != 1 || strings.Contains(stderr, "Install these dependencies") {
			t.Fatalf("calls = %d, stderr = %q, err = %v; want one refusal without a prompt", len(calls), stderr, err)
		}
		if !strings.Contains(err.Error(), "--with-dependencies") || cliExitCodeForError(err) != 2 {
			t.Fatalf(
				"error = %q (exit %d), want --with-dependencies guidance and exit 2",
				err,
				cliExitCodeForError(err),
			)
		}
	})

	t.Run("Should print the update command for a version_unmet dependency", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		unmet := core
		unmet.State, unmet.RetryCommand = "version_unmet", "compozy extension update acme-core"
		refusal := required(unmet)
		refusal.(*extensionOperationAPIError).payload.RetryCommand = unmet.RetryCommand
		deps := newDeps(t, true, &calls, refusal)
		_, _, err := executeRootCommandWithInput(t, deps, "y\n", "extension", "install", "compozy/typed")
		if err == nil || len(calls) != 1 || cliExitCodeForError(err) != 2 ||
			!strings.Contains(err.Error(), "Update first: compozy extension update acme-core") {
			t.Fatalf("calls = %d, err = %v; want the update guidance and exit 2 without a prompt", len(calls), err)
		}
	})

	t.Run("Should not let bare --allow-unverified consent for a dependency", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		gate := &extensionOperationAPIError{statusCode: 422, status: "422 Unprocessable Entity",
			payload: contract.ExtensionOperationErrorPayload{
				Error: "checksum unverified", Code: diagnosticcontract.CodeExtensionChecksumUnverified,
				Dependency: "acme-core",
			}}
		deps := newDeps(t, false, &calls, gate)
		_, _, err := executeRootCommandWithInput(t, deps, "", "extension", "install", "compozy/typed",
			"--with-dependencies", "--allow-unverified", "--yes", "-o", "json")
		if len(calls) != 1 || !calls[0].AllowUnverified || calls[0].Dependencies["acme-core"].AllowUnverified {
			t.Fatalf("request = %+v, want root-only allow_unverified", calls)
		}
		if err == nil || !strings.Contains(err.Error(), "--allow-unverified-dependency acme-core") {
			t.Fatalf("error = %v, want the per-dependency flag naming acme-core", err)
		}
	})

	t.Run("Should consent only for the named dependency and confirm a dependency gateway digest", func(t *testing.T) {
		t.Parallel()
		var calls []InstallExtensionRequest
		root := strings.Repeat("e", 64)
		deps := newDeps(t, false, &calls, nil)
		_, _, err := executeRootCommandWithInput(t, deps, "", "extension", "install", "compozy/typed",
			"--with-dependencies", "--allow-unverified-dependency", "acme-core", "--yes",
			"--confirm-gateway-requirement", root, "--confirm-gateway-requirement", "acme-core="+gateway, "-o", "json")
		if err != nil || len(calls) != 1 {
			t.Fatalf("install calls = %d, err = %v", len(calls), err)
		}
		request := calls[0]
		wantCore := contract.ExtensionDependencyConsent{
			ExpectedDigest: core.DigestSHA256, AllowUnverified: true, ConfirmGatewayDigest: gateway,
		}
		wantOther := contract.ExtensionDependencyConsent{ExpectedDigest: other.DigestSHA256}
		if !request.WithDependencies || request.AllowUnverified || request.ConfirmGatewayDigest != root ||
			request.Dependencies["acme-core"] != wantCore || request.Dependencies["acme-other"] != wantOther {
			t.Fatalf("request = %+v, want consent for acme-core only", request)
		}
	})

	t.Run("Should call only official and community dependencies registry-verified", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			dependency contract.ExtensionDependencyPayload
			want       string
		}{
			{contract.ExtensionDependencyPayload{RegistryTier: "official"}, "registry-verified (official)"},
			{contract.ExtensionDependencyPayload{RegistryTier: "community"}, "registry-verified (community)"},
			{contract.ExtensionDependencyPayload{RegistryTier: "unverified", State: "disabled"}, "unverified"},
			{contract.ExtensionDependencyPayload{}, "unverified"},
			{contract.ExtensionDependencyPayload{RegistryTier: "official", ConsentRequired: true}, "unverified"},
		} {
			if got := extensionDependencyTrustWord(tc.dependency); got != tc.want {
				t.Fatalf("trust word for %+v = %q, want %q", tc.dependency, got, tc.want)
			}
		}
	})
}

// Invariant: remove prints the dependents and exits 1 on extension_dependency_in_use; --force carries force to the
// daemon request [T-19].
func TestExtensionRemoveDependencyGuard(t *testing.T) {
	t.Parallel()
	inUse := &extensionOperationAPIError{statusCode: 409, status: "409 Conflict",
		payload: contract.ExtensionOperationErrorPayload{
			Error: "extension: dependency in use", Code: diagnosticcontract.CodeExtensionDependencyInUse,
			Dependents: []contract.ExtensionDependentPayload{{Name: "acme-tool", Profile: "default"}},
		}}
	for _, testCase := range []struct {
		name  string
		force bool
	}{
		{name: "Should print the dependents and exit 1 without --force"},
		{name: "Should carry --force to the removal", force: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			forced := false
			deps, _ := newExtensionLocalDeps(t, &stubClient{
				removeExtensionFn: func(ctx context.Context, name string) (ManagedExtensionRemoveRecord, error) {
					forced = extensionpkg.DependencyForced(ctx)
					if forced {
						return ManagedExtensionRemoveRecord{Name: name, Status: "removed"}, nil
					}
					return ManagedExtensionRemoveRecord{}, inUse
				},
			})
			markExtensionDaemonRunning(&deps)
			args := []string{"extension", "remove", "acme-core", "--global"}
			if testCase.force {
				args = append(args, "--force")
			}
			_, _, err := executeRootCommand(t, deps, args...)
			if testCase.force {
				if err != nil || !forced {
					t.Fatalf("forced remove: forced = %t, err = %v", forced, err)
				}
				if query := extensionForceQuery(extensionpkg.WithDependencyForce(t.Context(), true), nil); query.Get(
					extensionForceFlagName) != toolBoolTrue {
					t.Fatalf("force query = %v, want force=true", query)
				}
				return
			}
			if err == nil || cliExitCodeForError(err) != 1 ||
				!strings.Contains(err.Error(), "acme-tool (workspace global, profile default)") ||
				!strings.Contains(err.Error(), "--force") {
				t.Fatalf("error = %v (exit %d), want the dependents, --force guidance and exit 1",
					err, cliExitCodeForError(err))
			}
		})
	}
}
