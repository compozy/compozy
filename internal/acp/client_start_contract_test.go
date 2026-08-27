package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/testutil"
)

func TestStartInitializeContract(t *testing.T) {
	t.Parallel()

	t.Run("Should send the ACP protocol version as a JSON number", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "initialize.jsonl")
		proc := startHelperProcess(t, driver, "initialize_contract", "", StartOpts{
			Env: helperEnvWithCapture("initialize_contract", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		params := captureRequestParams(t, captureFile, acpsdk.AgentMethodInitialize)
		protocolVersionJSON, ok := params["protocolVersion"]
		if !ok {
			t.Fatal("initialize protocolVersion missing")
		}

		var protocolVersion int
		if err := json.Unmarshal(protocolVersionJSON, &protocolVersion); err != nil {
			t.Fatalf("initialize protocolVersion = %s, want JSON number: %v", protocolVersionJSON, err)
		}
		if protocolVersion != acpsdk.ProtocolVersionNumber {
			t.Fatalf(
				"initialize protocolVersion = %d, want %d",
				protocolVersion,
				acpsdk.ProtocolVersionNumber,
			)
		}
	})
}

func TestStartCapturesPromptCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scenario string
		want     Caps
	}{
		{
			name:     "Should preserve the image prompt capability after session negotiation",
			scenario: "prompt_capabilities_image",
			want:     Caps{PromptImage: true},
		},
		{
			name:     "Should preserve the audio prompt capability after session negotiation",
			scenario: "prompt_capabilities_audio",
			want:     Caps{PromptAudio: true},
		},
		{
			name:     "Should preserve the embedded-context prompt capability after session negotiation",
			scenario: "prompt_capabilities_embedded_context",
			want:     Caps{PromptEmbeddedContext: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			proc := startHelperProcess(t, driver, tt.scenario, "", StartOpts{})
			t.Cleanup(func() {
				stopProcess(t, driver, proc)
			})

			caps := proc.CapsSnapshot()
			if got, want := caps.PromptImage, tt.want.PromptImage; got != want {
				t.Fatalf("Start() PromptImage = %t, want %t", got, want)
			}
			if got, want := caps.PromptAudio, tt.want.PromptAudio; got != want {
				t.Fatalf("Start() PromptAudio = %t, want %t", got, want)
			}
			if got, want := caps.PromptEmbeddedContext, tt.want.PromptEmbeddedContext; got != want {
				t.Fatalf("Start() PromptEmbeddedContext = %t, want %t", got, want)
			}
		})
	}
}

func TestStartActivatesMCPAfterInitializeBeforeSessionNegotiation(t *testing.T) {
	t.Parallel()

	t.Run("Should activate MCP after initialize and before session negotiation", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "mcp-activation-order.jsonl")
		activated := false
		proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{
			Env: helperEnvWithCapture("stream_updates", "", captureFile),
			MCPServers: []compozyconfig.MCPServer{{
				Name:      "compozy-hosted-tools",
				Transport: compozyconfig.MCPServerTransportStdio,
				Command:   "/bin/compozy",
			}},
			ActivateMCPServers: func(ctx context.Context) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if !captureMethodExists(t, captureFile, acpsdk.AgentMethodInitialize) {
					t.Fatal("MCP activation ran before ACP initialize completed")
				}
				if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionNew) {
					t.Fatal("MCP activation ran after ACP session/new")
				}
				activated = true
				return nil
			},
		})
		defer stopProcess(t, driver, proc)

		if !activated {
			t.Fatal("ActivateMCPServers was not called")
		}
		if !captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionNew) {
			t.Fatal("ACP session/new was not sent after MCP activation")
		}
	})

	t.Run("Should stop before session negotiation when MCP activation fails", func(t *testing.T) {
		t.Parallel()

		activationErr := errors.New("MCP activation failed")
		driver := New()
		captureFile := filepath.Join(t.TempDir(), "mcp-activation-failure.jsonl")
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:   "helper",
			Command:     helperCommand(t),
			Cwd:         t.TempDir(),
			Env:         helperEnvWithCapture("stream_updates", "", captureFile),
			Permissions: compozyconfig.PermissionModeApproveAll,
			MCPServers: []compozyconfig.MCPServer{{
				Name:      "compozy-hosted-tools",
				Transport: compozyconfig.MCPServerTransportStdio,
				Command:   "/bin/compozy",
			}},
			ActivateMCPServers: func(context.Context) error {
				return activationErr
			},
		})
		if proc != nil {
			defer stopProcess(t, driver, proc)
			t.Fatalf("Start() process = %#v, want nil after activation failure", proc)
		}
		if !errors.Is(err, activationErr) {
			t.Fatalf("Start() error = %v, want wrapped activation error", err)
		}
		if !captureMethodExists(t, captureFile, acpsdk.AgentMethodInitialize) {
			t.Fatal("ACP initialize was not sent before MCP activation")
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionNew) {
			t.Fatal("ACP session/new was sent after MCP activation failed")
		}
	})
}

func TestDaemonMatchedEnvPinsCurrentBinary(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(
		executable,
	); resolveErr == nil &&
		strings.TrimSpace(resolved) != "" {
		executable = resolved
	}
	binDir := filepath.Dir(executable)

	env := daemonMatchedEnv([]string{
		"PATH=/should-be-ignored",
		"FOO=bar",
		"COMPOZY_BIN=/should-be-replaced",
		"PATH=/usr/local/bin" + string(os.PathListSeparator) + binDir + string(os.PathListSeparator) + "/usr/bin",
		"COMPOZY_BIN=/should-also-be-replaced",
	})

	gotCompozyBin, ok := envValue(env, "COMPOZY_BIN")
	if !ok || gotCompozyBin != executable {
		t.Fatalf("daemonMatchedEnv() COMPOZY_BIN = %q, %v, want %q", gotCompozyBin, ok, executable)
	}

	gotPath, ok := envValue(env, "PATH")
	if !ok {
		t.Fatal("daemonMatchedEnv() PATH missing")
	}
	wantPath := binDir + string(os.PathListSeparator) + "/usr/local/bin" + string(os.PathListSeparator) + "/usr/bin"
	if gotPath != wantPath {
		t.Fatalf("daemonMatchedEnv() PATH = %q, want %q", gotPath, wantPath)
	}

	pathCount := 0
	compozyBinCount := 0
	for _, variable := range env {
		switch {
		case strings.HasPrefix(variable, "PATH="):
			pathCount++
		case strings.HasPrefix(variable, "COMPOZY_BIN="):
			compozyBinCount++
		}
	}
	if pathCount != 1 || compozyBinCount != 1 {
		t.Fatalf(
			"daemonMatchedEnv() duplicate entries remain: PATH=%d COMPOZY_BIN=%d env=%#v",
			pathCount,
			compozyBinCount,
			env,
		)
	}
}

func TestStartResumeUsesLoadSession(t *testing.T) {
	t.Parallel()

	t.Run("Should resume through loadSession when a prior session ID is provided", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "load_session", "", StartOpts{
			ResumeSessionID: "sess-existing",
		})
		defer stopProcess(t, driver, proc)

		if proc.SessionID != "sess-existing" {
			t.Fatalf("Start() session id = %q, want %q", proc.SessionID, "sess-existing")
		}
		caps := proc.CapsSnapshot()
		if !caps.SupportsLoadSession {
			t.Fatal("Start() SupportsLoadSession = false, want true")
		}
		if !slices.Equal(caps.SupportedModes, []string{"loaded-mode"}) {
			t.Fatalf("Start() supported modes = %#v, want %#v", caps.SupportedModes, []string{"loaded-mode"})
		}
	})
}

func TestStartApproveAllSetsPermissiveSessionModeWhenSupported(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-new.jsonl")
	proc := startHelperProcess(t, driver, "mode_mapping", "", StartOpts{
		Permissions: compozyconfig.PermissionModeApproveAll,
		Env:         helperEnvWithCapture("mode_mapping", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-new"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "bypassPermissions"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartWithToolGatewayPreservesPermissiveSessionMode(t *testing.T) {
	t.Parallel()

	t.Run("Should keep approve-all semantics when tool execution is intercepted", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-mode-gateway.jsonl")
		proc := startHelperProcess(t, driver, "mode_mapping", "", StartOpts{
			Permissions: compozyconfig.PermissionModeApproveAll,
			Env:         helperEnvWithCapture("mode_mapping", "", captureFile),
			ToolGateway: toolExecutionGatewayFunc(
				func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
					return req, nil
				},
			),
		})
		defer stopProcess(t, driver, proc)

		params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
		request := decodeCapturedSetSessionModeRequest(t, params)
		if got, want := request.SessionID, "sess-new"; got != want {
			t.Fatalf("set-mode session id = %q, want %q", got, want)
		}
		if got, want := request.ModeID, "bypassPermissions"; got != want {
			t.Fatalf("set-mode mode id = %q, want %q", got, want)
		}
	})

	t.Run("Should select Cursor agent mode and expose it as current", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-mode-cursor.jsonl")
		proc := startHelperProcess(t, driver, "cursor_mode_mapping", "", StartOpts{
			Permissions: compozyconfig.PermissionModeApproveAll,
			Env:         helperEnvWithCapture("cursor_mode_mapping", "", captureFile),
			ToolGateway: toolExecutionGatewayFunc(
				func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
					return req, nil
				},
			),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionModeRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode),
		)
		if got, want := request.ModeID, "agent"; got != want {
			t.Fatalf("set-mode mode id = %q, want %q", got, want)
		}
		assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "mode", "agent", "agent", "plan", "ask")
	})

	t.Run("Should avoid changing Cursor when its native mode is already agent", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-current-agent-cursor.jsonl")
		proc := startHelperProcess(t, driver, "cursor_mode_current_agent", "", StartOpts{
			Permissions: compozyconfig.PermissionModeApproveAll,
			Env:         helperEnvWithCapture("cursor_mode_current_agent", "", captureFile),
			ToolGateway: toolExecutionGatewayFunc(
				func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
					return req, nil
				},
			),
		})
		defer stopProcess(t, driver, proc)

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetMode) {
			t.Fatal("set_mode was sent when Cursor already reported agent mode")
		}
		assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "mode", "agent", "agent", "plan", "ask")
	})
}

func TestStartResumeApproveReadsSetsReadOnlyLikeSessionModeWhenSupported(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-load.jsonl")
	proc := startHelperProcess(t, driver, "load_mode_mapping", "", StartOpts{
		ResumeSessionID: "sess-existing",
		Permissions:     compozyconfig.PermissionModeApproveReads,
		Env:             helperEnvWithCapture("load_mode_mapping", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-existing"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "plan"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartResumeWithToolGatewayPrefersApprovalMediatedMode(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-load-gateway.jsonl")
	proc := startHelperProcess(t, driver, "load_mode_mapping", "", StartOpts{
		ResumeSessionID: "sess-existing",
		Permissions:     compozyconfig.PermissionModeApproveReads,
		Env:             helperEnvWithCapture("load_mode_mapping", "", captureFile),
		ToolGateway: toolExecutionGatewayFunc(
			func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
				return req, nil
			},
		),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-existing"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "default"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartDenyAllWithToolGatewayPrefersApprovalMediatedSessionMode(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-mode-deny-gateway.jsonl")
	proc := startHelperProcess(t, driver, "mode_mapping", "", StartOpts{
		Permissions: compozyconfig.PermissionModeDenyAll,
		Env:         helperEnvWithCapture("mode_mapping", "", captureFile),
		ToolGateway: toolExecutionGatewayFunc(
			func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
				return req, nil
			},
		),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetMode)
	request := decodeCapturedSetSessionModeRequest(t, params)
	if got, want := request.SessionID, "sess-new"; got != want {
		t.Fatalf("set-mode session id = %q, want %q", got, want)
	}
	if got, want := request.ModeID, "default"; got != want {
		t.Fatalf("set-mode mode id = %q, want %q", got, want)
	}
}

func TestStartCapturesSessionConfigOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		scenario      string
		resumeSession string
		wantModel     string
		wantReasoning string
	}{
		{
			name:          "Should capture config options from session new",
			scenario:      "config_options",
			wantModel:     "new-model",
			wantReasoning: "medium",
		},
		{
			name:          "Should capture config options from session load",
			scenario:      "load_config_options",
			resumeSession: "sess-existing",
			wantModel:     "loaded-model",
			wantReasoning: "high",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			proc := startHelperProcess(t, driver, tc.scenario, "", StartOpts{
				ResumeSessionID: tc.resumeSession,
			})
			defer stopProcess(t, driver, proc)

			caps := proc.CapsSnapshot()
			assertConfigOption(t, caps.ConfigOptions, "model", tc.wantModel, "new-model", "loaded-model", "other-model")
			assertConfigOption(t, caps.ConfigOptions, "reasoning_effort", tc.wantReasoning, "minimal", "high", "xhigh")
		})
	}
}

func TestInspectSessionConfigOptionsDoesNotMutateTheACPNewSession(t *testing.T) {
	t.Parallel()

	t.Run("Should inspect options without mutating the new ACP session", func(t *testing.T) {
		t.Parallel()

		captureFile := filepath.Join(t.TempDir(), "session-inspection.jsonl")
		options, err := InspectSessionConfigOptions(testutil.Context(t), SessionInspectionRequest{
			AgentName: "helper",
			Command:   helperCommand(t),
			Cwd:       t.TempDir(),
			Env:       helperEnvWithCapture("config_options", "", captureFile),
		})
		if err != nil {
			t.Fatalf("InspectSessionConfigOptions() error = %v", err)
		}
		assertConfigOption(t, options, "model", "new-model", "new-model", "loaded-model", "other-model")
		if !captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionNew) {
			t.Fatal("session/new was not sent during ACP inspection")
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetMode) {
			t.Fatal("session/set_mode was sent during ACP inspection")
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("session/set_config_option was sent during ACP inspection")
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionCancel) {
			t.Fatal("session/cancel was sent during ACP inspection")
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionClose) {
			t.Fatal("session/close was sent without an advertised close capability")
		}
	})
}

func TestStopCancelsANonInspectionSession(t *testing.T) {
	t.Parallel()

	t.Run("Should cancel a non-inspection session", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-stop-cancel.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			Env: helperEnvWithCapture("config_options", "", captureFile),
		})
		if err := driver.Stop(testutil.Context(t), proc); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if !captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionCancel) {
			t.Fatal("session/cancel was not sent for a non-inspection session")
		}
	})
}

func TestStartUsesSetConfigOptionForPreferredModelWhenAvailable(t *testing.T) {
	t.Parallel()

	driver := New()
	captureFile := filepath.Join(t.TempDir(), "session-set-config-model.jsonl")
	proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
		PreferredModel: "other-model",
		Env:            helperEnvWithCapture("config_options", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	request := decodeCapturedSetSessionConfigOptionRequest(
		t,
		captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
	)
	if got := request.SessionID; got != "sess-new" {
		t.Fatalf("set-config session id = %q, want sess-new", got)
	}
	if got := request.ConfigID; got != "model" {
		t.Fatalf("set-config config id = %q, want model", got)
	}
	if got := request.Value; got != "other-model" {
		t.Fatalf("set-config value = %q, want other-model", got)
	}
	assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "model", "other-model", "other-model")
}

func TestStartRejectsModelOutsideAdvertisedConfigOptionsBeforeSetConfigOption(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an unadvertised model before applying config", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-config-model-rejected.jsonl")
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:      "helper",
			Command:        helperCommand(t),
			Cwd:            t.TempDir(),
			PreferredModel: "cursor-grok-4.5-high",
			Env:            helperEnvWithCapture("config_options", "", captureFile),
		})
		if proc != nil {
			defer stopProcess(t, driver, proc)
			t.Fatalf("Start() process = %#v, want nil after membership rejection", proc)
		}
		negotiationErr, ok := errors.AsType[*NegotiationError](err)
		if !ok || negotiationErr.Code != NegotiationCodeModelUnavailable {
			t.Fatalf("Start() error = %v, want model_unavailable NegotiationError", err)
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("session/set_config_option was sent for a model outside the advertised values")
		}
	})
}

func TestStartHandlesCurrentSessionConfigValues(t *testing.T) {
	t.Parallel()

	t.Run("Should explicitly apply a preferred model that is already current", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-current-model.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			PreferredModel: "new-model",
			Env:            helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionConfigOptionRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
		)
		if request.ConfigID != "model" || request.Value != "new-model" {
			t.Fatalf("set-config request = %#v, want explicit current model", request)
		}
	})

	t.Run("Should skip an explicit reasoning effort that is already current", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-current-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ReasoningEffort: "medium",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent when the requested reasoning effort was already current")
		}
	})
}

func TestStartUsesSetConfigOptionForReasoningEffortWhenAvailable(t *testing.T) {
	t.Parallel()

	t.Run("Should apply reasoning effort through session/set_config_option", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-config-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ReasoningEffort: "high",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionConfigOptionRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
		)
		if got := request.ConfigID; got != "reasoning_effort" {
			t.Fatalf("set-config config id = %q, want reasoning_effort", got)
		}
		if got := request.Value; got != "high" {
			t.Fatalf("set-config value = %q, want high", got)
		}
		assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "reasoning_effort", "high", "high")
	})
}

func TestStartNegotiatesRequestedSpeed(t *testing.T) {
	t.Parallel()

	t.Run("Should apply fast speed through session config", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-set-config-speed.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			Speed: speedpkg.SpeedFast,
			Env:   helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionConfigOptionRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
		)
		if request.ConfigID != "speed" || request.Value != "fast" {
			t.Fatalf("set-config request = %#v, want speed=fast", request)
		}
		resolution := proc.CapsSnapshot().SpeedResolution
		if resolution == nil ||
			resolution.Requested != speedpkg.SpeedFast ||
			resolution.Status != speedpkg.ResolutionApplied {
			t.Fatalf("speed resolution = %#v, want applied fast", resolution)
		}
	})

	t.Run("Should continue with an unsupported outcome when speed is absent", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-speed-unsupported.jsonl")
		proc := startHelperProcess(t, driver, "config_options_no_model", "", StartOpts{
			Speed: speedpkg.SpeedFast,
			Env:   helperEnvWithCapture("config_options_no_model", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent without an advertised speed option")
		}
		resolution := proc.CapsSnapshot().SpeedResolution
		if resolution == nil ||
			resolution.Status != speedpkg.ResolutionUnsupported ||
			resolution.Reason != speedpkg.ReasonCapabilityAbsent {
			t.Fatalf("speed resolution = %#v, want unsupported capability_absent", resolution)
		}
	})

	t.Run("Should fail atomically with a typed diagnostic when the provider rejects speed", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:   "helper",
			Command:     helperCommand(t),
			Cwd:         t.TempDir(),
			Env:         helperEnv("config_options_reject_speed", ""),
			Permissions: compozyconfig.PermissionModeApproveAll,
			Speed:       speedpkg.SpeedFast,
		})
		if proc != nil {
			t.Fatal("Start() process != nil, want failed setup cleanup")
		}
		negotiationErr, negotiationErrMatched := errors.AsType[*NegotiationError](err)
		if !negotiationErrMatched ||
			negotiationErr.Code != NegotiationCodeSpeedRejected ||
			negotiationErr.Stage != "speed" ||
			negotiationErr.Requested != "fast" {
			t.Fatalf("Start() error = %v, want speed_rejected NegotiationError", err)
		}
	})
}

func TestMatchSpeedConfigRejectsAmbiguity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject multiple speed capabilities", func(t *testing.T) {
		t.Parallel()

		option := sessionConfigOptionFromSDKForTest(t, helperSpeedConfigOption("normal"))
		match, reason := matchSpeedConfig(speedpkg.SpeedFast, []SessionConfigOption{option, option})
		if match != nil || reason != speedpkg.ReasonCapabilityAmbiguous {
			t.Fatalf("matchSpeedConfig() = %#v, %q, want capability ambiguity", match, reason)
		}
	})

	t.Run("Should reject speed values without one normal and one fast choice", func(t *testing.T) {
		t.Parallel()

		option := sessionConfigOptionFromSDKForTest(t, helperSpeedConfigOption("normal"))
		option.Values = option.Values[:1]
		match, reason := matchSpeedConfig(speedpkg.SpeedFast, []SessionConfigOption{option})
		if match != nil || reason != speedpkg.ReasonValueAmbiguous {
			t.Fatalf("matchSpeedConfig() = %#v, %q, want value ambiguity", match, reason)
		}
	})
}

func TestStartRejectsReasoningWithoutAnApplyStrategyBeforeLaunch(t *testing.T) {
	t.Parallel()

	t.Run("Should return a typed error without launching the provider", func(t *testing.T) {
		t.Parallel()

		launcher := &recordingLauncher{err: errors.New("provider must not launch")}
		driver := New(WithLauncher(launcher))
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:       "helper",
			Command:         "helper",
			Cwd:             t.TempDir(),
			Permissions:     compozyconfig.PermissionModeApproveAll,
			ReasoningEffort: "high",
			ProviderConfig: &compozyconfig.ProviderConfig{Models: compozyconfig.ProviderModelsConfig{
				Reasoning: compozyconfig.ProviderReasoningConfig{Apply: compozyconfig.ReasoningApplyNone},
			}},
		})
		if proc != nil {
			t.Fatal("Start() process != nil, want no launched process")
		}
		negotiationErr, negotiationErrMatched := errors.AsType[*NegotiationError](err)
		if !negotiationErrMatched || negotiationErr.Code != NegotiationCodeReasoningOptionMissing {
			t.Fatalf("Start() error = %v, want reasoning_option_missing NegotiationError", err)
		}
		if _, called := launcher.lastSpec(); called {
			t.Fatal("provider launcher was called before reasoning strategy validation")
		}
	})
}

func TestStartPassesThroughEveryAdvertisedCanonicalReasoningEffort(t *testing.T) {
	t.Parallel()

	for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run("Should pass through "+effort, func(t *testing.T) {
			t.Parallel()

			driver := New()
			captureFile := filepath.Join(t.TempDir(), "session-reasoning-"+effort+".jsonl")
			proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
				ReasoningEffort: effort,
				ProviderConfig:  reasoningACPProviderConfig(),
				Env:             helperEnvWithCapture("config_options", "", captureFile),
			})
			defer stopProcess(t, driver, proc)

			if effort == "medium" {
				if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
					t.Fatal("set_config_option was sent for the already-current reasoning effort")
				}
				assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "reasoning_effort", effort, effort)
				return
			}

			request := decodeCapturedSetSessionConfigOptionRequest(
				t,
				captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
			)
			if got := request.Value; got != effort {
				t.Fatalf("set-config value = %q, want %q", got, effort)
			}
		})
	}
}

func TestStartDistinguishesProviderDefaultFromExplicitNone(t *testing.T) {
	t.Parallel()

	t.Run("Should skip effort RPC for empty provider default", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-default-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ProviderConfig: reasoningACPProviderConfig(),
			Env:            helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent for empty provider-default effort")
		}
	})

	t.Run("Should send explicit none when advertised", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-none-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "config_options", "", StartOpts{
			ReasoningEffort: "none",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		request := decodeCapturedSetSessionConfigOptionRequest(
			t,
			captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption),
		)
		if got, want := request.Value, "none"; got != want {
			t.Fatalf("set-config value = %q, want %q", got, want)
		}
	})
}

func TestStartAppliesModelBeforeModelSpecificReasoning(t *testing.T) {
	t.Parallel()

	t.Run("Should apply mode then model then refreshed model-specific effort", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-model-specific-reasoning.jsonl")
		proc := startHelperProcess(t, driver, "model_specific_config_options", "", StartOpts{
			Permissions:     compozyconfig.PermissionModeApproveAll,
			PreferredModel:  "other-model",
			ReasoningEffort: "max",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("model_specific_config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)
		got := captureNegotiationSequence(t, captureFile)
		want := []string{"mode:bypassPermissions", "model:other-model", "effort:max"}
		if !slices.Equal(got, want) {
			t.Fatalf("negotiation sequence = %#v, want %#v", got, want)
		}

		requests := captureRequestParamsForMethod(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption)
		if got, want := len(requests), 2; got != want {
			t.Fatalf("set_config_option request count = %d, want %d", got, want)
		}
		modelRequest := decodeCapturedSetSessionConfigOptionRequest(t, requests[0])
		reasoningRequest := decodeCapturedSetSessionConfigOptionRequest(t, requests[1])
		if modelRequest.ConfigID != "model" || modelRequest.Value != "other-model" {
			t.Fatalf("first set-config request = %#v, want model=other-model", modelRequest)
		}
		if reasoningRequest.ConfigID != "effort" || reasoningRequest.Value != "max" {
			t.Fatalf("second set-config request = %#v, want effort=max", reasoningRequest)
		}
	})
}

func TestConfigureRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should apply model then refreshed reasoning effort then speed", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "configure-runtime.jsonl")
		proc := startHelperProcess(t, driver, "runtime_config_options", "", StartOpts{
			Env: helperEnvWithCapture("runtime_config_options", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		err := driver.ConfigureRuntime(testutil.Context(t), proc, RuntimeConfig{
			Model:           "other-model",
			ReasoningEffort: "max",
			Speed:           speedpkg.SpeedFast,
			ACPOptions: []SessionConfigOptionSelection{
				{ID: "thinking", BoolValue: boolPointer(true)},
				{ID: "context", ValueID: "large"},
			},
		})
		if err != nil {
			t.Fatalf("ConfigureRuntime() error = %v", err)
		}

		got := captureNegotiationSequence(t, captureFile)
		want := []string{"model:other-model", "effort:max", "speed:fast", "context:large", "thinking:true"}
		if !slices.Equal(got, want) {
			t.Fatalf("runtime configuration sequence = %#v, want %#v", got, want)
		}

		caps := proc.CapsSnapshot()
		assertConfigOption(t, caps.ConfigOptions, "model", "other-model", "other-model")
		assertConfigOption(t, caps.ConfigOptions, "effort", "max", "none", "max")
		assertConfigOption(t, caps.ConfigOptions, "speed", "fast", "normal", "fast")
		assertConfigOption(t, caps.ConfigOptions, "context", "large", "standard", "large")
		thinking, ok := findConfigOptionByID(caps.ConfigOptions, "thinking")
		if !ok || thinking.CurrentBool == nil || !*thinking.CurrentBool {
			t.Fatalf("thinking option = %#v, want true", thinking)
		}
		requests := captureRequestParamsForMethod(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption)
		thinkingRequest := decodeCapturedSetSessionConfigOptionRequest(t, requests[len(requests)-1])
		if thinkingRequest.Type != "boolean" || thinkingRequest.BoolValue == nil || !*thinkingRequest.BoolValue {
			t.Fatalf("thinking request = %#v, want ACP boolean true", thinkingRequest)
		}
		if caps.SpeedResolution == nil ||
			caps.SpeedResolution.Requested != speedpkg.SpeedFast ||
			caps.SpeedResolution.Status != speedpkg.ResolutionApplied {
			t.Fatalf("speed resolution = %#v, want applied fast", caps.SpeedResolution)
		}
	})

	t.Run("Should return a typed failure when the provider rejects speed", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "configure-runtime-speed-rejected.jsonl")
		proc := startHelperProcess(t, driver, "config_options_reject_speed", "", StartOpts{
			Env: helperEnvWithCapture("config_options_reject_speed", "", captureFile),
		})
		defer stopProcess(t, driver, proc)

		err := driver.ConfigureRuntime(testutil.Context(t), proc, RuntimeConfig{Speed: speedpkg.SpeedFast})
		negotiationErr, negotiationErrMatched := errors.AsType[*NegotiationError](err)
		if !negotiationErrMatched ||
			negotiationErr.Code != NegotiationCodeSpeedRejected ||
			negotiationErr.Stage != "speed" ||
			negotiationErr.Requested != string(speedpkg.SpeedFast) {
			t.Fatalf("ConfigureRuntime() error = %v, want speed_rejected NegotiationError", err)
		}
		if !captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("ConfigureRuntime() did not send the advertised speed config option")
		}
		resolution := proc.CapsSnapshot().SpeedResolution
		if resolution == nil || resolution.Status != speedpkg.ResolutionRejected ||
			resolution.Reason != speedpkg.ReasonProviderRejected {
			t.Fatalf("speed resolution = %#v, want rejected by provider", resolution)
		}
	})
}

func TestStartRejectsReasoningEffortWhenConfigOptionIsAbsent(t *testing.T) {
	t.Parallel()

	t.Run("Should fail before prompting when the effort option is absent", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-no-reasoning-config.jsonl")
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:       "helper",
			Command:         helperCommand(t),
			Cwd:             t.TempDir(),
			Permissions:     compozyconfig.PermissionModeApproveAll,
			ReasoningEffort: "xhigh",
			ProviderConfig:  reasoningACPProviderConfig(),
			Env:             helperEnvWithCapture("config_options_no_reasoning", "", captureFile),
		})
		if proc != nil {
			defer stopProcess(t, driver, proc)
		}
		if err == nil {
			t.Fatal("Start() error = nil, want reasoning_option_missing")
		}
		negotiationErr, negotiationErrMatched := errors.AsType[*NegotiationError](err)
		if !negotiationErrMatched || negotiationErr.Code != NegotiationCodeReasoningOptionMissing {
			t.Fatalf("Start() error = %v, want reasoning_option_missing NegotiationError", err)
		}

		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent without a reasoning config option")
		}
	})
}

func TestStartRejectsPreferredModelWhenModelConfigOptionIsAbsent(t *testing.T) {
	t.Parallel()

	t.Run("Should reject preferred model without ACP model config option", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-no-model-config.jsonl")
		proc, err := driver.Start(testutil.Context(t), StartOpts{
			AgentName:      "helper",
			Command:        helperCommand(t),
			Cwd:            t.TempDir(),
			Env:            helperEnvWithCapture("config_options_no_model", "", captureFile),
			Permissions:    compozyconfig.PermissionModeApproveAll,
			PreferredModel: "new-model",
		})
		if proc != nil {
			defer stopProcess(t, driver, proc)
		}
		if err == nil {
			t.Fatal("Start() error = nil, want missing model config option error")
		}
		if !errors.Is(err, errModelConfigOptionRequired) {
			t.Fatalf("Start() error = %v, want model config option required", err)
		}
		if captureMethodExists(t, captureFile, acpsdk.AgentMethodSessionSetConfigOption) {
			t.Fatal("set_config_option was sent when no model config option was available")
		}
	})
}

func TestStartRejectsUnavailableSessionConfigOptionValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		opts            StartOpts
		wantCode        string
		forbiddenMethod string
	}{
		{
			name: "Should reject preferred model absent from model config option values",
			opts: StartOpts{
				PreferredModel: "missing-model",
			},
			wantCode:        NegotiationCodeModelUnavailable,
			forbiddenMethod: acpsdk.AgentMethodSessionSetConfigOption,
		},
		{
			name: "Should reject reasoning effort absent from reasoning config option values",
			opts: StartOpts{
				ReasoningEffort: "turbo",
			},
			wantCode:        NegotiationCodeReasoningEffortUnsupported,
			forbiddenMethod: acpsdk.AgentMethodSessionSetConfigOption,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			captureFile := filepath.Join(t.TempDir(), "session-unavailable-config-option.jsonl")
			opts := StartOpts{
				AgentName:   "helper",
				Command:     helperCommand(t),
				Cwd:         t.TempDir(),
				Env:         helperEnvWithCapture("config_options", "", captureFile),
				Permissions: compozyconfig.PermissionModeApproveAll,
			}
			opts.PreferredModel = tc.opts.PreferredModel
			opts.ReasoningEffort = tc.opts.ReasoningEffort
			if opts.ReasoningEffort != "" {
				opts.ProviderConfig = reasoningACPProviderConfig()
			}
			proc, err := driver.Start(testutil.Context(t), opts)
			if proc != nil {
				defer stopProcess(t, driver, proc)
			}
			if err == nil {
				t.Fatal("Start() error = nil, want unavailable config option error")
			}
			negotiationErr, negotiationErrMatched := errors.AsType[*NegotiationError](err)
			if !negotiationErrMatched || negotiationErr.Code != tc.wantCode {
				t.Fatalf("Start() error = %v, want NegotiationError code %q", err, tc.wantCode)
			}
			if captureMethodExists(t, captureFile, tc.forbiddenMethod) {
				t.Fatalf("forbidden method %q was sent after unavailable config value", tc.forbiddenMethod)
			}
		})
	}
}

func TestSessionConfigOptionUpdateMutatesCaps(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := startHelperProcess(t, driver, "config_option_update", "", StartOpts{})
	defer stopProcess(t, driver, proc)

	events, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-config-options",
		Message: "update config",
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	collectEvents(t, events)

	caps := proc.CapsSnapshot()
	assertConfigOption(t, caps.ConfigOptions, "model", "other-model", "other-model")
	assertConfigOption(t, caps.ConfigOptions, "reasoning_effort", "xhigh", "xhigh")
}

func TestStartWithEmptyAdditionalDirsKeepsBaselinePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts StartOpts
	}{
		{
			name: "nil additional dirs",
			opts: StartOpts{},
		},
		{
			name: "explicit empty additional dirs",
			opts: StartOpts{AdditionalDirs: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			captureFile := filepath.Join(t.TempDir(), strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			opts := tt.opts
			opts.Env = helperEnvWithCapture("stream_updates", "", captureFile)

			proc := startHelperProcess(t, driver, "stream_updates", "", opts)
			defer stopProcess(t, driver, proc)

			params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionNew)
			if _, exists := params["additional_dirs"]; exists {
				t.Fatalf("session/new params include additional_dirs for %s: %#v", tt.name, params)
			}
		})
	}
}

func TestStartIncludesAdditionalDirsInNewSessionPayload(t *testing.T) {
	t.Parallel()

	driver := New()
	root := t.TempDir()
	additionalOne := t.TempDir()
	additionalTwo := t.TempDir()
	captureFile := filepath.Join(t.TempDir(), "session-new.jsonl")

	proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{
		Cwd:            root,
		AdditionalDirs: []string{additionalOne, additionalTwo},
		Env:            helperEnvWithCapture("stream_updates", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionNew)
	request := decodeCapturedNewSessionRequest(t, params)
	if got, want := request.Cwd, mustCanonicalDir(t, root); got != want {
		t.Fatalf("session/new cwd = %q, want %q", got, want)
	}
	if got, want := request.AdditionalDirs, []string{
		mustCanonicalDir(t, additionalOne),
		mustCanonicalDir(t, additionalTwo),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("session/new additional_dirs = %#v, want %#v", got, want)
	}
}

func TestStartIncludesAdditionalDirsInLoadSessionPayload(t *testing.T) {
	t.Parallel()

	driver := New()
	root := t.TempDir()
	additionalOne := t.TempDir()
	additionalTwo := t.TempDir()
	captureFile := filepath.Join(t.TempDir(), "session-load.jsonl")

	proc := startHelperProcess(t, driver, "load_session", "", StartOpts{
		Cwd:             root,
		AdditionalDirs:  []string{additionalOne, additionalTwo},
		ResumeSessionID: "sess-existing",
		Env:             helperEnvWithCapture("load_session", "", captureFile),
	})
	defer stopProcess(t, driver, proc)

	params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionLoad)
	request := decodeCapturedLoadSessionRequest(t, params)
	if got, want := request.Cwd, mustCanonicalDir(t, root); got != want {
		t.Fatalf("session/load cwd = %q, want %q", got, want)
	}
	if request.SessionID != "sess-existing" {
		t.Fatalf("session/load sessionId = %q, want %q", request.SessionID, "sess-existing")
	}
	if got, want := request.AdditionalDirs, []string{
		mustCanonicalDir(t, additionalOne),
		mustCanonicalDir(t, additionalTwo),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("session/load additional_dirs = %#v, want %#v", got, want)
	}
}

func TestStartMCPServersSkipsRemoteTransports(t *testing.T) {
	t.Parallel()

	t.Run("Should skip remote transports when starting MCP servers", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "session-new-mcp.jsonl")
		proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{
			Cwd: t.TempDir(),
			Env: helperEnvWithCapture("stream_updates", "", captureFile),
			MCPServers: []compozyconfig.MCPServer{
				{
					Name:      "compozy-hosted-tools",
					Transport: compozyconfig.MCPServerTransportStdio,
					Command:   "/bin/compozy",
					Args:      []string{"tool", "mcp", "--session", "sess-1", "--bind-nonce", "nonce"},
					Env:       map[string]string{"COMPOZY_HOME": "/tmp/compozy-home"},
				},
				{
					Name:      "remote-http",
					Transport: compozyconfig.MCPServerTransportHTTP,
					URL:       "https://example.test/mcp",
				},
			},
		})
		defer stopProcess(t, driver, proc)

		params := captureRequestParams(t, captureFile, acpsdk.AgentMethodSessionNew)
		request := decodeCapturedNewSessionRequest(t, params)
		if got, want := len(request.MCPServers), 1; got != want {
			t.Fatalf("session/new mcpServers = %#v, want only hosted stdio entry", request.MCPServers)
		}
		stdio := request.MCPServers[0].Stdio
		if stdio == nil {
			t.Fatalf("session/new mcpServers[0] = %#v, want stdio variant", request.MCPServers[0])
		}
		if stdio.Name != "compozy-hosted-tools" || stdio.Command != "/bin/compozy" {
			t.Fatalf("hosted stdio entry = %#v, want hosted command", stdio)
		}
		if !slices.Equal(stdio.Args, []string{"tool", "mcp", "--session", "sess-1", "--bind-nonce", "nonce"}) {
			t.Fatalf("hosted stdio args = %#v, want tool mcp bind args", stdio.Args)
		}
		if got, want := len(stdio.Env), 1; got != want || stdio.Env[0].Name != "COMPOZY_HOME" {
			t.Fatalf("hosted stdio env = %#v, want COMPOZY_HOME only", stdio.Env)
		}
	})
}

func TestStartResumeReturnsSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		envScenario string
		wantErr     error
	}{
		"load session failure": {
			envScenario: "load_session_error",
			wantErr:     ErrLoadSessionFailed,
		},
		"agent missing load session support": {
			envScenario: "stream_updates",
			wantErr:     ErrAgentDoesNotSupportSession,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			driver := New()
			_, err := driver.Start(testutil.Context(t), StartOpts{
				AgentName:       "helper",
				Command:         helperCommand(t),
				Cwd:             t.TempDir(),
				Env:             helperEnv(tc.envScenario, ""),
				Permissions:     compozyconfig.PermissionModeApproveAll,
				ResumeSessionID: "sess-existing",
			})
			if err == nil {
				t.Fatalf("Start(%s) error = nil, want non-nil", tc.envScenario)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Start(%s) error = %v, want errors.Is(..., %v)", tc.envScenario, err, tc.wantErr)
			}
		})
	}
}

func TestStartIncludesAgentContextInLaunchErrors(t *testing.T) {
	t.Parallel()

	driver := New()
	_, err := driver.Start(testutil.Context(t), StartOpts{
		AgentName:   "missing-helper",
		Command:     "/definitely/missing-binary",
		Cwd:         t.TempDir(),
		Permissions: compozyconfig.PermissionModeApproveAll,
	})
	if err == nil {
		t.Fatal("Start() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `start agent "missing-helper" subprocess "/definitely/missing-binary"`) {
		t.Fatalf("Start() error = %q, want agent and command context", err)
	}
}

func TestIsLoadSessionResourceMissing(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want bool
	}{
		"ShouldDetectResourceMissingRequestError": {
			err: fmt.Errorf(
				"%w: load session %q for %q: %w",
				ErrLoadSessionFailed,
				"sess-existing",
				"helper",
				&acpsdk.RequestError{
					Code:    requestErrorResourceNotFoundCode,
					Message: "Resource not found: sess-existing",
				},
			),
			want: true,
		},
		"ShouldRejectDifferentRequestError": {
			err: fmt.Errorf(
				"%w: load session %q for %q: %w",
				ErrLoadSessionFailed,
				"sess-existing",
				"helper",
				&acpsdk.RequestError{Code: -32603, Message: "Internal error"},
			),
			want: false,
		},
		"ShouldRejectNonLoadSessionError": {
			err:  errors.New("boom"),
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := IsLoadSessionResourceMissing(tc.err); got != tc.want {
				t.Fatalf("IsLoadSessionResourceMissing() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCleanupFailedStartReturnsJoinedErrorWhenStopFails(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := &AgentProcess{
		done:   make(chan struct{}),
		stderr: &lockedBuffer{},
	}
	stopErr := errors.New("stop failed")
	proc.setWaitError(stopErr)
	close(proc.done)

	startErr := fmt.Errorf(
		"%w: load session %q for %q: %w",
		ErrLoadSessionFailed,
		"sess-existing",
		"helper",
		errors.New("load failed"),
	)
	err := driver.cleanupFailedStart(proc, startErr)
	if err == nil {
		t.Fatal("cleanupFailedStart() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrLoadSessionFailed) {
		t.Fatalf("cleanupFailedStart() error = %v, want ErrLoadSessionFailed", err)
	}
	if !errors.Is(err, stopErr) {
		t.Fatalf("cleanupFailedStart() error = %v, want stopErr", err)
	}
	if !strings.Contains(err.Error(), "stop failed while cleaning up failed start") {
		t.Fatalf("cleanupFailedStart() error = %v, want cleanup stop context", err)
	}
}

func TestCleanupFailedStartIncludesRedactedBoundedStderr(t *testing.T) {
	t.Parallel()

	const secret = "hermes-cleanup-secret"
	process := &AgentProcess{
		done:   make(chan struct{}),
		stderr: &lockedBuffer{},
	}
	if _, err := process.stderr.Write(
		[]byte("token=" + secret + " " + strings.Repeat("startup context ", 400)),
	); err != nil {
		t.Fatalf("stderr.Write() error = %v", err)
	}
	close(process.done)

	err := New().cleanupFailedStart(process, errors.New("session setup failed"))
	if err == nil {
		t.Fatal("cleanupFailedStart() error = nil, want non-nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("cleanupFailedStart() leaked secret: %v", err)
	}
	stderrIndex := strings.Index(err.Error(), "stderr=")
	if stderrIndex < 0 {
		t.Fatalf("cleanupFailedStart() = %v, want stderr context", err)
	}
	if got := len(err.Error()[stderrIndex+len("stderr="):]); got > maxFailureSummaryBytes {
		t.Fatalf("attached stderr length = %d, want <= %d", got, maxFailureSummaryBytes)
	}
}

func TestStartCapturesHermesModelStateAsTypedConfigOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		payload           string
		wantModel         string
		wantValues        []string
		wantOptionIDs     []string
		wantModelReadOnly bool
		wantReasoning     string
		wantBooleanID     string
		wantBooleanValue  bool
	}{
		{
			name:              "Should capture models from session new when config options omit a model",
			payload:           `{"sessionId":"sess-new","models":{"currentModelId":"openrouter:grok-4.6","availableModels":[{"modelId":"openrouter:grok-4.6","name":"Grok 4.6","description":"fast"},{"modelId":"openrouter:opus-5","name":"Opus 5"}]},"configOptions":[{"type":"select","id":"reasoning_effort","name":"Reasoning","currentValue":"high","options":[{"value":"high","name":"High"}]}]}`,
			wantModel:         "openrouter:grok-4.6",
			wantValues:        []string{"openrouter:grok-4.6", "openrouter:opus-5"},
			wantOptionIDs:     []string{"reasoning_effort", "model"},
			wantModelReadOnly: true,
			wantReasoning:     "high",
		},
		{
			name:              "Should capture models from session load when config options omit a model",
			payload:           `{"models":{"currentModelId":"anthropic:claude-opus-5","availableModels":[{"modelId":"anthropic:claude-opus-5","name":"Claude Opus 5"}]},"configOptions":[{"type":"boolean","id":"thinking","name":"Thinking","currentValue":true}]}`,
			wantModel:         "anthropic:claude-opus-5",
			wantValues:        []string{"anthropic:claude-opus-5"},
			wantOptionIDs:     []string{"thinking", "model"},
			wantModelReadOnly: true,
			wantBooleanID:     "thinking",
			wantBooleanValue:  true,
		},
		{
			name:             "Should preserve an advertised model config option over the Hermes extension",
			payload:          `{"models":{"currentModelId":"hermes:extension-model","availableModels":[{"modelId":"hermes:extension-model","name":"Extension model"}]},"configOptions":[{"type":"select","id":"model","name":"Model","currentValue":"provider:configured-model","options":[{"value":"provider:configured-model","name":"Configured model"}]},{"type":"boolean","id":"thinking","name":"Thinking","currentValue":false}]}`,
			wantModel:        "provider:configured-model",
			wantValues:       []string{"provider:configured-model"},
			wantOptionIDs:    []string{"model", "thinking"},
			wantBooleanID:    "thinking",
			wantBooleanValue: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var response wireSessionSetupResponse
			if err := json.Unmarshal([]byte(tc.payload), &response); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			caps := captureSessionSetupCaps(Caps{}, response)

			if got, want := configOptionIDs(caps.ConfigOptions), tc.wantOptionIDs; !slices.Equal(got, want) {
				t.Fatalf("config option ids = %#v, want %#v", got, want)
			}
			model, ok := ModelConfigOption(caps.ConfigOptions)
			if !ok {
				t.Fatal("ModelConfigOption() ok = false, want true")
			}
			if model.CurrentValueID != tc.wantModel {
				t.Fatalf("model current value = %q, want %q", model.CurrentValueID, tc.wantModel)
			}
			if model.ReadOnly != tc.wantModelReadOnly {
				t.Fatalf("model read-only = %t, want %t", model.ReadOnly, tc.wantModelReadOnly)
			}
			values := make([]string, 0, len(model.Values))
			for _, value := range model.Values {
				values = append(values, value.Value)
			}
			if !slices.Equal(values, tc.wantValues) {
				t.Fatalf("model values = %#v, want %#v", values, tc.wantValues)
			}
			if tc.wantReasoning != "" {
				reasoning, ok := findConfigOptionByID(caps.ConfigOptions, "reasoning_effort")
				if !ok || reasoning.CurrentValueID != tc.wantReasoning {
					t.Fatalf("reasoning option = %#v, %v, want current %q", reasoning, ok, tc.wantReasoning)
				}
			}
			if tc.wantBooleanID != "" {
				boolean, ok := findConfigOptionByID(caps.ConfigOptions, tc.wantBooleanID)
				if !ok || boolean.CurrentBool == nil || *boolean.CurrentBool != tc.wantBooleanValue {
					t.Fatalf("boolean option = %#v, %v, want current %t", boolean, ok, tc.wantBooleanValue)
				}
			}
		})
	}
}

func configOptionIDs(options []SessionConfigOption) []string {
	ids := make([]string, 0, len(options))
	for _, option := range options {
		ids = append(ids, option.ID)
	}
	return ids
}

func TestHermesDiscoveryModelIsReadOnlyForModernACP(t *testing.T) {
	t.Parallel()

	modelOption, ok := sessionModelConfigOption(&wireSessionModelState{
		CurrentModelID: "openrouter:grok-4.6",
		AvailableModels: []wireSessionModelInfo{
			{ModelID: "openrouter:grok-4.6", Name: "Grok 4.6"},
		},
	})
	if !ok {
		t.Fatal("sessionModelConfigOption() ok = false, want true")
	}
	if !modelOption.ReadOnly {
		t.Fatal("discovered model option read-only = false, want true")
	}

	process := &AgentProcess{
		conn: &acpsdk.Connection{},
		caps: Caps{ConfigOptions: []SessionConfigOption{modelOption}},
	}
	applied, err := New().applySessionModel(
		context.Background(),
		process,
		"openrouter:grok-4.6",
	)
	if applied {
		t.Fatal("applySessionModel() applied = true, want false for discovery-only model")
	}
	if !errors.Is(err, errModelConfigOptionReadOnly) {
		t.Fatalf("applySessionModel() error = %v, want read-only diagnostic", err)
	}
	if !strings.Contains(err.Error(), "session/set_model") {
		t.Fatalf("applySessionModel() error = %v, want session/set_model guidance", err)
	}
}

func TestAttachStderrRedactsAndBoundsSetupDiagnostics(t *testing.T) {
	t.Parallel()

	const secret = "super-secret-hermes-token"
	rawStderr := "token=" + secret + " " + strings.Repeat("startup context ", 400)
	err := attachStderr(errors.New("ACP initialize handshake failed"), rawStderr)
	if err == nil {
		t.Fatal("attachStderr() error = nil, want wrapped error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("attachStderr() leaked secret: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("attachStderr() = %v, want redaction marker", err)
	}
	stderrIndex := strings.Index(err.Error(), "stderr=")
	if stderrIndex < 0 {
		t.Fatalf("attachStderr() = %v, want stderr context", err)
	}
	if got := len(err.Error()[stderrIndex+len("stderr="):]); got > maxFailureSummaryBytes {
		t.Fatalf("attached stderr length = %d, want <= %d", got, maxFailureSummaryBytes)
	}
}
