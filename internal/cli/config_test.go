package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	aghdaemon "github.com/compozy/agh/internal/daemon"
	aghupdate "github.com/compozy/agh/internal/update"
)

func TestConfigCommandsMutateValidateAndInspectTempHome(t *testing.T) {
	t.Parallel()
	t.Run("Should mutate validate and inspect a temporary home", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
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

		cfg, err := aghconfig.LoadGlobalConfig(homePaths)
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
			pathRecord.GlobalMCPJSON != filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName) {
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
			"alt+KeyH",
		); err != nil {
			t.Fatalf("config set window manager shortcut error = %v", err)
		}
		configured, err := aghconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(window manager) error = %v", err)
		}
		if got := configured.WindowManager.Snap.RepeatRatios; len(got) != 3 || got[1] != 0.75 {
			t.Fatalf("WindowManager.Snap.RepeatRatios = %#v, want [0.5 0.75 0.25]", got)
		}
		if got := configured.WindowManager.Shortcuts["window.focus.left"]; got != "alt+KeyH" {
			t.Fatalf("WindowManager.Shortcuts[window.focus.left] = %q, want alt+KeyH", got)
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
		configured, err = aghconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig(roles) error = %v", err)
		}
		if got := configured.Roles.MemoryController; got.TopK != 7 || got.MaxTokensOut != 512 {
			t.Fatalf("Roles.MemoryController = %#v, want top_k=7 max_tokens_out=512", got)
		}
	})
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
			name:          "Should apply the extension side-load policy live",
			path:          "extensions.marketplace.allow_unverified",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{})
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

		deps := newTestDeps(t, &stubClient{})
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
		workspaceConfig := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName)
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

	deps := newTestDeps(t, client)
	deps.readDaemonInfo = func(string) (aghdaemon.Info, error) {
		return aghdaemon.Info{PID: 42, Port: 2123, StartedAt: fixedTestNow}, nil
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

	deps := newTestDeps(t, client)
	deps.readDaemonInfo = func(string) (aghdaemon.Info, error) {
		return aghdaemon.Info{PID: 42, Port: 2123, StartedAt: fixedTestNow}, nil
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

func TestConfigSetRejectsLegacyEnvironmentMutationPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "Should reject legacy defaults environment", path: "defaults.environment"},
		{name: "Should reject legacy environment profile", path: "environments.dev.backend"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{})
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

	deps := newTestDeps(t, &stubClient{})
	workspaceRoot := t.TempDir()
	dotenvPath := filepath.Join(workspaceRoot, ".env")
	before := "AGH_TASK09_API_KEY=very-secret\u200b-token OTHER=value\n"
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
	if record.DotEnv.Status != aghconfig.DotEnvStatusRepaired || !record.DotEnv.Repaired {
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
		"AGH_TASK09_API_KEY=very-secret-token",
		"OTHER=value",
	} {
		if !strings.Contains(string(after), want) {
			t.Fatalf("repaired .env missing %q:\n%s", want, string(after))
		}
	}
}

func TestConfigValidateUsesWorkspaceDotEnvForHomeResolution(t *testing.T) {
	workspaceRoot := t.TempDir()
	dotEnvHome := filepath.Join(t.TempDir(), "dotenv-home")
	processHome := filepath.Join(t.TempDir(), "process-home")
	dotenvPath := filepath.Join(workspaceRoot, ".env")
	if err := os.WriteFile(dotenvPath, []byte("AGH_HOME="+dotEnvHome+"\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(.env) error = %v", err)
	}

	deps := newTestDeps(t, &stubClient{})
	deps.getwd = func() (string, error) {
		return workspaceRoot, nil
	}
	deps.resolveHomeForWorkspace = aghconfig.ResolveHomePathsForWorkspace

	restoreEnvAfterTest(t, "AGH_HOME")
	if err := os.Unsetenv("AGH_HOME"); err != nil {
		t.Fatalf("os.Unsetenv(AGH_HOME) error = %v", err)
	}
	stdout, _, err := executeRootCommand(t, deps, "config", "validate", "-o", "json")
	if err != nil {
		t.Fatalf("config validate error = %v", err)
	}
	var cwdRecord configValidateRecord
	if err := json.Unmarshal([]byte(stdout), &cwdRecord); err != nil {
		t.Fatalf("json.Unmarshal(config validate cwd) error = %v", err)
	}
	if cwdRecord.ConfigFile != filepath.Join(dotEnvHome, aghconfig.ConfigName) {
		t.Fatalf("ConfigFile = %q, want dotenv home", cwdRecord.ConfigFile)
	}

	stdout, _, err = executeRootCommand(t, deps, "config", "validate", "--workspace", workspaceRoot, "-o", "json")
	if err != nil {
		t.Fatalf("config validate --workspace error = %v", err)
	}
	var workspaceRecord configValidateRecord
	if err := json.Unmarshal([]byte(stdout), &workspaceRecord); err != nil {
		t.Fatalf("json.Unmarshal(config validate workspace) error = %v", err)
	}
	if workspaceRecord.ConfigFile != filepath.Join(dotEnvHome, aghconfig.ConfigName) {
		t.Fatalf("workspace ConfigFile = %q, want dotenv home", workspaceRecord.ConfigFile)
	}

	if err := os.Setenv("AGH_HOME", processHome); err != nil {
		t.Fatalf("os.Setenv(AGH_HOME) error = %v", err)
	}
	stdout, _, err = executeRootCommand(t, deps, "config", "validate", "--workspace", workspaceRoot, "-o", "json")
	if err != nil {
		t.Fatalf("config validate process AGH_HOME error = %v", err)
	}
	var processRecord configValidateRecord
	if err := json.Unmarshal([]byte(stdout), &processRecord); err != nil {
		t.Fatalf("json.Unmarshal(config validate process) error = %v", err)
	}
	if processRecord.ConfigFile != filepath.Join(processHome, aghconfig.ConfigName) {
		t.Fatalf("process ConfigFile = %q, want process AGH_HOME", processRecord.ConfigFile)
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

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(homePaths.ConfigFile), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(config dir) error = %v", err)
		}
		if err := os.WriteFile(homePaths.ConfigFile, []byte("[[mcp_servers\nname = \"oops\"\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(config) error = %v", err)
		}

		deps := newTestDeps(t, &stubClient{})
		deps.resolveHome = func() (aghconfig.HomePaths, error) {
			return homePaths, nil
		}
		deps.resolveHomeForWorkspace = func(string) (aghconfig.HomePaths, error) {
			return homePaths, nil
		}
		stdout, _, err := executeRootCommand(t, deps, "config", "validate", "-o", "json")
		if err == nil {
			t.Fatal("config validate error = nil, want invalid config failure")
		}

		var record configValidateRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(config validate invalid) error = %v; stdout=%s", err, stdout)
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

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
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

		deps := newTestDeps(t, &stubClient{})
		deps.resolveHome = func() (aghconfig.HomePaths, error) {
			return homePaths, nil
		}
		deps.resolveHomeForWorkspace = func(string) (aghconfig.HomePaths, error) {
			return homePaths, nil
		}
		stdout, _, err := executeRootCommand(t, deps, "config", "validate", "-o", "json")
		if err == nil {
			t.Fatal("config validate error = nil, want validation failure")
		}

		var record configValidateRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(config validate validation) error = %v; stdout=%s", err, stdout)
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

	deps := newTestDeps(t, &stubClient{})
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

	workspaceConfig := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName)
	contents, err := os.ReadFile(workspaceConfig)
	if err != nil {
		t.Fatalf("ReadFile(workspace config) error = %v", err)
	}
	if !strings.Contains(string(contents), "max_wakes = 12") {
		t.Fatalf("workspace config = %s, want network Live max_wakes 12", string(contents))
	}
	loaded, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}
	if got, want := loaded.Network.Live.Defaults.MaxWakes, 12; got != want {
		t.Fatalf("Network.Live.Defaults.MaxWakes = %d, want %d", got, want)
	}
	if got, want := loaded.Network.Live.Limits.MaxWakes, aghconfig.DefaultNetworkConfig().Live.Limits.MaxWakes; got != want {
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

func TestConfigSetSupportsAgentAuthoredContextPaths(t *testing.T) {
	t.Parallel()

	t.Run("Should write valid agent Soul and Heartbeat overlays and reject invalid values", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
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

		loaded, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if got := loaded.Agents.Soul.ContextProjectionBytes; got != 1536 {
			t.Fatalf("Agents.Soul.ContextProjectionBytes = %d, want 1536", got)
		}
		if got := loaded.Agents.Heartbeat.DefaultInterval.String(); got != "25m0s" {
			t.Fatalf("Agents.Heartbeat.DefaultInterval = %q, want 25m0s", got)
		}

		workspaceConfig := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.ConfigName)
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

	deps := newTestDeps(t, &stubClient{})
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
`)

	listOut, _, err := executeRootCommand(t, deps, "config", "list", "-o", "json")
	if err != nil {
		t.Fatalf("config list error = %v", err)
	}
	if strings.Contains(listOut, "env:MCP_TOKEN") ||
		strings.Contains(listOut, "vault:sandbox/dev/api-token") {
		t.Fatalf("config list leaked secret values:\n%s", listOut)
	}
	if !strings.Contains(listOut, aghconfig.RedactedValue()) {
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
	if valueRecord.Value != aghconfig.RedactedValue() || !valueRecord.Redacted {
		t.Fatalf("redacted config value = %#v, want placeholder/redacted", valueRecord)
	}
}

func TestConfigRenderingAndMutationHelpers(t *testing.T) {
	t.Parallel()

	entries := []configEntry{
		{Path: "defaults.provider", Value: "claude"},
		{Path: "http.port", Value: int64(4141)},
		{Path: "telemetry.enabled", Value: true},
		{Path: "mcp_servers[0].env.API_TOKEN", Value: aghconfig.RedactedValue(), Redacted: true},
		{Path: "providers.claude.models", Value: []string{"sonnet", "opus"}},
	}

	t.Run("Should flatten anonymous TOML fields into canonical parent paths", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.Config{Roles: aghconfig.DefaultRolesConfig()}
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
			HomeDir:              "/home/agh",
			GlobalConfig:         "/home/agh/config.toml",
			GlobalMCPJSON:        "/home/agh/mcp.json",
			Scope:                "workspace",
			WorkspaceRoot:        "/workspace/project",
			WorkspaceConfig:      "/workspace/project/.agh/config.toml",
			WorkspaceMCPJSON:     "/workspace/project/.agh/mcp.json",
			Managed:              true,
			Manager:              "homebrew",
			SelectedConfigTarget: "/workspace/project/.agh/config.toml",
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
				wantKind:    configSetString,
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
				name:        "Should allow window manager string behavior",
				path:        "window_manager.new_window_policy",
				wantKind:    configSetString,
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
				name:        "Should allow window manager ratio arrays",
				path:        "window_manager.snap.repeat_ratios",
				wantKind:    configSetFloatSlice,
				wantAllowed: true,
			},
			{
				name:        "Should allow window manager shortcut entries",
				path:        "window_manager.shortcuts.window.focus.left",
				wantKind:    configSetString,
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

	deps := newTestDeps(t, &stubClient{})
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
	if setRecord.Value != aghconfig.RedactedValue() || !setRecord.Redacted {
		t.Fatalf("secret config set record = %#v, want redacted placeholder", setRecord)
	}

	managedDeps := newTestDeps(t, &stubClient{})
	managedDeps.getenv = func(key string) string {
		if key == aghupdate.ManagedEnvName {
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

	out, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), "completion", "bash")
	if err != nil {
		t.Fatalf("completion bash error = %v", err)
	}
	for _, want := range []string{"bash completion V2 for agh", "__start_agh"} {
		if !strings.Contains(out, want) {
			t.Fatalf("completion output missing %q\n%s", want, out[:min(len(out), 400)])
		}
	}
}
