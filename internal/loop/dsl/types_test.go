package dsl_test

import (
	"strings"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
)

func TestDefinitionShouldNormalizeAndValidateHeader(t *testing.T) {
	t.Parallel()

	t.Run("Should apply zero-value defaults on normalize", func(t *testing.T) {
		t.Parallel()

		def := dsl.Definition{}
		def.Normalize()
		if def.APIVersion != dsl.APIVersion {
			t.Fatalf("APIVersion = %q, want %q", def.APIVersion, dsl.APIVersion)
		}
		if def.Kind != dsl.KindLoop {
			t.Fatalf("Kind = %q, want %q", def.Kind, dsl.KindLoop)
		}
		if def.Concurrency != dsl.ConcurrencyForbid {
			t.Fatalf("Concurrency = %q, want %q", def.Concurrency, dsl.ConcurrencyForbid)
		}
		if def.Inputs == nil || def.Start == nil || def.Graph.Nodes == nil || def.Graph.Edges == nil {
			t.Fatal("Normalize() left nil authoring collections")
		}
		if err := def.ValidateHeader(); err != nil {
			t.Fatalf("ValidateHeader() error = %v", err)
		}
	})

	t.Run("Should reject wrong apiVersion", func(t *testing.T) {
		t.Parallel()

		def := dsl.Definition{APIVersion: "agh.loop/v2", Kind: dsl.KindLoop}
		err := def.ValidateHeader()
		requireErrorContains(t, err, `apiVersion must be "agh.loop/v1"`)
	})

	t.Run("Should reject wrong kind", func(t *testing.T) {
		t.Parallel()

		def := dsl.Definition{APIVersion: dsl.APIVersion, Kind: "Workflow"}
		err := def.ValidateHeader()
		requireErrorContains(t, err, `kind must be "Loop"`)
	})
}

func TestEnumsShouldClassifyLoopKinds(t *testing.T) {
	t.Parallel()

	t.Run("Should classify reserved action kinds", func(t *testing.T) {
		t.Parallel()

		reserved := dsl.ReservedActionKinds()
		if len(reserved) != 4 {
			t.Fatalf("ReservedActionKinds() len = %d, want 4", len(reserved))
		}
		for _, kind := range []dsl.ActionKind{
			dsl.ActionRunAgent,
			dsl.ActionRunLoop,
			dsl.ActionTransform,
			dsl.ActionGoal,
		} {
			if !dsl.IsReservedActionKind(string(kind)) {
				t.Fatalf("IsReservedActionKind(%q) = false, want true", kind)
			}
		}
		if dsl.IsReservedActionKind("agh__tool_info") {
			t.Fatal("IsReservedActionKind(open ToolID) = true, want false")
		}
	})

	t.Run("Should classify known control kinds", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []dsl.ControlKind{
			dsl.ControlFanOut,
			dsl.ControlCollect,
			dsl.ControlBranch,
			dsl.ControlGate,
			dsl.ControlSubLoop,
		} {
			if !dsl.IsKnownControlKind(string(kind)) {
				t.Fatalf("IsKnownControlKind(%q) = false, want true", kind)
			}
		}
		if dsl.IsKnownControlKind("parallel") {
			t.Fatal("IsKnownControlKind(unknown) = true, want false")
		}
	})

	t.Run("Should classify known source kinds", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []dsl.SourceKind{
			dsl.SourceInput,
			dsl.SourceFileImport,
			dsl.SourceWatchSource,
			dsl.SourceWatchEvents,
		} {
			if !dsl.IsKnownSourceKind(string(kind)) {
				t.Fatalf("IsKnownSourceKind(%q) = false, want true", kind)
			}
		}
		if dsl.IsKnownSourceKind("mailbox") {
			t.Fatal("IsKnownSourceKind(unknown) = true, want false")
		}
	})
}

func TestNodeParamsShouldDecodePerKindSchemas(t *testing.T) {
	t.Parallel()

	t.Run("Should decode run-agent params", func(t *testing.T) {
		t.Parallel()

		runAgent := dsl.NodeParams{
			"agent":         "codex",
			"prompt":        "Ship it",
			"output_schema": map[string]any{"summary": "string"},
			"cwd":           "/repo",
			"model":         "gpt-5",
			"allowed_tools": []string{"agh__task_read"},
			"max_turns":     3,
		}
		var agentParams dsl.RunAgentParams
		if err := runAgent.Decode(&agentParams); err != nil {
			t.Fatalf("Decode(RunAgentParams) error = %v", err)
		}
		if agentParams.Agent != "codex" || agentParams.Prompt != "Ship it" {
			t.Fatalf("RunAgentParams = %#v, want decoded agent/prompt", agentParams)
		}
		if got := agentParams.OutputSchema["summary"]; got != "string" {
			t.Fatalf("OutputSchema[summary] = %#v, want string", got)
		}
		if len(agentParams.AllowedTools) != 1 || agentParams.AllowedTools[0] != "agh__task_read" {
			t.Fatalf("AllowedTools = %#v, want agh__task_read", agentParams.AllowedTools)
		}
	})

	t.Run("Should decode transform params", func(t *testing.T) {
		t.Parallel()

		transform := dsl.NodeParams{
			"map": map[string]any{
				"summary": map[string]any{"template": "{{ .nodes.agent.output.summary }}"},
			},
		}
		var transformParams dsl.TransformParams
		if err := transform.Decode(&transformParams); err != nil {
			t.Fatalf("Decode(TransformParams) error = %v", err)
		}
		if transformParams.Map["summary"].Template == "" {
			t.Fatalf("TransformParams.Map = %#v, want summary template", transformParams.Map)
		}
	})

	t.Run("Should decode goal params and continuous retry fields", func(t *testing.T) {
		t.Parallel()

		goal := dsl.NodeParams{
			"agent":        "codex",
			"objective":    "Make verification pass",
			"judge":        []any{map[string]any{"id": "verify", "type": "command", "check": "make verify"}},
			"max_turns":    12,
			"on_exhausted": "escalate",
			"output_schema": map[string]any{
				"status": map[string]any{"type": "string", "enum": []any{"complete", "blocked"}},
			},
			"session": map[string]any{"mode": "continuous"},
			"retry":   map[string]any{"max_attempts": 2, "on_failure": "fresh_session"},
		}
		var params struct {
			dsl.GoalParams `yaml:",inline"`
			Session        dsl.SessionSpec `yaml:"session"`
			Retry          dsl.RetrySpec   `yaml:"retry"`
		}
		if err := goal.Decode(&params); err != nil {
			t.Fatalf("Decode(GoalParams) error = %v", err)
		}
		if params.Agent != "codex" || params.Objective != "Make verification pass" {
			t.Fatalf("GoalParams = %#v, want decoded agent/objective", params)
		}
		if len(params.Judge) != 1 || params.Judge[0].Check != "make verify" {
			t.Fatalf("GoalParams.Judge = %#v, want command check", params.Judge)
		}
		if params.OutputSchema == nil {
			t.Fatal("GoalParams.OutputSchema = nil, want decoded schema")
		}

		if params.Session.Mode != "continuous" || params.Retry.MaxAttempts != 2 ||
			params.Retry.OnFailure != "fresh_session" {
			t.Fatalf("session/retry = %#v/%#v, want continuous fresh_session", params.Session, params.Retry)
		}
	})

	t.Run("Should derive fixed continuous goal handles", func(t *testing.T) {
		t.Parallel()

		implicit, err := dsl.DeriveGoalHandle("converge_fix", "", 0)
		if err != nil {
			t.Fatalf("DeriveGoalHandle(implicit) error = %v", err)
		}
		explicit, err := dsl.DeriveGoalHandle("converge_fix", "  primary  ", 1)
		if err != nil {
			t.Fatalf("DeriveGoalHandle(explicit) error = %v", err)
		}
		if len(implicit) != len("goal:")+64 || !strings.HasPrefix(implicit, "goal:") {
			t.Fatalf("implicit handle = %q, want fixed goal sha256 form", implicit)
		}
		if implicit == explicit {
			t.Fatalf("handles = %q, want item/label-specific identities", implicit)
		}
		repeated, err := dsl.DeriveGoalHandle("converge_fix", "primary", 1)
		if err != nil {
			t.Fatalf("DeriveGoalHandle(repeated) error = %v", err)
		}
		if repeated != explicit {
			t.Fatalf("trimmed handle = %q, want %q", repeated, explicit)
		}
	})
}

func TestParseSerializeShouldRejectInvalidDocuments(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		run      func() error
		contains string
	}{
		{
			name: "Should reject empty parse document",
			run: func() error {
				_, err := dsl.Parse([]byte(" \n\t"))
				return err
			},
			contains: "document is empty",
		},
		{
			name: "Should reject wrong apiVersion during parse",
			run: func() error {
				_, err := dsl.Parse([]byte("apiVersion: agh.loop/v2\nkind: Loop\n"))
				return err
			},
			contains: `apiVersion must be "agh.loop/v1"`,
		},
		{
			name: "Should reject invalid YAML during parse",
			run: func() error {
				_, err := dsl.Parse([]byte("apiVersion: agh.loop/v1\nkind: ["))
				return err
			},
			contains: "parse loop definition",
		},
		{
			name: "Should reject wrong kind during serialize",
			run: func() error {
				_, err := dsl.Serialize(dsl.Definition{APIVersion: dsl.APIVersion, Kind: "Workflow"})
				return err
			},
			contains: `kind must be "Loop"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireErrorContains(t, tc.run(), tc.contains)
		})
	}
}

func requireErrorContains(t *testing.T, err error, contains string) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want containing %q", contains)
	}
	if !strings.Contains(err.Error(), contains) {
		t.Fatalf("error = %v, want containing %q", err, contains)
	}
}
