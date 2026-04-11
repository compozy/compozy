package agent

import (
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/modelprovider"
)

func TestActivateOverlayRegistersDeclarativeRuntimeSpec(t *testing.T) {
	restore, err := ActivateOverlay([]OverlayEntry{
		{
			Name:    "ext-adapter",
			Command: "mock-acp --serve",
			Metadata: map[string]string{
				"display_name":      "Mock ACP",
				"default_model":     "mock-model",
				"agent_name":        "codex",
				"supports_add_dirs": "true",
			},
		},
	})
	if err != nil {
		t.Fatalf("activate ACP overlay: %v", err)
	}
	defer restore()

	if err := ValidateRuntimeConfig(&model.RuntimeConfig{
		Mode:                   model.ExecutionModePRDTasks,
		IDE:                    "ext-adapter",
		OutputFormat:           model.OutputFormatText,
		BatchSize:              1,
		MaxRetries:             0,
		RetryBackoffMultiplier: 1.5,
	}); err != nil {
		t.Fatalf("validate runtime config with overlay IDE: %v", err)
	}

	spec, err := lookupAgentSpec("ext-adapter")
	if err != nil {
		t.Fatalf("lookup overlay spec: %v", err)
	}
	if spec.Command != "mock-acp" {
		t.Fatalf("unexpected overlay command: %q", spec.Command)
	}
	if len(spec.FixedArgs) != 1 || spec.FixedArgs[0] != "--serve" {
		t.Fatalf("unexpected overlay fixed args: %#v", spec.FixedArgs)
	}
	if spec.SetupAgentName != "codex" {
		t.Fatalf("unexpected setup agent name: %q", spec.SetupAgentName)
	}
	if got := DisplayName("ext-adapter"); got != "Mock ACP" {
		t.Fatalf("unexpected overlay display name: %q", got)
	}
	if got, err := SetupAgentName("ext-adapter"); err != nil || got != "codex" {
		t.Fatalf("unexpected overlay setup agent mapping: got %q err=%v", got, err)
	}
	if got, err := ResolveRuntimeModel("ext-adapter", ""); err != nil || got != "mock-model" {
		t.Fatalf("unexpected overlay runtime model: got %q err=%v", got, err)
	}
}

func TestActivateOverlayParsesQuotedCommandAndMetadataArgs(t *testing.T) {
	restore, err := ActivateOverlay([]OverlayEntry{
		{
			Name:    "quoted-adapter",
			Command: "\"/opt/My Tool/bin/tool\" --serve",
			Metadata: map[string]string{
				"fixed_args": "\"two words\" --extra",
				"probe_args": "--probe \"quoted value\"",
			},
		},
	})
	if err != nil {
		t.Fatalf("activate quoted ACP overlay: %v", err)
	}
	defer restore()

	spec, err := lookupAgentSpec("quoted-adapter")
	if err != nil {
		t.Fatalf("lookup quoted overlay spec: %v", err)
	}
	if spec.Command != "/opt/My Tool/bin/tool" {
		t.Fatalf("unexpected quoted overlay command: %q", spec.Command)
	}
	if want := []string{"two words", "--extra"}; !reflect.DeepEqual(spec.FixedArgs, want) {
		t.Fatalf("unexpected quoted fixed args\nwant: %#v\ngot:  %#v", want, spec.FixedArgs)
	}
	if want := []string{"--probe", "quoted value"}; !reflect.DeepEqual(spec.ProbeArgs, want) {
		t.Fatalf("unexpected quoted probe args\nwant: %#v\ngot:  %#v", want, spec.ProbeArgs)
	}
}

func TestActivateOverlayPreservesBackslashesInsideDoubleQuotedArgs(t *testing.T) {
	restore, err := ActivateOverlay([]OverlayEntry{
		{
			Name:    "windows-adapter",
			Command: `"C:\Program Files\Tool\tool.exe" --serve`,
			Metadata: map[string]string{
				"fixed_args": `"C:\Program Files\Tool\config.json"`,
			},
		},
	})
	if err != nil {
		t.Fatalf("activate windows ACP overlay: %v", err)
	}
	defer restore()

	spec, err := lookupAgentSpec("windows-adapter")
	if err != nil {
		t.Fatalf("lookup windows overlay spec: %v", err)
	}
	if spec.Command != `C:\Program Files\Tool\tool.exe` {
		t.Fatalf("unexpected windows overlay command: %q", spec.Command)
	}
	if want := []string{`C:\Program Files\Tool\config.json`}; !reflect.DeepEqual(spec.FixedArgs, want) {
		t.Fatalf("unexpected windows fixed args\nwant: %#v\ngot:  %#v", want, spec.FixedArgs)
	}
}

func TestActivateOverlayRejectsUnterminatedQuotedArgs(t *testing.T) {
	_, err := ActivateOverlay([]OverlayEntry{{
		Name:    "broken-adapter",
		Command: "\"/opt/My Tool/bin/tool",
	}})
	if err == nil {
		t.Fatal("expected quoted overlay command to fail")
	}
	if !strings.Contains(err.Error(), "unterminated quote") {
		t.Fatalf("unexpected quoted overlay error: %v", err)
	}
}

func TestActivateOverlaySupportsTypedLauncherAndBootstrapFields(t *testing.T) {
	supportsAddDirs := true
	usesBootstrapModel := true

	restoreModels, err := modelprovider.ActivateOverlay([]modelprovider.OverlayEntry{{
		Name:   "ext-model",
		Target: "openai/gpt-5.4",
	}})
	if err != nil {
		t.Fatalf("activate model overlay: %v", err)
	}
	defer restoreModels()

	restore, err := ActivateOverlay([]OverlayEntry{
		{
			Name:               "typed-adapter",
			Command:            "echo",
			DisplayName:        "Typed ACP",
			DefaultModel:       "ext-model",
			SetupAgentName:     "codex",
			SupportsAddDirs:    &supportsAddDirs,
			UsesBootstrapModel: &usesBootstrapModel,
			FixedArgs:          []string{"serve"},
			ProbeArgs:          []string{"--probe"},
			EnvVars: map[string]string{
				"MOCK_ENV": "enabled",
			},
			Fallbacks: []Launcher{{
				Command:   "npx",
				FixedArgs: []string{"-y", "mock-acp"},
			}},
			Bootstrap: OverlayBootstrap{
				ModelFlag:             "--model",
				ReasoningEffortFlag:   "--reasoning",
				AddDirFlag:            "--add-dir",
				DefaultAccessModeArgs: []string{"--sandbox"},
				FullAccessModeArgs:    []string{"--danger"},
			},
		},
	})
	if err != nil {
		t.Fatalf("activate typed ACP overlay: %v", err)
	}
	defer restore()

	spec, err := lookupAgentSpec("typed-adapter")
	if err != nil {
		t.Fatalf("lookup typed overlay spec: %v", err)
	}
	if spec.DisplayName != "Typed ACP" {
		t.Fatalf("spec.DisplayName = %q, want %q", spec.DisplayName, "Typed ACP")
	}
	if spec.Command != "echo" {
		t.Fatalf("spec.Command = %q, want %q", spec.Command, "echo")
	}
	if want := []string{"serve"}; !reflect.DeepEqual(spec.FixedArgs, want) {
		t.Fatalf("spec.FixedArgs = %#v, want %#v", spec.FixedArgs, want)
	}
	if want := []string{"--probe"}; !reflect.DeepEqual(spec.ProbeArgs, want) {
		t.Fatalf("spec.ProbeArgs = %#v, want %#v", spec.ProbeArgs, want)
	}
	if !spec.SupportsAddDirs || !spec.UsesBootstrapModel {
		t.Fatalf("unexpected support flags: %#v", spec)
	}
	if spec.EnvVars["MOCK_ENV"] != "enabled" {
		t.Fatalf("spec.EnvVars = %#v, want MOCK_ENV to propagate", spec.EnvVars)
	}
	if len(spec.Fallbacks) != 1 || spec.Fallbacks[0].Command != "npx" {
		t.Fatalf("spec.Fallbacks = %#v, want typed fallback launcher", spec.Fallbacks)
	}
	if got, err := ResolveRuntimeModel("typed-adapter", ""); err != nil || got != "openai/gpt-5.4" {
		t.Fatalf("ResolveRuntimeModel() = %q err=%v, want %q", got, err, "openai/gpt-5.4")
	}
	command := BuildShellCommandString("typed-adapter", "ext-model", []string{"../docs"}, "high", model.AccessModeFull)
	for _, snippet := range []string{"echo", "serve", "--model", "openai/gpt-5.4", "--reasoning", "high", "--add-dir", "../docs", "--danger"} {
		if !strings.Contains(command, snippet) {
			t.Fatalf("BuildShellCommandString() = %q, want to contain %q", command, snippet)
		}
	}
}
