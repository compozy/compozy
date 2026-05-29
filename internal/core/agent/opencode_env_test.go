package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func TestSanitizeInheritedLaunchEnvironmentForOpenCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec Spec
		env  map[string]string
		want map[string]string
	}{
		{
			name: "restore source config dir and drop Orca vars",
			spec: Spec{SetupAgentName: "opencode"},
			env: map[string]string{
				openCodeConfigDirEnv:           "/Users/test/Library/Application Support/orca/opencode-hooks/missing",
				orcaOpenCodeConfigDirEnv:       "/Users/test/Library/Application Support/orca/opencode-hooks/missing",
				orcaOpenCodeSourceConfigDirEnv: "/Users/test/.config/opencode",
				"KEEP":                         "value",
			},
			want: map[string]string{
				openCodeConfigDirEnv: "/Users/test/.config/opencode",
				"KEEP":               "value",
			},
		},
		{
			name: "drop Orca overlay config when no source exists",
			spec: Spec{SetupAgentName: "opencode"},
			env: map[string]string{
				openCodeConfigDirEnv:     "/Users/test/Library/Application Support/orca/opencode-hooks/missing",
				orcaOpenCodeConfigDirEnv: "/Users/test/Library/Application Support/orca/opencode-hooks/missing",
				"KEEP":                   "value",
			},
			want: map[string]string{
				"KEEP": "value",
			},
		},
		{
			name: "drop stale Orca config dir even without Orca helper vars",
			spec: Spec{ID: model.IDEOpenCode},
			env: map[string]string{
				openCodeConfigDirEnv: "/Users/test/Library/Application Support/orca/opencode-config-overlays/overlay",
				"KEEP":               "value",
			},
			want: map[string]string{
				"KEEP": "value",
			},
		},
		{
			name: "preserve explicit custom config dir when Orca vars are stale",
			spec: Spec{SetupAgentName: "opencode"},
			env: map[string]string{
				openCodeConfigDirEnv:           "/Users/test/.config/opencode-custom",
				orcaOpenCodeConfigDirEnv:       "/Users/test/Library/Application Support/orca/opencode-hooks/missing",
				orcaOpenCodeSourceConfigDirEnv: "/Users/test/.config/opencode-source",
				"KEEP":                         "value",
			},
			want: map[string]string{
				openCodeConfigDirEnv: "/Users/test/.config/opencode-custom",
				"KEEP":               "value",
			},
		},
		{
			name: "leave non OpenCode runtimes unchanged",
			spec: Spec{ID: model.IDECodex, SetupAgentName: "codex"},
			env: map[string]string{
				openCodeConfigDirEnv:     "/Users/test/custom-opencode",
				orcaOpenCodeConfigDirEnv: "/Users/test/Library/Application Support/orca/opencode-hooks/missing",
			},
			want: map[string]string{
				openCodeConfigDirEnv:     "/Users/test/custom-opencode",
				orcaOpenCodeConfigDirEnv: "/Users/test/Library/Application Support/orca/opencode-hooks/missing",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := cloneStringMap(tc.env)
			sanitizeInheritedLaunchEnvironment(tc.spec, env)
			if !reflect.DeepEqual(env, tc.want) {
				t.Fatalf("sanitizeInheritedLaunchEnvironment() = %#v, want %#v", env, tc.want)
			}
		})
	}
}

func TestBuildLaunchEnvironmentAppliesExplicitOverridesAfterSanitizingOpenCode(t *testing.T) {
	t.Setenv(openCodeConfigDirEnv, "/Users/test/Library/Application Support/orca/opencode-hooks/missing")
	t.Setenv(orcaOpenCodeConfigDirEnv, "/Users/test/Library/Application Support/orca/opencode-hooks/missing")
	t.Setenv(orcaOpenCodeSourceConfigDirEnv, "/Users/test/.config/opencode")

	env := parseEnvironmentAssignments(buildLaunchEnvironment(
		Spec{SetupAgentName: "opencode", EnvVars: map[string]string{"SPEC_ONLY": "1"}},
		map[string]string{openCodeConfigDirEnv: "/Users/test/custom-opencode"},
	))

	if got := env[openCodeConfigDirEnv]; got != "/Users/test/custom-opencode" {
		t.Fatalf("%s = %q, want %q", openCodeConfigDirEnv, got, "/Users/test/custom-opencode")
	}
	if got := env["SPEC_ONLY"]; got != "1" {
		t.Fatalf("SPEC_ONLY = %q, want %q", got, "1")
	}
	for _, key := range []string{orcaOpenCodeConfigDirEnv, orcaOpenCodeSourceConfigDirEnv} {
		if _, ok := env[key]; ok {
			t.Fatalf("expected %s to be removed from launch env, got %#v", key, env)
		}
	}
}

func TestEnsureAvailableSanitizesOrcaInjectedOpenCodeProbeEnvironment(t *testing.T) {
	overlayDir := "/Users/test/Library/Application Support/orca/opencode-hooks/missing"
	t.Setenv(openCodeConfigDirEnv, overlayDir)
	t.Setenv(orcaOpenCodeConfigDirEnv, overlayDir)
	t.Setenv(orcaOpenCodeSourceConfigDirEnv, "")

	specID := "test-open-code-probe-" + sanitizeTestName(t.Name())
	registerTestSpec(t, Spec{
		ID:             specID,
		DisplayName:    "Test OpenCode Probe",
		SetupAgentName: "opencode",
		DefaultModel:   "test-model",
		Command:        os.Args[0],
		ProbeArgs:      []string{"-test.run=TestProbeEnvHelperProcess", "--"},
		EnvVars: map[string]string{
			"GO_WANT_PROBE_ENV_HELPER": "1",
			"GO_EXPECT_ENV_ABSENT": strings.Join([]string{
				openCodeConfigDirEnv,
				orcaOpenCodeConfigDirEnv,
				orcaOpenCodeSourceConfigDirEnv,
			}, ","),
		},
	})

	if err := EnsureAvailable(context.Background(), &model.RuntimeConfig{IDE: specID}); err != nil {
		t.Fatalf("EnsureAvailable() error = %v", err)
	}
}

func TestClientCreateSessionRestoresSourceOpenCodeConfigDir(t *testing.T) {
	workingDir := t.TempDir()
	overlayDir := "/Users/test/Library/Application Support/orca/opencode-hooks/missing"
	sourceDir := "/Users/test/.config/opencode"
	t.Setenv(openCodeConfigDirEnv, overlayDir)
	t.Setenv(orcaOpenCodeConfigDirEnv, overlayDir)
	t.Setenv(orcaOpenCodeSourceConfigDirEnv, sourceDir)

	scenario := helperScenario{
		ExpectedCWD:        workingDir,
		ExpectedProcessCWD: workingDir,
		ExpectedPrompt:     "hello from compozy",
		ExpectedProcessEnv: map[string]string{
			openCodeConfigDirEnv: sourceDir,
		},
		UnexpectedProcessEnv: []string{
			orcaOpenCodeConfigDirEnv,
			orcaOpenCodeSourceConfigDirEnv,
		},
		StopReason: string(acp.StopReasonEndTurn),
	}

	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal helper scenario: %v", err)
	}

	specID := "test-open-code-session-" + sanitizeTestName(t.Name())
	registerTestSpec(t, Spec{
		ID:                 specID,
		DisplayName:        "Test OpenCode Session",
		SetupAgentName:     "opencode",
		DefaultModel:       "test-model",
		Command:            os.Args[0],
		UsesBootstrapModel: true,
		EnvVars: map[string]string{
			"GO_WANT_ACP_HELPER_PROCESS": "1",
			"GO_ACP_SCENARIO":            string(scenarioJSON),
		},
		BootstrapArgs: func(_, _ string, _ []string, _ string) []string {
			return []string{"-test.run=TestACPHelperProcess", "--"}
		},
	})

	client, err := NewClient(context.Background(), ClientConfig{
		IDE:             specID,
		Model:           "test-model",
		ReasoningEffort: "medium",
		ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: workingDir,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	_ = collectSessionUpdates(t, session)
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestProbeEnvHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_PROBE_ENV_HELPER") != "1" {
		return
	}
	if err := validateHelperEnvironment(nil, splitCSV(os.Getenv("GO_EXPECT_ENV_ABSENT"))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func validateHelperEnvironment(expected map[string]string, absent []string) error {
	for key, want := range expected {
		got, ok := os.LookupEnv(key)
		if !ok {
			return fmt.Errorf("missing environment variable %s", key)
		}
		if got != want {
			return fmt.Errorf("unexpected %s value %q, want %q", key, got, want)
		}
	}
	for _, key := range absent {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if got, ok := os.LookupEnv(key); ok {
			return fmt.Errorf("unexpected environment variable %s=%q", key, got)
		}
	}
	return nil
}

func parseEnvironmentAssignments(assignments []string) map[string]string {
	env := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		key, value, ok := strings.Cut(assignment, "=")
		if !ok {
			continue
		}
		env[key] = value
	}
	return env
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
