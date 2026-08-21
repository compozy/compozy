package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/compozy/compozy/internal/windowmanager"
)

func TestConfigCommandsMutateValidateAndInspectTempHome(t *testing.T) {
	t.Parallel()
	t.Run("Should mutate validate and inspect a temporary home [IT-016]", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}

		setOut, _, err := executeRootCommand(t, deps, "config", "set", "defaults.provider", "claude", "-o", "json")
		if err != nil {
			t.Fatalf("config set defaults.provider error = %v", err)
		}
		var setRecord configSetRecord
		if err := json.Unmarshal([]byte(setOut), &setRecord); err != nil {
			t.Fatalf("json.Unmarshal(config set) error = %v", err)
		}
		if setRecord.Path != "defaults.provider" || setRecord.Value != "claude" {
			t.Fatalf("config set record = %#v, want defaults.provider=claude", setRecord)
		}
		sandboxOut, _, err := executeRootCommand(t, deps, "config", "set", "defaults.sandbox", "local", "-o", "json")
		if err != nil {
			t.Fatalf("config set defaults.sandbox error = %v", err)
		}
		var sandboxSetRecord configSetRecord
		if err := json.Unmarshal([]byte(sandboxOut), &sandboxSetRecord); err != nil {
			t.Fatalf("json.Unmarshal(config set defaults.sandbox) error = %v", err)
		}
		if sandboxSetRecord.Path != "defaults.sandbox" || sandboxSetRecord.Value != "local" {
			t.Fatalf("config set sandbox record = %#v, want defaults.sandbox=local", sandboxSetRecord)
		}
		deadlineOut, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"session.supervision.prompt_deadline",
			"8s",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("config set session.supervision.prompt_deadline error = %v", err)
		}
		var deadlineSetRecord configSetRecord
		if err := json.Unmarshal([]byte(deadlineOut), &deadlineSetRecord); err != nil {
			t.Fatalf("json.Unmarshal(config set session.supervision.prompt_deadline) error = %v", err)
		}
		if deadlineSetRecord.Path != "session.supervision.prompt_deadline" || deadlineSetRecord.Value != "8s" {
			t.Fatalf(
				"config set prompt deadline record = %#v, want session.supervision.prompt_deadline=8s",
				deadlineSetRecord,
			)
		}

		cfg, err := compozyconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig() error = %v", err)
		}
		if cfg.Defaults.Provider != "claude" {
			t.Fatalf("Defaults.Provider = %q, want claude", cfg.Defaults.Provider)
		}
		if cfg.Defaults.Sandbox != "local" {
			t.Fatalf("Defaults.Sandbox = %q, want local", cfg.Defaults.Sandbox)
		}
		if got, want := cfg.Session.Supervision.PromptDeadline.String(), "8s"; got != want {
			t.Fatalf("Session.Supervision.PromptDeadline = %q, want %q", got, want)
		}

		getOut, _, err := executeRootCommand(t, deps, "config", "get", "defaults.provider", "-o", "json")
		if err != nil {
			t.Fatalf("config get defaults.provider error = %v", err)
		}
		var valueRecord configValueRecord
		if err := json.Unmarshal([]byte(getOut), &valueRecord); err != nil {
			t.Fatalf("json.Unmarshal(config get) error = %v", err)
		}
		if valueRecord.Value != "claude" || valueRecord.Redacted {
			t.Fatalf("config get record = %#v, want unredacted claude", valueRecord)
		}

		validateOut, _, err := executeRootCommand(t, deps, "config", "validate", "-o", "json")
		if err != nil {
			t.Fatalf("config validate error = %v", err)
		}
		var validateRecord configValidateRecord
		if err := json.Unmarshal([]byte(validateOut), &validateRecord); err != nil {
			t.Fatalf("json.Unmarshal(config validate) error = %v", err)
		}
		if validateRecord.Status != "valid" || validateRecord.ConfigFile != homePaths.ConfigFile {
			t.Fatalf("config validate record = %#v, want valid config file", validateRecord)
		}

		pathOut, _, err := executeRootCommand(t, deps, "config", "path", "-o", "json")
		if err != nil {
			t.Fatalf("config path error = %v", err)
		}
		var pathRecord configPathRecord
		if err := json.Unmarshal([]byte(pathOut), &pathRecord); err != nil {
			t.Fatalf("json.Unmarshal(config path) error = %v", err)
		}
		if pathRecord.GlobalConfig != homePaths.ConfigFile ||
			pathRecord.GlobalMCPJSON != filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName) {
			t.Fatalf("config path record = %#v, want resolved home paths", pathRecord)
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"window_manager.snap.repeat_ratios",
			`[0.5,0.75,0.25]`,
		); err != nil {
			t.Fatalf("config set window manager ratios error = %v", err)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"window_manager.shortcuts.window.focus.left",
			`["alt+KeyH","control+alt+shift+KeyH"]`,
		); err != nil {
			t.Fatalf("config set window manager shortcut error = %v", err)
		}
		configured, err := compozyconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(window manager) error = %v", err)
		}
		if got := configured.WindowManager.Snap.RepeatRatios; len(got) != 3 || got[1] != 0.75 {
			t.Fatalf("WindowManager.Snap.RepeatRatios = %#v, want [0.5 0.75 0.25]", got)
		}
		if got, want := configured.WindowManager.Shortcuts["window.focus.left"], (windowmanager.ShortcutBinding{"alt+KeyH", "control+alt+shift+KeyH"}); !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("WindowManager.Shortcuts[window.focus.left] = %q, want %q", got, want)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"window_manager.global_shortcuts.session.new",
			"meta+shift+KeyN",
		); err != nil {
			t.Fatalf("config set window manager global shortcut error = %v", err)
		}
		configured, err = compozyconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(global shortcuts) error = %v", err)
		}
		if got, want := configured.WindowManager.GlobalShortcuts["session.new"], "meta+shift+KeyN"; got != want {
			t.Fatalf("WindowManager.GlobalShortcuts[session.new] = %q, want %q", got, want)
		}
		discoveryOut, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"get",
			"window_manager",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("config get window_manager error = %v", err)
		}
		var discoveryRecord configValueRecord
		if err := json.Unmarshal([]byte(discoveryOut), &discoveryRecord); err != nil {
			t.Fatalf("json.Unmarshal(config get window_manager) error = %v", err)
		}
		discovery, ok := discoveryRecord.Value.(map[string]any)
		if !ok || discovery["defaults"] == nil || discovery["effective"] == nil ||
			!strings.Contains(discoveryOut, "global_shortcuts") ||
			!strings.Contains(discoveryOut, "session.new") {
			t.Fatalf("config get window_manager value = %#v, want global_shortcuts.session.new", discoveryRecord.Value)
		}
		if _, _, invalidErr := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"window_manager.shortcuts.window.focus.left",
			`["alt+KeyH",42]`,
		); invalidErr == nil {
			t.Fatal("config set invalid shortcut array error = nil")
		}
		unchanged, err := compozyconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(after rejected shortcut array) error = %v", err)
		}
		if got, want := unchanged.WindowManager.Shortcuts["window.focus.left"], (windowmanager.ShortcutBinding{"alt+KeyH", "control+alt+shift+KeyH"}); !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("shortcut after atomic rejection = %q, want %q", got, want)
		}

		for _, mutation := range [][2]string{
			{"attention.toasts", "false"},
			{"attention.sound", "false"},
			{"attention.system", "true"},
			{"attention.muted_workspaces", `["ws_0123456789abcdef"]`},
		} {
			if _, _, err := executeRootCommand(t, deps, "config", "set", mutation[0], mutation[1]); err != nil {
				t.Fatalf("config set %s error = %v", mutation[0], err)
			}
		}
		configured, err = compozyconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(attention) error = %v", err)
		}
		if configured.Attention.Toasts || configured.Attention.Sound || !configured.Attention.System ||
			!slices.Equal(configured.Attention.MutedWorkspaces, []string{"ws_0123456789abcdef"}) {
			t.Fatalf("Attention = %#v, want false/false/true with one muted workspace", configured.Attention)
		}

		for _, mutation := range [][2]string{
			{"shell.sessions.sort", "attention"},
			{"shell.sessions.scope", "all-workspaces"},
		} {
			if _, _, err := executeRootCommand(t, deps, "config", "set", mutation[0], mutation[1]); err != nil {
				t.Fatalf("config set %s error = %v", mutation[0], err)
			}
		}
		configured, err = compozyconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(shell) error = %v", err)
		}
		if configured.Shell.Sessions.Sort != compozyconfig.ShellSessionSortAttention ||
			configured.Shell.Sessions.Scope != compozyconfig.ShellSessionScopeAllWorkspaces {
			t.Fatalf("Shell = %#v, want attention/all-workspaces", configured.Shell)
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"roles",
			`{"memory_controller":{"enabled":true,"provider":"pi","model":"anthropic/claude-haiku-4","timeout":"500ms","top_k":7,"prompt_version":"v2","max_tokens_out":512}}`,
		); err != nil {
			t.Fatalf("config set roles error = %v", err)
		}
		configured, err = compozyconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(roles) error = %v", err)
		}
		if got := configured.Roles.MemoryController; got.TopK != 7 || got.MaxTokensOut != 512 {
			t.Fatalf("Roles.MemoryController = %#v, want top_k=7 max_tokens_out=512", got)
		}
	})
}

func TestConfigSetShouldManageLoopLifecycleDefaults(t *testing.T) {
	t.Parallel()
	t.Run("Should persist every agent mutable loop lifecycle path", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		values := []struct {
			path  string
			value string
		}{
			{path: "loops.defaults.delivery.retry.max_attempts", value: "4"},
			{path: "loops.defaults.delivery.retry.backoff_base", value: "2s"},
			{path: "loops.defaults.delivery.retry.backoff_max", value: "45s"},
			{path: "loops.defaults.delivery.liveness.silence_window", value: "20m"},
			{path: "loops.defaults.delivery.resume.death_streak_limit", value: "4"},
			{path: "loops.defaults.delivery.predicates.cost_limit", value: "12000"},
			{path: "loops.defaults.delivery.waits.admission_attempts", value: "4"},
			{path: "loops.defaults.delivery.waits.admission_retry_interval", value: "75s"},
			{path: "loops.defaults.delivery.admission.tombstone_horizon", value: "240h"},
			{path: "loops.defaults.watch.retry.max_attempts", value: "2"},
			{path: "loops.defaults.watch.retry.backoff_base", value: "3s"},
			{path: "loops.defaults.watch.retry.backoff_max", value: "20s"},
			{path: "loops.defaults.watch.liveness.silence_window", value: "0s"},
			{path: "loops.defaults.watch.resume.death_streak_limit", value: "5"},
			{path: "loops.defaults.watch.predicates.cost_limit", value: "9000"},
			{path: "loops.defaults.watch.waits.admission_attempts", value: "2"},
			{path: "loops.defaults.watch.waits.admission_retry_interval", value: "30s"},
			{path: "loops.defaults.watch.admission.tombstone_horizon", value: "96h"},
			{path: "loops.breaker.threshold", value: "7"},
			{path: "loops.breaker.probe_interval", value: "90s"},
		}

		for _, value := range values {
			t.Run("Should set and get "+value.path, func(t *testing.T) {
				setOut, _, err := executeRootCommand(
					t,
					deps,
					"config",
					"set",
					value.path,
					value.value,
					"-o",
					"json",
				)
				if err != nil {
					t.Fatalf("config set %s error = %v", value.path, err)
				}

				var setRecord configSetRecord
				if err := json.Unmarshal([]byte(setOut), &setRecord); err != nil {
					t.Fatalf("json.Unmarshal(config set %s) error = %v", value.path, err)
				}
				if setRecord.Path != value.path {
					t.Fatalf("config set record path = %q, want %q", setRecord.Path, value.path)
				}

				getOut, _, err := executeRootCommand(t, deps, "config", "get", value.path, "-o", "json")
				if err != nil {
					t.Fatalf("config get %s error = %v", value.path, err)
				}
				var valueRecord configValueRecord
				if err := json.Unmarshal([]byte(getOut), &valueRecord); err != nil {
					t.Fatalf("json.Unmarshal(config get %s) error = %v", value.path, err)
				}
				if got := fmt.Sprint(valueRecord.Value); got != value.value {
					t.Fatalf("config get %s value = %q, want %q", value.path, got, value.value)
				}
			})
		}
	})
}

func TestConfigCommandsManageDynamicLoopInputDefaults(t *testing.T) {
	t.Parallel()

	var calls []contract.PutLoopInputDefaultRequest
	client := &stubClient{
		putLoopInputDefaultFn: func(
			_ context.Context,
			workspaceID string,
			loopName string,
			key string,
			request contract.PutLoopInputDefaultRequest,
		) (contract.LoopInputDefaultResponse, error) {
			if workspaceID != "/workspace/project" || loopName != "review-and-fix" || key == "" {
				t.Fatalf("PutLoopInputDefault() target = %s/%s/%s", workspaceID, loopName, key)
			}
			calls = append(calls, request)
			return contract.LoopInputDefaultResponse{
				LoopName: loopName, Key: key, Scope: request.Scope, Present: true, Value: request.Value,
			}, nil
		},
	}
	deps := newWorkspaceTestDeps(t, client)
	for _, testCase := range []struct {
		path      string
		rawValue  string
		wantValue any
	}{
		{path: "loops.inputs.review-and-fix.auto_commit", rawValue: "false", wantValue: false},
		{path: "loops.inputs.review-and-fix.retries", rawValue: "0", wantValue: float64(0)},
		{path: "loops.inputs.review-and-fix.reviewer", rawValue: "", wantValue: ""},
		{
			path:      "loops.inputs.review-and-fix.runtime",
			rawValue:  `{"provider":"codex","model":"gpt-5.6","reasoning":"high"}`,
			wantValue: map[string]any{"provider": "codex", "model": "gpt-5.6", "reasoning": "high"},
		},
	} {
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"config", "set", testCase.path, testCase.rawValue, "-o", "json",
		)
		if err != nil {
			t.Fatalf("config set %s error = %v", testCase.path, err)
		}
		var setRecord configSetRecord
		if err := json.Unmarshal([]byte(stdout), &setRecord); err != nil {
			t.Fatalf("json.Unmarshal(config set %s) error = %v", testCase.path, err)
		}
		if setRecord.Path != testCase.path || !reflect.DeepEqual(setRecord.Value, testCase.wantValue) {
			t.Fatalf("config set %s record = %#v, want %#v", testCase.path, setRecord, testCase.wantValue)
		}
	}
	if len(calls) != 4 {
		t.Fatalf("PutLoopInputDefault() calls = %d, want 4", len(calls))
	}
	for _, call := range calls {
		if call.Scope != contract.LoopInputDefaultsScopeUser {
			t.Fatalf("PutLoopInputDefault() scope = %q, want user", call.Scope)
		}
	}
}

func TestConfigSetReportsMutationLifecycle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		path             string
		value            string
		wantLifecycle    string
		wantApplied      bool
		wantRestart      bool
		wantRestartScope string
	}{
		{
			name:             "Should report daemon restart for persisted log config",
			path:             "log.level",
			value:            "debug",
			wantLifecycle:    "restart-required",
			wantRestart:      true,
			wantRestartScope: "daemon",
		},
		{
			name:          "Should report applied now for disabled skills",
			path:          "skills.disabled_skills",
			value:         `["agent-browser"]`,
			wantLifecycle: "live",
			wantApplied:   true,
		},
		{
			name:          "Should apply command palette personalization live",
			path:          "cmd_palette.personalization",
			value:         "false",
			wantLifecycle: "live",
			wantApplied:   true,
		},
		{
			name:          "Should apply command palette fallback targets live",
			path:          "cmd_palette.fallback_targets",
			value:         `["agent"]`,
			wantLifecycle: "live",
			wantApplied:   true,
		},
		{
			name:          "Should apply the extension side-load policy live",
			path:          "extensions.trust.allow_unverified",
			value:         "true",
			wantLifecycle: "live",
			wantApplied:   true,
		},
		{
			name:          "Should apply the Marketplace catalog base URL live",
			path:          "marketplace.catalog.base_url",
			value:         "http://127.0.0.1:64135",
			wantLifecycle: "live",
			wantApplied:   true,
		},
		{
			name:          "Should apply the Marketplace catalog TTL live",
			path:          "marketplace.catalog.ttl",
			value:         "30m",
			wantLifecycle: "live",
			wantApplied:   true,
		},
		{
			name:          "Should apply the Marketplace catalog timeout live",
			path:          "marketplace.catalog.timeout",
			value:         "5s",
			wantLifecycle: "live",
			wantApplied:   true,
		},
		{
			name:             "Should report daemon restart for loop defaults",
			path:             "loops.defaults.delivery.fan_out_width",
			value:            "3",
			wantLifecycle:    "restart-required",
			wantRestart:      true,
			wantRestartScope: "daemon",
		},
		{
			name:             "Should report daemon restart for Goal context policy",
			path:             "goals.context_nudge_ratio",
			value:            "0.4",
			wantLifecycle:    "restart-required",
			wantRestart:      true,
			wantRestartScope: "daemon",
		},
		{
			name:             "Should report daemon restart for Goal outbox batch size",
			path:             "goals.outbox_batch_size",
			value:            "64",
			wantLifecycle:    "restart-required",
			wantRestart:      true,
			wantRestartScope: "daemon",
		},
		{
			name:             "Should report daemon restart for Goal outbox poll interval",
			path:             "goals.outbox_poll_interval",
			value:            "750ms",
			wantLifecycle:    "restart-required",
			wantRestart:      true,
			wantRestartScope: "daemon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newWorkspaceTestDeps(t, &stubClient{})
			out, _, err := executeRootCommand(t, deps, "config", "set", tt.path, tt.value, "-o", "json")
			if err != nil {
				t.Fatalf("config set %s error = %v", tt.path, err)
			}
			var record configSetRecord
			if err := json.Unmarshal([]byte(out), &record); err != nil {
				t.Fatalf("json.Unmarshal(config set %s) error = %v", tt.path, err)
			}
			if record.Lifecycle != tt.wantLifecycle {
				t.Fatalf("config set %s lifecycle = %q, want %q", tt.path, record.Lifecycle, tt.wantLifecycle)
			}
			if record.Applied != tt.wantApplied {
				t.Fatalf("config set %s applied = %v, want %v", tt.path, record.Applied, tt.wantApplied)
			}
			if record.RestartRequired != tt.wantRestart {
				t.Fatalf(
					"config set %s restart_required = %v, want %v",
					tt.path,
					record.RestartRequired,
					tt.wantRestart,
				)
			}
			if record.RestartScope != tt.wantRestartScope {
				t.Fatalf("config set %s restart_scope = %q, want %q", tt.path, record.RestartScope, tt.wantRestartScope)
			}
		})
	}

	t.Run("Should reject global-only Marketplace catalog writes at workspace scope", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		workspaceRoot := t.TempDir()
		_, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"marketplace.catalog.ttl",
			"30m",
			"--scope",
			"workspace",
			"--workspace",
			workspaceRoot,
		)
		if err == nil || !strings.Contains(err.Error(), "global-only") {
			t.Fatalf("config set workspace Marketplace error = %v, want global-only rejection", err)
		}
		workspaceConfig := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName)
		if _, statErr := os.Stat(workspaceConfig); !os.IsNotExist(statErr) {
			t.Fatalf("workspace config stat error = %v, want no file", statErr)
		}
	})
}

func TestConfigSetDisabledSkillsUsesDaemonSettingsWhenRunning(t *testing.T) {
	t.Parallel()

	var captured UpdateSettingsSkillsRequest
	client := &stubClient{
		updateSettingsSkillsFn: func(
			_ context.Context,
			request UpdateSettingsSkillsRequest,
		) (SettingsMutationRecord, error) {
			captured = request
			return SettingsMutationRecord{
				Section:          "skills",
				Scope:            contract.SettingsScopeGlobal,
				Lifecycle:        contract.SettingsApplyLifecycleLive,
				ApplyRecordID:    "cfgapp-test",
				Applied:          true,
				ActiveGeneration: 1,
				ActiveConfigHash: "sha256:test",
				NextAction:       contract.SettingsApplyNextActionNone,
			}, nil
		},
	}

	deps := newWorkspaceTestDeps(t, client)
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 42, Port: 2123, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(pid int) bool { return pid == 42 }

	out, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"skills.disabled_skills",
		`["agent-browser"]`,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config set disabled skills error = %v", err)
	}

	if got, want := captured.Config.DisabledSkills, []string{
		"agent-browser",
	}; strings.Join(
		got,
		",",
	) != strings.Join(
		want,
		",",
	) {
		t.Fatalf("daemon settings payload disabled_skills = %#v, want %#v", got, want)
	}
	if !captured.Config.Enabled {
		t.Fatal("daemon settings payload unexpectedly disabled skills subsystem")
	}

	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	if _, err := os.Stat(homePaths.ConfigFile); !os.IsNotExist(err) {
		t.Fatalf("config set wrote local overlay while daemon-backed path should own persistence: stat err=%v", err)
	}

	var record configSetRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(config set disabled skills) error = %v", err)
	}
	if record.Lifecycle != string(contract.SettingsApplyLifecycleLive) || !record.Applied {
		t.Fatalf("config set disabled skills record = %#v, want live/applied=true", record)
	}
}

func TestConfigSetWindowManagerWritesOverlayAndReloadsWhenDaemonRuns(t *testing.T) {
	t.Parallel()

	var reloadCalls int
	client := &stubClient{
		reloadSettingsFn: func(context.Context) (SettingsMutationRecord, error) {
			reloadCalls++
			return SettingsMutationRecord{
				Section:          "window-manager",
				Scope:            contract.SettingsScopeGlobal,
				Lifecycle:        contract.SettingsApplyLifecycleLive,
				ApplyRecordID:    "cfgapp-window-manager",
				Applied:          true,
				ActiveGeneration: 2,
				ActiveConfigHash: "sha256:window-manager",
				NextAction:       contract.SettingsApplyNextActionNone,
			}, nil
		},
	}

	deps := newWorkspaceTestDeps(t, client)
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 42, Port: 2123, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(pid int) bool { return pid == 42 }

	out, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"window_manager.nav_stack_limit",
		"1",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config set window_manager.nav_stack_limit error = %v", err)
	}
	if reloadCalls != 1 {
		t.Fatalf("ReloadSettings calls = %d, want 1", reloadCalls)
	}

	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	cfg, err := compozyconfig.LoadGlobalConfig(homePaths)
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if cfg.WindowManager.NavStackLimit != 1 {
		t.Fatalf("WindowManager.NavStackLimit = %d, want 1", cfg.WindowManager.NavStackLimit)
	}

	var record configSetRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(config set window manager) error = %v", err)
	}
	if record.Lifecycle != string(contract.SettingsApplyLifecycleLive) || !record.Applied ||
		record.RestartRequired {
		t.Fatalf("config set window manager record = %#v, want live/applied=true", record)
	}
}

func TestConfigSetAttentionUsesDaemonSettingsWhenRunning(t *testing.T) {
	t.Parallel()

	var captured UpdateSettingsAttentionRequest
	client := &stubClient{
		updateSettingsAttentionFn: func(
			_ context.Context,
			request UpdateSettingsAttentionRequest,
		) (SettingsMutationRecord, error) {
			captured = request
			return SettingsMutationRecord{
				Section:          "attention",
				Scope:            contract.SettingsScopeGlobal,
				Lifecycle:        contract.SettingsApplyLifecycleLive,
				ApplyRecordID:    "cfgapp-attention",
				Applied:          true,
				ActiveGeneration: 3,
				ActiveConfigHash: "sha256:attention",
				NextAction:       contract.SettingsApplyNextActionNone,
			}, nil
		},
	}

	deps := newWorkspaceTestDeps(t, client)
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 42, Port: 2123, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(pid int) bool { return pid == 42 }

	out, _, err := executeRootCommand(t, deps, "config", "set", "attention.system", "true", "-o", "json")
	if err != nil {
		t.Fatalf("config set attention.system error = %v", err)
	}
	if !captured.Config.System || !captured.Config.Toasts || !captured.Config.Sound ||
		len(captured.Config.MutedWorkspaces) != 0 {
		t.Fatalf("daemon attention payload = %#v, want complete defaults with system=true", captured.Config)
	}
	if captured.Config.MutedWorkspaces == nil {
		t.Fatal("daemon attention muted_workspaces = nil, want a non-null empty array")
	}
	payload, err := json.Marshal(captured.Config)
	if err != nil {
		t.Fatalf("json.Marshal(daemon attention payload) error = %v", err)
	}
	if !bytes.Contains(payload, []byte(`"muted_workspaces":[]`)) {
		t.Fatalf("daemon attention payload JSON = %s, want muted_workspaces:[]", payload)
	}

	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	if _, err := os.Stat(homePaths.ConfigFile); !os.IsNotExist(err) {
		t.Fatalf("config set wrote local overlay while daemon-backed path should own persistence: stat err=%v", err)
	}

	var record configSetRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(config set attention) error = %v", err)
	}
	if record.Lifecycle != string(contract.SettingsApplyLifecycleLive) || !record.Applied ||
		record.RestartRequired {
		t.Fatalf("config set attention record = %#v, want live/applied=true", record)
	}
}

func TestConfigSetShellUsesDaemonSettingsWhenRunning(t *testing.T) {
	t.Parallel()

	var captured UpdateSettingsShellRequest
	client := &stubClient{
		updateSettingsShellFn: func(
			_ context.Context,
			request UpdateSettingsShellRequest,
		) (SettingsMutationRecord, error) {
			captured = request
			return SettingsMutationRecord{
				Section:          "shell",
				Scope:            contract.SettingsScopeGlobal,
				Lifecycle:        contract.SettingsApplyLifecycleLive,
				ApplyRecordID:    "cfgapp-shell",
				Applied:          true,
				ActiveGeneration: 4,
				ActiveConfigHash: "sha256:shell",
				NextAction:       contract.SettingsApplyNextActionNone,
			}, nil
		},
	}

	deps := newWorkspaceTestDeps(t, client)
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 42, Port: 2123, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(pid int) bool { return pid == 42 }

	out, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"shell.sessions.scope",
		"all-workspaces",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config set shell.sessions.scope error = %v", err)
	}
	if captured.Config.Sessions.Sort != contract.SettingsShellSessionSortLastActivity ||
		captured.Config.Sessions.Scope != contract.SettingsShellSessionScopeAllWorkspaces {
		t.Fatalf("daemon shell payload = %#v, want last_activity/all-workspaces", captured.Config)
	}

	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	if _, err := os.Stat(homePaths.ConfigFile); !os.IsNotExist(err) {
		t.Fatalf("config set wrote local overlay while daemon-backed path should own persistence: stat err=%v", err)
	}

	var record configSetRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(config set shell) error = %v", err)
	}
	if record.Lifecycle != string(contract.SettingsApplyLifecycleLive) || !record.Applied ||
		record.RestartRequired {
		t.Fatalf("config set shell record = %#v, want live/applied=true", record)
	}
}

func TestConfigSetReloadsReachableDaemonWhenProcessTimestampMetadataLags(t *testing.T) {
	t.Parallel()

	reloadCalled := false
	client := &stubClient{
		daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			return DaemonStatus{Status: "running", PID: 42}, nil
		},
		reloadSettingsFn: func(context.Context) (SettingsMutationRecord, error) {
			reloadCalled = true
			return SettingsMutationRecord{
				Lifecycle:        contract.SettingsApplyLifecycleLive,
				ApplyRecordID:    "cfgapp-late-metadata",
				Applied:          true,
				ActiveGeneration: 2,
				ActiveConfigHash: "sha256:late-metadata",
				NextAction:       contract.SettingsApplyNextActionNone,
			}, nil
		},
	}

	deps := newWorkspaceTestDeps(t, client)
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 42, Port: 2123, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(pid int) bool { return pid == 42 }
	deps.processMatchesStartTime = func(int, time.Time) bool { return false }

	out, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"marketplace.catalog.base_url",
		"http://127.0.0.1:64135",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config set marketplace catalog base URL error = %v", err)
	}
	if !reloadCalled {
		t.Fatal("ReloadSettings was not called through the reachable daemon")
	}

	var record configSetRecord
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("json.Unmarshal(config set marketplace catalog base URL) error = %v", err)
	}
	if record.ApplyRecordID != "cfgapp-late-metadata" || record.ActiveGeneration != 2 || !record.Applied {
		t.Fatalf("config set record = %#v, want daemon-confirmed active generation", record)
	}
}

func TestConfigSetRejectsRemovedMutationPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "Should reject legacy defaults environment", path: "defaults.environment"},
		{name: "Should reject legacy environment profile", path: "environments.dev.backend"},
		{name: "Should reject recall signal metrics", path: "memory.recall.signals.metrics_enabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newWorkspaceTestDeps(t, &stubClient{})
			_, _, err := executeRootCommand(t, deps, "config", "set", tt.path, "local")
			if err == nil {
				t.Fatalf("config set %s error = nil, want unsupported path", tt.path)
			}
			if !strings.Contains(err.Error(), "is not supported by config set") {
				t.Fatalf("config set %s error = %v, want unsupported path", tt.path, err)
			}
		})
	}
}

func TestConfigValidateRepairEnvRepairsWorkspaceDotEnvWithoutLeakingValues(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{})
	workspaceRoot := t.TempDir()
	dotenvPath := filepath.Join(workspaceRoot, ".env")
	before := "COMPOZY_TASK09_API_KEY=very-secret\u200b-token OTHER=value\n"
	if err := os.WriteFile(dotenvPath, []byte(before), 0o600); err != nil {
		t.Fatalf("os.WriteFile(.env) error = %v", err)
	}

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"validate",
		"--workspace",
		workspaceRoot,
		"--repair-env",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config validate --repair-env error = %v", err)
	}
	if strings.Contains(stdout, "very-secret") {
		t.Fatalf("config validate --repair-env leaked .env value:\n%s", stdout)
	}

	var record configValidateRecord
	if err := json.Unmarshal([]byte(stdout), &record); err != nil {
		t.Fatalf("json.Unmarshal(config validate --repair-env) error = %v", err)
	}
	if record.DotEnv == nil {
		t.Fatal("config validate --repair-env DotEnv = nil, want repair report")
	}
	if record.DotEnv.Status != compozyconfig.DotEnvStatusRepaired || !record.DotEnv.Repaired {
		t.Fatalf("DotEnv report = %#v, want repaired", record.DotEnv)
	}
	if len(record.DotEnv.Diagnostics) != 2 {
		t.Fatalf("DotEnv diagnostics = %#v, want multi-key and sanitizer diagnostics", record.DotEnv.Diagnostics)
	}

	after, readErr := os.ReadFile(dotenvPath)
	if readErr != nil {
		t.Fatalf("os.ReadFile(.env after repair) error = %v", readErr)
	}
	for _, want := range []string{
		"COMPOZY_TASK09_API_KEY=very-secret-token",
		"OTHER=value",
	} {
		if !strings.Contains(string(after), want) {
			t.Fatalf("repaired .env missing %q:\n%s", want, string(after))
		}
	}
}

func TestConfigValidateUsesWorkspaceDotEnvForHomeResolution(t *testing.T) {
	workspaceRoot := t.TempDir()
	nested := filepath.Join(workspaceRoot, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}
	dotEnvHome := filepath.Join(t.TempDir(), "dotenv-home")
	processHome := filepath.Join(t.TempDir(), "process-home")
	dotenvPath := filepath.Join(workspaceRoot, ".env")
	if err := os.WriteFile(dotenvPath, []byte("COMPOZY_HOME="+dotEnvHome+"\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(.env) error = %v", err)
	}

	deps := newWorkspaceTestDeps(t, &stubClient{
		getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
			return WorkspaceDetailRecord{
				Workspace: WorkspaceRecord{ID: "ws-root", RootDir: workspaceRoot},
			}, nil
		},
	})
	deps.getwd = func() (string, error) {
		return nested, nil
	}
	deps.resolveHomeForWorkspace = compozyconfig.ResolveHomePathsForWorkspace

	restoreEnvAfterTest(t, "COMPOZY_HOME")
	if err := os.Unsetenv("COMPOZY_HOME"); err != nil {
		t.Fatalf("os.Unsetenv(COMPOZY_HOME) error = %v", err)
	}
	stdout, _, err := executeRootCommand(t, deps, "config", "validate", "--repair-env", "-o", "json")
	if err != nil {
		t.Fatalf("config validate --repair-env from nested cwd error = %v", err)
	}
	var cwdRecord configValidateRecord
	if err := json.Unmarshal([]byte(stdout), &cwdRecord); err != nil {
		t.Fatalf("json.Unmarshal(config validate cwd) error = %v", err)
	}
	if cwdRecord.ConfigFile != filepath.Join(dotEnvHome, compozyconfig.ConfigName) {
		t.Fatalf("ConfigFile = %q, want dotenv home", cwdRecord.ConfigFile)
	}
	if cwdRecord.WorkspaceRoot != workspaceRoot {
		t.Fatalf("WorkspaceRoot = %q, want %q", cwdRecord.WorkspaceRoot, workspaceRoot)
	}

	stdout, _, err = executeRootCommand(t, deps, "config", "validate", "--workspace", workspaceRoot, "-o", "json")
	if err != nil {
		t.Fatalf("config validate --workspace error = %v", err)
	}
	var workspaceRecord configValidateRecord
	if err := json.Unmarshal([]byte(stdout), &workspaceRecord); err != nil {
		t.Fatalf("json.Unmarshal(config validate workspace) error = %v", err)
	}
	if workspaceRecord.ConfigFile != filepath.Join(dotEnvHome, compozyconfig.ConfigName) {
		t.Fatalf("workspace ConfigFile = %q, want dotenv home", workspaceRecord.ConfigFile)
	}

	if err := os.Setenv("COMPOZY_HOME", processHome); err != nil {
		t.Fatalf("os.Setenv(COMPOZY_HOME) error = %v", err)
	}
	stdout, _, err = executeRootCommand(t, deps, "config", "validate", "--workspace", workspaceRoot, "-o", "json")
	if err != nil {
		t.Fatalf("config validate process COMPOZY_HOME error = %v", err)
	}
	var processRecord configValidateRecord
	if err := json.Unmarshal([]byte(stdout), &processRecord); err != nil {
		t.Fatalf("json.Unmarshal(config validate process) error = %v", err)
	}
	if processRecord.ConfigFile != filepath.Join(processHome, compozyconfig.ConfigName) {
		t.Fatalf("process ConfigFile = %q, want process COMPOZY_HOME", processRecord.ConfigFile)
	}
}

func restoreEnvAfterTest(t *testing.T, key string) {
	t.Helper()

	value, existed := os.LookupEnv(key)
	t.Cleanup(func() {
		if existed {
			if err := os.Setenv(key, value); err != nil {
				t.Errorf("restore %s error = %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Errorf("unset %s error = %v", key, err)
		}
	})
}

func TestConfigValidateReportsInvalidConfigAsJSON(t *testing.T) {
	t.Run("Should emit an invalid JSON record for TOML parse errors", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(homePaths.ConfigFile), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(config dir) error = %v", err)
		}
		if err := os.WriteFile(homePaths.ConfigFile, []byte("[[mcp_servers\nname = \"oops\"\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(config) error = %v", err)
		}

		deps := newWorkspaceTestDeps(t, &stubClient{})
		deps.resolveHome = func() (compozyconfig.HomePaths, error) {
			return homePaths, nil
		}
		deps.resolveHomeForWorkspace = func(string) (compozyconfig.HomePaths, error) {
			return homePaths, nil
		}
		stdout, _, commandErr := executeRootCommand(t, deps, "config", "validate", "-o", "json")
		if commandErr == nil {
			t.Fatal("config validate error = nil, want invalid config failure")
		}

		var record configValidateRecord
		if decodeErr := json.Unmarshal([]byte(stdout), &record); decodeErr != nil {
			t.Fatalf(
				"json.Unmarshal(config validate invalid) error = %v; command error=%v; stdout=%s",
				decodeErr,
				commandErr,
				stdout,
			)
		}
		if record.Status != "invalid" {
			t.Fatalf("Status = %q, want invalid", record.Status)
		}
		if len(record.Errors) != 1 {
			t.Fatalf("Errors = %#v, want one config parse error", record.Errors)
		}
		got := record.Errors[0]
		if got.Code != "config.parse" {
			t.Fatalf("Errors[0].Code = %q, want config.parse", got.Code)
		}
		if got.File != homePaths.ConfigFile {
			t.Fatalf("Errors[0].File = %q, want %q", got.File, homePaths.ConfigFile)
		}
		if got.Line == 0 || strings.TrimSpace(got.Message) == "" {
			t.Fatalf("Errors[0] = %#v, want line and message", got)
		}
	})

	t.Run("Should include config path for validation errors", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(homePaths.ConfigFile), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(config dir) error = %v", err)
		}
		config := []byte("[defaults]\nagent = \"\"\nprovider = \"codex\"\n")
		if err := os.WriteFile(homePaths.ConfigFile, config, 0o600); err != nil {
			t.Fatalf("os.WriteFile(config) error = %v", err)
		}

		deps := newWorkspaceTestDeps(t, &stubClient{})
		deps.resolveHome = func() (compozyconfig.HomePaths, error) {
			return homePaths, nil
		}
		deps.resolveHomeForWorkspace = func(string) (compozyconfig.HomePaths, error) {
			return homePaths, nil
		}
		stdout, _, commandErr := executeRootCommand(t, deps, "config", "validate", "-o", "json")
		if commandErr == nil {
			t.Fatal("config validate error = nil, want validation failure")
		}

		var record configValidateRecord
		if decodeErr := json.Unmarshal([]byte(stdout), &record); decodeErr != nil {
			t.Fatalf(
				"json.Unmarshal(config validate validation) error = %v; command error=%v; stdout=%s",
				decodeErr,
				commandErr,
				stdout,
			)
		}
		if record.Status != "invalid" {
			t.Fatalf("Status = %q, want invalid", record.Status)
		}
		if len(record.Errors) != 1 {
			t.Fatalf("Errors = %#v, want one validation error", record.Errors)
		}
		got := record.Errors[0]
		if got.Code != "config.validation" {
			t.Fatalf("Errors[0].Code = %q, want config.validation", got.Code)
		}
		if got.Path != "defaults.agent" {
			t.Fatalf("Errors[0].Path = %q, want defaults.agent", got.Path)
		}
		if got.Message != "defaults.agent is required" {
			t.Fatalf("Errors[0].Message = %q, want defaults.agent is required", got.Message)
		}
	})
}

func TestConfigCommandsUseWorkspaceScopeAndValidateBeforeWriting(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{})
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	deps.getwd = func() (string, error) {
		return workspaceRoot, nil
	}

	if _, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"network.live.defaults.max_wakes",
		"12",
		"--scope",
		"workspace",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace config set error = %v", err)
	}

	workspaceConfig := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName)
	contents, err := os.ReadFile(workspaceConfig)
	if err != nil {
		t.Fatalf("ReadFile(workspace config) error = %v", err)
	}
	if !strings.Contains(string(contents), "max_wakes = 12") {
		t.Fatalf("workspace config = %s, want network Live max_wakes 12", string(contents))
	}
	loaded, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}
	if got, want := loaded.Network.Live.Defaults.MaxWakes, 12; got != want {
		t.Fatalf("Network.Live.Defaults.MaxWakes = %d, want %d", got, want)
	}
	if got, want := loaded.Network.Live.Limits.MaxWakes, compozyconfig.DefaultNetworkConfig().Live.Limits.MaxWakes; got != want {
		t.Fatalf("Network.Live.Limits.MaxWakes = %d, want default %d", got, want)
	}

	before := string(contents)
	_, _, err = executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"http.port",
		"70000",
		"--scope",
		"workspace",
	)
	if err == nil {
		t.Fatal("config set invalid http.port error = nil, want validation failure")
	}
	after, readErr := os.ReadFile(workspaceConfig)
	if readErr != nil {
		t.Fatalf("ReadFile(workspace config after invalid set) error = %v", readErr)
	}
	if string(after) != before {
		t.Fatalf("workspace config changed after invalid set\nbefore:\n%s\nafter:\n%s", before, string(after))
	}
}

func TestConfigSetSupportsWorktreeLifecyclePaths(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{})
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	deps.getwd = func() (string, error) { return workspaceRoot, nil }

	for _, mutation := range []struct {
		path  string
		value string
	}{
		{path: "worktrees.root", value: filepath.Join(t.TempDir(), "checkouts")},
		{path: "worktrees.run_branch_namespace", value: "qa/run/"},
		{path: "worktrees.copy_list", value: `[".env", ".secrets/*"]`},
		{path: "worktrees.setup_command", value: "bun install"},
		{path: "worktrees.setup_timeout", value: "45s"},
		{path: "worktrees.discovery_cache_ttl", value: "5s"},
	} {
		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			mutation.path,
			mutation.value,
			"--scope",
			"workspace",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("config set %s error = %v", mutation.path, err)
		}
	}

	loaded, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}
	if loaded.Worktrees.SetupCommand != "bun install" || loaded.Worktrees.SetupTimeout != 45*time.Second ||
		loaded.Worktrees.DiscoveryCacheTTL != 5*time.Second || len(loaded.Worktrees.CopyList) != 2 {
		t.Fatalf("Worktrees config = %#v, want typed lifecycle values", loaded.Worktrees)
	}
}

func TestConfigCommandsResolveContextBeforeSelectingOverlays(t *testing.T) {
	t.Parallel()

	t.Run("Should read the cwd workspace overlay from a nested directory", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		nested := filepath.Join(workspaceRoot, "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("MkdirAll(nested) error = %v", err)
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws-root", RootDir: workspaceRoot},
				}, nil
			},
		})
		deps.getwd = func() (string, error) { return nested, nil }

		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"network.live.defaults.max_wakes",
			"12",
			"--scope",
			"workspace",
		); err != nil {
			t.Fatalf("config set workspace overlay error = %v", err)
		}
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"get",
			"network.live.defaults.max_wakes",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("config get from nested cwd error = %v", err)
		}
		var record configValueRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(config get) error = %v", err)
		}
		if got := int(record.Value.(float64)); got != 12 {
			t.Fatalf("config get value = %#v, want 12", record.Value)
		}
	})

	t.Run("Should honor an explicit workspace when selecting the global home", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		cwd := t.TempDir()
		workspaceHome, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom(workspace) error = %v", err)
		}
		cwdHome, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom(cwd) error = %v", err)
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws-explicit", RootDir: workspaceRoot},
				}, nil
			},
		})
		deps.getwd = func() (string, error) { return cwd, nil }
		deps.resolveHomeForWorkspace = func(path string) (compozyconfig.HomePaths, error) {
			if path == workspaceRoot {
				return workspaceHome, nil
			}
			return cwdHome, nil
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"defaults.provider",
			"claude",
			"--workspace",
			workspaceRoot,
		); err != nil {
			t.Fatalf("config set global with explicit workspace error = %v", err)
		}
		if _, err := os.Stat(workspaceHome.ConfigFile); err != nil {
			t.Fatalf("Stat(workspace global config) error = %v", err)
		}
		if _, err := os.Stat(cwdHome.ConfigFile); !os.IsNotExist(err) {
			t.Fatalf("Stat(cwd global config) error = %v, want not exist", err)
		}
	})

	t.Run("Should not load the operator global file as an isolated runtime workspace overlay", func(t *testing.T) {
		t.Parallel()

		operatorHome := t.TempDir()
		operatorConfigDir := filepath.Join(operatorHome, compozyconfig.DirName)
		if err := os.MkdirAll(operatorConfigDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(operator config) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(operatorConfigDir, compozyconfig.ConfigName),
			[]byte("[gateway]\nprivate_port = 5252\n"),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(operator config) error = %v", err)
		}
		runtimeHome, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "runtime"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom(runtime) error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(runtimeHome); err != nil {
			t.Fatalf("EnsureHomeLayout(runtime) error = %v", err)
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws-operator-home", RootDir: operatorHome},
				}, nil
			},
		})
		deps.getwd = func() (string, error) { return operatorHome, nil }
		deps.resolveHomeForWorkspace = func(string) (compozyconfig.HomePaths, error) {
			return runtimeHome, nil
		}
		baseGetenv := deps.getenv
		deps.getenv = func(key string) string {
			if key == "HOME" {
				return operatorHome
			}
			return baseGetenv(key)
		}

		stdout, _, err := executeRootCommand(t, deps, "config", "list", "-o", "json")
		if err != nil {
			t.Fatalf("config list error = %v", err)
		}
		var record configListRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(config list) error = %v", err)
		}
		if record.WorkspaceRoot != operatorHome {
			t.Fatalf("WorkspaceRoot = %q, want %q", record.WorkspaceRoot, operatorHome)
		}
		if _, _, err := executeRootCommand(t, deps, "config", "validate", "-o", "json"); err != nil {
			t.Fatalf("config validate error = %v", err)
		}
	})
}

func TestConfigSetSupportsAgentAuthoredContextPaths(t *testing.T) {
	t.Parallel()

	t.Run("Should write valid agent Soul and Heartbeat overlays and reject invalid values", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		workspaceRoot := t.TempDir()
		deps.getwd = func() (string, error) {
			return workspaceRoot, nil
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"agents.soul.context_projection_bytes",
			"1536",
			"--scope",
			"workspace",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("config set agents.soul.context_projection_bytes error = %v", err)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"agents.heartbeat.default_interval",
			"25m",
			"--scope",
			"workspace",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("config set agents.heartbeat.default_interval error = %v", err)
		}

		loaded, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if got := loaded.Agents.Soul.ContextProjectionBytes; got != 1536 {
			t.Fatalf("Agents.Soul.ContextProjectionBytes = %d, want 1536", got)
		}
		if got := loaded.Agents.Heartbeat.DefaultInterval.String(); got != "25m0s" {
			t.Fatalf("Agents.Heartbeat.DefaultInterval = %q, want 25m0s", got)
		}

		workspaceConfig := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName)
		before, err := os.ReadFile(workspaceConfig)
		if err != nil {
			t.Fatalf("ReadFile(workspace config) error = %v", err)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"agents.soul.context_projection_bytes",
			"0",
			"--scope",
			"workspace",
		); err == nil {
			t.Fatal("config set invalid agents.soul.context_projection_bytes error = nil, want validation failure")
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"config",
			"set",
			"agents.heartbeat.default_interval",
			"0s",
			"--scope",
			"workspace",
		); err == nil {
			t.Fatal("config set invalid agents.heartbeat.default_interval error = nil, want validation failure")
		}
		after, err := os.ReadFile(workspaceConfig)
		if err != nil {
			t.Fatalf("ReadFile(workspace config after invalid set) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("workspace config changed after invalid agent config set\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
}

func TestConfigOutputRedactsMCPAndSandboxSecrets(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{})
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "remote"
command = "remote-mcp"
secret_env = { MCP_TOKEN = "env:MCP_TOKEN" }

[sandboxes.dev]
backend = "local"

	[sandboxes.dev.secret_env]
	API_TOKEN = "vault:sandbox/dev/api-token"

[providers.private]
command = "private-acp"
auth_mode = "native_cli"
auth_login_command = "private login --token raw-login-secret"
`)

	listOut, _, err := executeRootCommand(t, deps, "config", "list", "-o", "json")
	if err != nil {
		t.Fatalf("config list error = %v", err)
	}
	if strings.Contains(listOut, "env:MCP_TOKEN") ||
		strings.Contains(listOut, "vault:sandbox/dev/api-token") ||
		strings.Contains(listOut, "raw-login-secret") {
		t.Fatalf("config list leaked secret values:\n%s", listOut)
	}
	if !strings.Contains(listOut, compozyconfig.RedactedValue()) {
		t.Fatalf("config list = %s, want redacted placeholder", listOut)
	}

	getOut, _, err := executeRootCommand(t, deps, "config", "get", "mcp_servers[0].secret_env.MCP_TOKEN", "-o", "json")
	if err != nil {
		t.Fatalf("config get redacted MCP env error = %v", err)
	}
	var valueRecord configValueRecord
	if err := json.Unmarshal([]byte(getOut), &valueRecord); err != nil {
		t.Fatalf("json.Unmarshal(config get redacted) error = %v", err)
	}
	if valueRecord.Value != compozyconfig.RedactedValue() || !valueRecord.Redacted {
		t.Fatalf("redacted config value = %#v, want placeholder/redacted", valueRecord)
	}

	loginGetOut, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"get",
		"providers.private.auth_login_command",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config get write-only provider login command error = %v", err)
	}
	var loginRecord configValueRecord
	if err := json.Unmarshal([]byte(loginGetOut), &loginRecord); err != nil {
		t.Fatalf("json.Unmarshal(config get provider login) error = %v", err)
	}
	if loginRecord.Value != compozyconfig.RedactedValue() || !loginRecord.Redacted {
		t.Fatalf("provider login config value = %#v, want placeholder/redacted", loginRecord)
	}

	for _, command := range [][]string{{"config", "show", "-o", "json"}, {"config", "diff", "-o", "json"}} {
		output, _, commandErr := executeRootCommand(t, deps, command...)
		if commandErr != nil {
			t.Fatalf("%s error = %v", strings.Join(command, " "), commandErr)
		}
		if strings.Contains(output, "raw-login-secret") || strings.Contains(output, "private login --token") {
			t.Fatalf("%s leaked provider login command:\n%s", strings.Join(command, " "), output)
		}
	}

	setOut, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"providers.private.auth_login_command",
		"private login --token replacement-secret",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config set write-only provider login command error = %v", err)
	}
	var loginSetRecord configSetRecord
	if err := json.Unmarshal([]byte(setOut), &loginSetRecord); err != nil {
		t.Fatalf("json.Unmarshal(config set provider login) error = %v", err)
	}
	if loginSetRecord.Value != compozyconfig.RedactedValue() || !loginSetRecord.Redacted {
		t.Fatalf("provider login config set record = %#v, want placeholder/redacted", loginSetRecord)
	}
	if strings.Contains(setOut, "replacement-secret") || strings.Contains(setOut, "private login --token") {
		t.Fatalf("config set leaked provider login command:\n%s", setOut)
	}
}

func TestConfigRenderingAndMutationHelpers(t *testing.T) {
	t.Parallel()

	entries := []configEntry{
		{Path: "defaults.provider", Value: "claude"},
		{Path: "http.port", Value: int64(4141)},
		{Path: "telemetry.enabled", Value: true},
		{Path: "mcp_servers[0].env.API_TOKEN", Value: compozyconfig.RedactedValue(), Redacted: true},
		{Path: "providers.claude.models", Value: []string{"sonnet", "opus"}},
	}

	t.Run("Should flatten anonymous TOML fields into canonical parent paths", func(t *testing.T) {
		t.Parallel()

		cfg := compozyconfig.Config{Roles: compozyconfig.DefaultRolesConfig()}
		flattened := flattenConfigEntries(redactedConfigMap(&cfg))
		foundCanonical := false
		for _, entry := range flattened {
			switch entry.Path {
			case "roles.coordinator.enabled":
				foundCanonical = true
			case "roles.coordinator.roleconfig.enabled":
				t.Fatalf("config list exposed embedded Go field path %q", entry.Path)
			}
		}
		if !foundCanonical {
			t.Fatal("config list omitted canonical path roles.coordinator.enabled")
		}
	})

	t.Run("Should render show bundle outputs", func(t *testing.T) {
		t.Parallel()

		showBundle := configShowBundle(configShowRecord{
			Scope:    "global",
			Redacted: true,
			Config:   map[string]any{"defaults": map[string]any{"provider": "claude"}},
		}, entries)
		showHuman, err := showBundle.human()
		if err != nil {
			t.Fatalf("configShowBundle.human() error = %v", err)
		}
		if !strings.Contains(showHuman, "defaults.provider") || !strings.Contains(showHuman, "true") {
			t.Fatalf("config show human = %q, want rows", showHuman)
		}
		showToon, err := showBundle.toon()
		if err != nil {
			t.Fatalf("configShowBundle.toon() error = %v", err)
		}
		if !strings.Contains(showToon, "config[5]") {
			t.Fatalf("config show toon = %q, want toon rows", showToon)
		}
	})

	t.Run("Should render list bundle outputs", func(t *testing.T) {
		t.Parallel()

		listBundle := configListBundle(configListRecord{Scope: "global", Entries: entries})
		listHuman, err := listBundle.human()
		if err != nil {
			t.Fatalf("configListBundle.human() error = %v", err)
		}
		if !strings.Contains(listHuman, "mcp_servers[0].env.API_TOKEN") {
			t.Fatalf("config list human = %q, want redacted entry", listHuman)
		}
		listToon, err := listBundle.toon()
		if err != nil {
			t.Fatalf("configListBundle.toon() error = %v", err)
		}
		if !strings.Contains(listToon, "config[5]") {
			t.Fatalf("config list toon = %q, want toon rows", listToon)
		}
	})

	t.Run("Should render path bundle outputs", func(t *testing.T) {
		t.Parallel()

		pathBundle := configPathBundle(configPathRecord{
			HomeDir:              "/home/compozy",
			GlobalConfig:         "/home/compozy/config.toml",
			GlobalMCPJSON:        "/home/compozy/mcp.json",
			Scope:                "workspace",
			WorkspaceRoot:        "/workspace/project",
			WorkspaceConfig:      "/workspace/project/.compozy/config.toml",
			WorkspaceMCPJSON:     "/workspace/project/.compozy/mcp.json",
			Managed:              true,
			Manager:              "homebrew",
			SelectedConfigTarget: "/workspace/project/.compozy/config.toml",
		})
		pathHuman, err := pathBundle.human()
		if err != nil {
			t.Fatalf("configPathBundle.human() error = %v", err)
		}
		if !strings.Contains(pathHuman, "Workspace Config") || !strings.Contains(pathHuman, "homebrew") {
			t.Fatalf("config path human = %q, want workspace details", pathHuman)
		}
		pathToon, err := pathBundle.toon()
		if err != nil {
			t.Fatalf("configPathBundle.toon() error = %v", err)
		}
		if !strings.Contains(pathToon, "config_paths") || !strings.Contains(pathToon, "/workspace/project") {
			t.Fatalf("config path toon = %q, want workspace fields", pathToon)
		}
	})

	t.Run("Should classify mutation paths", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name        string
			path        string
			wantKind    configSetValueKind
			wantRedact  bool
			wantAllowed bool
		}{
			{
				name:        "Should allow provider command",
				path:        "providers.claude.command",
				wantKind:    configSetStringOrStringSlice,
				wantAllowed: true,
			},
			{
				name:        "Should allow provider model default",
				path:        "providers.codex.models.default",
				wantKind:    configSetString,
				wantAllowed: true,
			},
			{
				name:        "Should allow provider model discovery enabled",
				path:        "providers.codex.models.discovery.enabled",
				wantKind:    configSetBool,
				wantAllowed: true,
			},
			{
				name:        "Should redact sandbox env values",
				path:        "sandboxes.dev.env.API_TOKEN",
				wantKind:    configSetString,
				wantRedact:  true,
				wantAllowed: true,
			},
			{
				name:        "Should allow sandbox public ingress",
				path:        "sandboxes.dev.network.allow_public_ingress",
				wantKind:    configSetBool,
				wantAllowed: true,
			},
			{
				name:        "Should allow sandbox network allow list",
				path:        "sandboxes.dev.network.allow_list",
				wantKind:    configSetStringSlice,
				wantAllowed: true,
			},
			{
				name:        "Should allow sandbox Daytona image",
				path:        "sandboxes.dev.daytona.image",
				wantKind:    configSetString,
				wantAllowed: true,
			},
			{
				name:        "Should classify loop predicate cost as unsigned",
				path:        "loops.defaults.delivery.predicates.cost_limit",
				wantKind:    configSetUint64,
				wantAllowed: true,
			},
			{
				name:        "Should allow Marketplace catalog base URL",
				path:        "marketplace.catalog.base_url",
				wantKind:    configSetString,
				wantAllowed: true,
			},
			{
				name:        "Should allow Marketplace catalog TTL",
				path:        "marketplace.catalog.ttl",
				wantKind:    configSetDuration,
				wantAllowed: true,
			},
			{
				name:        "Should allow Marketplace catalog timeout",
				path:        "marketplace.catalog.timeout",
				wantKind:    configSetDuration,
				wantAllowed: true,
			},
			{
				name:        "Should allow extension unverified trust policy",
				path:        "extensions.trust.allow_unverified",
				wantKind:    configSetBool,
				wantAllowed: true,
			},
			{
				name:        "Should allow GitHub extension source toggle",
				path:        "extensions.sources.github.enabled",
				wantKind:    configSetBool,
				wantAllowed: true,
			},
			{
				name:        "Should allow GitHub extension source base URL",
				path:        "extensions.sources.github.base_url",
				wantKind:    configSetString,
				wantAllowed: true,
			},
			{
				name:        "Should allow direct Git extension source toggle",
				path:        "extensions.sources.git.enabled",
				wantKind:    configSetBool,
				wantAllowed: true,
			},
			{
				name:        "Should allow extension dev watch interval",
				path:        "extensions.dev.watch_interval",
				wantKind:    configSetDuration,
				wantAllowed: true,
			},
			{
				name:        "Should allow window manager string behavior",
				path:        "window_manager.new_window_policy",
				wantKind:    configSetString,
				wantAllowed: true,
			},
			{
				name:        "Should allow command palette personalization",
				path:        "cmd_palette.personalization",
				wantKind:    configSetBool,
				wantAllowed: true,
			},
			{
				name:        "Should allow command palette fallback targets",
				path:        "cmd_palette.fallback_targets",
				wantKind:    configSetStringSlice,
				wantAllowed: true,
			},
			{
				name:        "Should allow window manager boolean behavior",
				path:        "window_manager.focus_wrap",
				wantKind:    configSetBool,
				wantAllowed: true,
			},
			{
				name:        "Should allow window manager integer behavior",
				path:        "window_manager.snap.edge_band",
				wantKind:    configSetInt,
				wantAllowed: true,
			},
			{
				name:        "Should allow the window manager navigation stack limit",
				path:        "window_manager.nav_stack_limit",
				wantKind:    configSetInt,
				wantAllowed: true,
			},
			{
				name:        "Should allow the window manager closed entry limit",
				path:        "window_manager.closed_entry_limit",
				wantKind:    configSetInt,
				wantAllowed: true,
			},
			{
				name:        "Should allow window manager ratio arrays",
				path:        "window_manager.snap.repeat_ratios",
				wantKind:    configSetFloatSlice,
				wantAllowed: true,
			},
			{
				name:        "Should allow window manager shortcut entries",
				path:        "window_manager.shortcuts.window.focus.left",
				wantKind:    configSetStringOrStringSlice,
				wantAllowed: true,
			},
			{
				name:        "Should reject unknown window manager values",
				path:        "window_manager.snap.unknown",
				wantAllowed: false,
			},
			{
				name:        "Should allow full roles section",
				path:        "roles",
				wantKind:    configSetTable,
				wantAllowed: true,
			},
			{
				name:        "Should allow dream role model",
				path:        "roles.dream.model",
				wantKind:    configSetString,
				wantAllowed: true,
			},
			{
				name:        "Should allow memory controller role timeout",
				path:        "roles.memory_controller.timeout",
				wantKind:    configSetDuration,
				wantAllowed: true,
			},
			{name: "Should reject removed dream agent", path: "memory.dream.agent", wantAllowed: false},
			{name: "Should reject removed dream enabled", path: "memory.dream.enabled", wantAllowed: false},
			{name: "Should reject removed extractor model", path: "memory.extractor.model", wantAllowed: false},
			{name: "Should reject removed extractor enabled", path: "memory.extractor.enabled", wantAllowed: false},
			{name: "Should reject removed controller model", path: "memory.controller.llm.model", wantAllowed: false},
			{name: "Should reject unknown sandbox values", path: "sandboxes.dev.unknown.value", wantAllowed: false},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				_, kind, redacted, err := configMutationPath(tc.path)
				allowed := err == nil
				if allowed != tc.wantAllowed || (allowed && (kind != tc.wantKind || redacted != tc.wantRedact)) {
					t.Fatalf(
						"classifyConfigMutationPath(%q) = (%v, %v, %v), want (%v, %v, %v)",
						tc.path,
						kind,
						redacted,
						allowed,
						tc.wantKind,
						tc.wantRedact,
						tc.wantAllowed,
					)
				}
			})
		}
	})

	t.Run("Should reject removed extension paths with replacements", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			path        string
			replacement string
		}{
			{path: "extensions.marketplace.allow_unverified", replacement: "extensions.trust.allow_unverified"},
			{path: "extensions.marketplace.base_url", replacement: "extensions.sources.github.base_url"},
			{path: "extensions.marketplace.registry", replacement: "extensions.sources.github.enabled"},
			{path: "extensions.marketplace.enabled", replacement: "extensions.trust or extensions.sources"},
		}
		for _, tt := range tests {
			_, _, _, err := configMutationPath(tt.path)
			if err == nil {
				t.Fatalf("configMutationPath(%q) error = nil, want removed-path rejection", tt.path)
			}
			if !strings.Contains(err.Error(), tt.path) || !strings.Contains(err.Error(), tt.replacement) {
				t.Fatalf("configMutationPath(%q) error = %v, want replacement %q", tt.path, err, tt.replacement)
			}
		}
	})

	t.Run("Should parse string slice values", func(t *testing.T) {
		t.Parallel()

		values, err := parseStringSliceValue(`["alpha","beta"]`)
		if err != nil {
			t.Fatalf("parseStringSliceValue(json) error = %v", err)
		}
		if strings.Join(values, ",") != "alpha,beta" {
			t.Fatalf("parseStringSliceValue(json) = %#v, want alpha,beta", values)
		}
		values, err = parseStringSliceValue("alpha, beta, ,gamma")
		if err != nil {
			t.Fatalf("parseStringSliceValue(csv) error = %v", err)
		}
		if strings.Join(values, ",") != "alpha,beta,gamma" {
			t.Fatalf("parseStringSliceValue(csv) = %#v, want alpha,beta,gamma", values)
		}
		values, err = parseStringSliceValue(" ")
		if err != nil {
			t.Fatalf("parseStringSliceValue(empty) error = %v", err)
		}
		if len(values) != 0 {
			t.Fatalf("parseStringSliceValue(empty) = %#v, want empty", values)
		}
		if _, err := parseStringSliceValue(`["ok",1]`); err == nil {
			t.Fatal("parseStringSliceValue(invalid json) error = nil, want error")
		} else if !strings.Contains(err.Error(), "string") {
			t.Fatalf("parseStringSliceValue(invalid json) error = %v, want element type detail", err)
		}
	})

	t.Run("Should parse a complete roles object", func(t *testing.T) {
		t.Parallel()

		value, err := parseConfigSetValue(
			configSetTable,
			`{"auto_title":{"enabled":true,"fallback_chain":[{"provider":"codex","model":"gpt-5-mini"}]}}`,
		)
		if err != nil {
			t.Fatalf("parseConfigSetValue(roles object) error = %v", err)
		}
		table, ok := value.(map[string]any)
		if !ok || table["auto_title"] == nil {
			t.Fatalf("parseConfigSetValue(roles object) = %#v, want object", value)
		}
		if _, err := parseConfigSetValue(configSetTable, `[]`); err == nil {
			t.Fatal("parseConfigSetValue(array) error = nil, want object error")
		}
	})

	t.Run("Should enforce unsigned integer bounds", func(t *testing.T) {
		t.Parallel()

		value, err := parseConfigSetValue(configSetUint64, "18446744073709551615")
		if err != nil || value != uint64(18446744073709551615) {
			t.Fatalf("parseConfigSetValue(max uint64) = (%#v, %v), want max uint64", value, err)
		}
		for _, invalid := range []string{"-1", "18446744073709551616"} {
			if _, err := parseConfigSetValue(configSetUint64, invalid); err == nil {
				t.Fatalf("parseConfigSetValue(%q) error = nil, want bounds error", invalid)
			}
		}
	})

	t.Run("Should parse float slice values", func(t *testing.T) {
		t.Parallel()

		values, err := parseFloatSliceValue(`[0.5,0.666667,0.333333]`)
		if err != nil {
			t.Fatalf("parseFloatSliceValue(valid) error = %v", err)
		}
		if len(values) != 3 || values[0] != 0.5 || values[1] != 0.666667 || values[2] != 0.333333 {
			t.Fatalf("parseFloatSliceValue(valid) = %#v, want canonical ratios", values)
		}
		if _, err := parseFloatSliceValue(`[0.5,"invalid"]`); err == nil {
			t.Fatal("parseFloatSliceValue(invalid) error = nil, want error")
		}
	})
}

func TestConfigSetRedactsSensitiveMutationOutputAndManagedModeBlocksMutation(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{})
	if _, _, err := executeRootCommand(t, deps, "config", "set", "sandboxes.dev.backend", "local"); err != nil {
		t.Fatalf("config set sandbox backend error = %v", err)
	}
	out, _, err := executeRootCommand(
		t,
		deps,
		"config",
		"set",
		"sandboxes.dev.secret_env.API_TOKEN",
		"vault:sandbox/dev/api-token",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("config set sandbox env error = %v", err)
	}
	if strings.Contains(out, "vault:sandbox/dev/api-token") {
		t.Fatalf("config set leaked secret value:\n%s", out)
	}
	var setRecord configSetRecord
	if err := json.Unmarshal([]byte(out), &setRecord); err != nil {
		t.Fatalf("json.Unmarshal(secret config set) error = %v", err)
	}
	if setRecord.Value != compozyconfig.RedactedValue() || !setRecord.Redacted {
		t.Fatalf("secret config set record = %#v, want redacted placeholder", setRecord)
	}

	managedDeps := newWorkspaceTestDeps(t, &stubClient{})
	managedDeps.getenv = func(key string) string {
		if key == compozyupdate.ManagedEnvName {
			return "homebrew"
		}
		return ""
	}
	_, _, err = executeRootCommand(t, managedDeps, "config", "set", "defaults.provider", "claude")
	if err == nil {
		t.Fatal("managed config set error = nil, want managed mutation refusal")
	}
	if !strings.Contains(err.Error(), "managed by homebrew") {
		t.Fatalf("managed config set error = %v, want homebrew detail", err)
	}
}

func TestCompletionCommandEmitsShellCompletion(t *testing.T) {
	t.Parallel()

	out, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, &stubClient{}), "completion", "bash")
	if err != nil {
		t.Fatalf("completion bash error = %v", err)
	}
	for _, want := range []string{"bash completion V2 for compozy", "__start_compozy"} {
		if !strings.Contains(out, want) {
			t.Fatalf("completion output missing %q\n%s", want, out[:min(len(out), 400)])
		}
	}
}

func TestGatewayConfigSetPathsExcludeSurfaceAuthority(t *testing.T) {
	t.Parallel()

	t.Run("Should expose only gateway ceiling and operating bounds [UT-004]", func(t *testing.T) {
		t.Parallel()

		kinds := gatewayConfigSetPathKinds()
		want := map[string]configSetValueKind{
			"gateway.enabled":                    configSetBool,
			"gateway.private_port":               configSetInt,
			"gateway.public_port":                configSetInt,
			"gateway.pairing.ttl":                configSetDuration,
			"gateway.pairing.max_pending":        configSetInt,
			"gateway.stream_ticket.ttl":          configSetDuration,
			"gateway.auth.rate_limit.window":     configSetDuration,
			"gateway.auth.rate_limit.max_fails":  configSetInt,
			"gateway.verify.timeout":             configSetDuration,
			"gateway.verify.public_dns_resolver": configSetString,
		}
		if len(kinds) != len(want) {
			t.Fatalf("gateway config-set path count = %d, want %d", len(kinds), len(want))
		}
		for path, wantKind := range want {
			if gotKind, ok := kinds[path]; !ok || gotKind != wantKind {
				t.Fatalf("gateway config-set kind for %q = (%d, %t), want (%d, true)", path, gotKind, ok, wantKind)
			}
		}
		if _, ok := kinds["gateway.public_ui.enabled"]; ok {
			t.Fatal("gateway.public_ui.enabled is config-settable, want durable policy authority only")
		}
	})
}
