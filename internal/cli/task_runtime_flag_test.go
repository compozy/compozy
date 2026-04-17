package cli

import (
	"strings"
	"testing"
)

func TestParseTaskRuntimeRule(t *testing.T) {
	t.Parallel()

	t.Run("Should parse a type-based rule", func(t *testing.T) {
		t.Parallel()

		rule, err := parseTaskRuntimeRule("type=frontend,ide=codex,model=gpt-5.4,reasoning-effort=xhigh")
		if err != nil {
			t.Fatalf("parseTaskRuntimeRule() error = %v", err)
		}
		if rule.Type == nil || *rule.Type != "frontend" {
			t.Fatalf("unexpected type selector: %#v", rule.Type)
		}
		if rule.IDE == nil || *rule.IDE != "codex" {
			t.Fatalf("unexpected ide override: %#v", rule.IDE)
		}
		if rule.Model == nil || *rule.Model != "gpt-5.4" {
			t.Fatalf("unexpected model override: %#v", rule.Model)
		}
		if rule.ReasoningEffort == nil || *rule.ReasoningEffort != "xhigh" {
			t.Fatalf("unexpected reasoning override: %#v", rule.ReasoningEffort)
		}
	})

	t.Run("Should parse quoted CSV values", func(t *testing.T) {
		t.Parallel()

		rule, err := parseTaskRuntimeRule(`id=task_01,"model=gpt-5.4,preview"`)
		if err != nil {
			t.Fatalf("parseTaskRuntimeRule() error = %v", err)
		}
		if rule.ID == nil || *rule.ID != "task_01" {
			t.Fatalf("unexpected id selector: %#v", rule.ID)
		}
		if rule.Model == nil || *rule.Model != "gpt-5.4,preview" {
			t.Fatalf("unexpected model override: %#v", rule.Model)
		}
	})

	for _, tc := range []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "rejects missing selector",
			input:   "model=gpt-5.4",
			wantErr: "selector id=... or type=... is required",
		},
		{
			name:    "rejects both selectors",
			input:   "id=task_01,type=frontend,model=gpt-5.4",
			wantErr: "id and type cannot be combined",
		},
		{
			name:    "rejects missing overrides",
			input:   "type=frontend",
			wantErr: "must define at least one of ide, model, or reasoning-effort",
		},
		{
			name:    "rejects unknown keys",
			input:   "type=frontend,provider=codex",
			wantErr: `unknown key "provider"`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseTaskRuntimeRule(tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error\nwant substring: %q\ngot: %v", tc.wantErr, err)
			}
		})
	}
}
