package agent

import (
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func TestResolveSessionSelectValue(t *testing.T) {
	t.Parallel()

	option := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"default[]",
		[]acp.SessionConfigSelectOption{
			{Value: "default[]", Name: "Auto"},
			{Value: "grok-4.5[effort=high,fast=true]", Name: "grok-4.5"},
			{Value: "gpt-5.6-sol[context=272k,reasoning=medium,fast=false]", Name: "gpt-5.6-sol"},
		},
	)

	tests := []struct {
		name      string
		requested string
		want      string
	}{
		{
			name:      "exact advertised value",
			requested: "grok-4.5[effort=high,fast=true]",
			want:      "grok-4.5[effort=high,fast=true]",
		},
		{
			name:      "friendly model name",
			requested: "grok-4.5",
			want:      "grok-4.5[effort=high,fast=true]",
		},
		{
			name:      "normalized friendly model name",
			requested: " GPT-5.6-SOL ",
			want:      "gpt-5.6-sol[context=272k,reasoning=medium,fast=false]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveSessionSelectValue(option, tt.requested, "model")
			if err != nil {
				t.Fatalf("resolveSessionSelectValue() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveSessionSelectValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSessionSelectValueReportsAdvertisedChoices(t *testing.T) {
	t.Parallel()

	option := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"default[]",
		[]acp.SessionConfigSelectOption{
			{Value: "default[]", Name: "Auto"},
			{Value: "grok-4.5[effort=high,fast=true]", Name: "grok-4.5"},
		},
	)

	_, err := resolveSessionSelectValue(option, "grok-4.5-fast-xhigh", "model")
	if err == nil {
		t.Fatal("expected unsupported model error")
	}
	for _, want := range []string{
		`model "grok-4.5-fast-xhigh" is not available`,
		`Auto (default[])`,
		`grok-4.5 (grok-4.5[effort=high,fast=true])`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestSessionSelectValuesSupportsGroupedOptions(t *testing.T) {
	t.Parallel()

	grouped := acp.SessionConfigSelectOptionsGrouped{
		{
			Group: "openai",
			Name:  "OpenAI",
			Options: []acp.SessionConfigSelectOption{
				{Value: "gpt-5.6-sol", Name: "GPT-5.6 Sol"},
			},
		},
		{
			Group: "anthropic",
			Name:  "Anthropic",
			Options: []acp.SessionConfigSelectOption{
				{Value: "claude-fable-5", Name: "Fable"},
			},
		},
	}
	option := &acp.SessionConfigOptionSelect{
		Id:           "model",
		Name:         "Model",
		CurrentValue: "gpt-5.6-sol",
		Type:         "select",
		Options: acp.SessionConfigSelectOptions{
			Grouped: &grouped,
		},
	}

	got := sessionSelectValues(option)
	if len(got) != 2 {
		t.Fatalf("sessionSelectValues() length = %d, want 2", len(got))
	}
	if got[0].Value != "gpt-5.6-sol" || got[1].Value != "claude-fable-5" {
		t.Fatalf("sessionSelectValues() = %#v", got)
	}
}

func TestSessionModeForModelAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		spec       Spec
		modelName  string
		accessMode string
		want       string
	}{
		{
			name:       "Fable canonical model forces auto under full access",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "claude-fable-5",
			accessMode: model.AccessModeFull,
			want:       "auto",
		},
		{
			name:       "Fable alias forces auto under default access",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "fable",
			accessMode: model.AccessModeDefault,
			want:       "auto",
		},
		{
			name:       "Fable one million suffix forces auto",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "claude-fable-5[1m]",
			accessMode: model.AccessModeFull,
			want:       "auto",
		},
		{
			name:       "Fable provider alias forces auto",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "anthropic/fable-5",
			accessMode: model.AccessModeFull,
			want:       "auto",
		},
		{
			name:       "other Claude models preserve bypass",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "opus",
			accessMode: model.AccessModeFull,
			want:       "bypassPermissions",
		},
		{
			name:       "Codex full access uses advertised full mode",
			spec:       Spec{ID: model.IDECodex, FullAccessModeID: "agent-full-access"},
			modelName:  "gpt-5.6-sol",
			accessMode: model.AccessModeFull,
			want:       "agent-full-access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sessionModeForModelAccess(tt.spec, tt.modelName, tt.accessMode); got != tt.want {
				t.Fatalf("sessionModeForModelAccess() = %q, want %q", got, tt.want)
			}
		})
	}
}

func testSessionSelectOption(
	id string,
	category acp.SessionConfigOptionCategory,
	current string,
	values []acp.SessionConfigSelectOption,
) *acp.SessionConfigOptionSelect {
	options := acp.SessionConfigSelectOptionsUngrouped(values)
	return &acp.SessionConfigOptionSelect{
		Id:           acp.SessionConfigId(id),
		Name:         id,
		Category:     &category,
		CurrentValue: acp.SessionConfigValueId(current),
		Type:         "select",
		Options: acp.SessionConfigSelectOptions{
			Ungrouped: &options,
		},
	}
}
