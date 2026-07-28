package cli

import (
	"strings"
	"testing"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func TestParseLoopRuntimeFlagsShouldParseDefaultsAndRules(t *testing.T) {
	t.Parallel()

	t.Run("Should parse worker judge and bare model defaults", func(t *testing.T) {
		t.Parallel()

		got, err := parseLoopRuntimeFlags([]string{
			"worker=claude/opus@xhigh",
			"judge=codex/gpt-5.5",
		})
		if err != nil {
			t.Fatalf("parseLoopRuntimeFlags() error = %v", err)
		}
		if got.RuntimeDefaults == nil {
			t.Fatal("RuntimeDefaults = nil")
		}
		assertRuntimeFlagSpec(t, got.RuntimeDefaults.Worker, looppkg.RuntimeSpec{
			Provider: "claude", Model: "opus", Reasoning: "xhigh",
		})
		assertRuntimeFlagSpec(t, got.RuntimeDefaults.Judge, looppkg.RuntimeSpec{
			Provider: "codex", Model: "gpt-5.5",
		})

		bare, err := parseLoopRuntimeFlags([]string{"worker=opus"})
		if err != nil {
			t.Fatalf("parseLoopRuntimeFlags(bare) error = %v", err)
		}
		assertRuntimeFlagSpec(t, bare.RuntimeDefaults.Worker, looppkg.RuntimeSpec{Model: "opus"})
	})

	t.Run("Should parse id and type rules", func(t *testing.T) {
		t.Parallel()

		got, err := parseLoopRuntimeFlags([]string{
			"type=frontend:claude/opus",
			"id=task_03:codex/gpt-5.5-codex@high",
		})
		if err != nil {
			t.Fatalf("parseLoopRuntimeFlags() error = %v", err)
		}
		if len(got.RuntimeRules) != 2 {
			t.Fatalf("RuntimeRules = %#v, want two rules", got.RuntimeRules)
		}
		if got.RuntimeRules[0].Match.Type != "frontend" {
			t.Fatalf("RuntimeRules[0].Match = %#v, want type frontend", got.RuntimeRules[0].Match)
		}
		assertRuntimeFlagSpec(t, got.RuntimeRules[0].Runtime, looppkg.RuntimeSpec{
			Provider: "claude", Model: "opus",
		})
		if got.RuntimeRules[1].Match.ID != "task_03" {
			t.Fatalf("RuntimeRules[1].Match = %#v, want id task_03", got.RuntimeRules[1].Match)
		}
		assertRuntimeFlagSpec(t, got.RuntimeRules[1].Runtime, looppkg.RuntimeSpec{
			Provider: "codex", Model: "gpt-5.5-codex", Reasoning: "high",
		})
	})

	t.Run("Should preserve slash models and explicit skipped fields", func(t *testing.T) {
		t.Parallel()

		got, err := parseLoopRuntimeFlags([]string{
			"type=docs:-/gpt-5.5-mini",
			"worker=openrouter/anthropic/claude-opus-4-7@high",
			"judge=claude/-@high",
		})
		if err != nil {
			t.Fatalf("parseLoopRuntimeFlags() error = %v", err)
		}
		assertRuntimeFlagSpec(t, got.RuntimeRules[0].Runtime, looppkg.RuntimeSpec{Model: "gpt-5.5-mini"})
		assertRuntimeFlagSpec(t, got.RuntimeDefaults.Worker, looppkg.RuntimeSpec{
			Provider: "openrouter", Model: "anthropic/claude-opus-4-7", Reasoning: "high",
		})
		assertRuntimeFlagSpec(t, got.RuntimeDefaults.Judge, looppkg.RuntimeSpec{
			Provider: "claude", Reasoning: "high",
		})
	})

	t.Run("Should preserve repeated rule order", func(t *testing.T) {
		t.Parallel()

		got, err := parseLoopRuntimeFlags([]string{
			"type=frontend:claude/sonnet",
			"type=frontend:claude/opus",
		})
		if err != nil {
			t.Fatalf("parseLoopRuntimeFlags() error = %v", err)
		}
		if len(got.RuntimeRules) != 2 || got.RuntimeRules[0].Runtime.Model != "sonnet" ||
			got.RuntimeRules[1].Runtime.Model != "opus" {
			t.Fatalf("RuntimeRules = %#v, want original repeat order", got.RuntimeRules)
		}
		resolved, err := looppkg.ResolveItemRuntime(looppkg.RuntimeLayers{
			RunRules: got.RuntimeRules,
		}, looppkg.ItemRuntime{TaskType: "frontend"})
		if err != nil {
			t.Fatalf("ResolveItemRuntime() error = %v", err)
		}
		if resolved.Runtime.Model != "opus" || resolved.Source.Model != looppkg.RuntimeSourceRun {
			t.Fatalf("resolved runtime = %#v, want later run rule", resolved)
		}
	})
}

func TestParseLoopRuntimeFlagsShouldRejectInvalidGrammar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "Should reject a missing rule separator", value: "type=frontend", want: "rule requires match:runtime"},
		{name: "Should reject an empty rule runtime", value: "type=frontend:", want: "runtime expression is empty"},
		{name: "Should reject an unknown selector", value: "foo=claude/opus", want: "unknown selector"},
		{
			name: "Should reject a duplicate selector collision", value: "type=frontend,id=task_03:claude/opus",
			want: "rule match contains duplicate selectors",
		},
		{
			name: "Should reject a match value containing a colon", value: "type=front:end:claude/opus",
			want: "rule match must contain one selector value without ':'",
		},
		{name: "Should reject invalid reasoning", value: "worker=claude/opus@ultra", want: "invalid reasoning suffix"},
		{
			name: "Should reject implicit empty provider segments", value: "worker=/opus",
			want: "empty provider/model segments must use '-'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseLoopRuntimeFlags([]string{tc.value})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("parseLoopRuntimeFlags(%q) error = %v, want %q", tc.value, err, tc.want)
			}
			if !strings.Contains(err.Error(), "MIGRATION_GUIDE.md#per-task-runtime-selection") {
				t.Fatalf("parseLoopRuntimeFlags(%q) error = %v, want migration guide", tc.value, err)
			}
		})
	}
}

func assertRuntimeFlagSpec(t *testing.T, got looppkg.RuntimeSpec, want looppkg.RuntimeSpec) {
	t.Helper()
	if got.Provider != want.Provider || got.Model != want.Model || got.Reasoning != want.Reasoning {
		t.Fatalf("runtime spec = %#v, want %#v", got, want)
	}
}
