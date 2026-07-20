package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func TestConfigureSessionRetainsOMPRuntimeModelForAuto(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"anthropic/claude-opus-4-6",
		[]acp.SessionConfigSelectOption{
			{Value: "anthropic/claude-opus-4-6", Name: "Claude Opus 4.6"},
			{Value: "openai/gpt-5.6-sol", Name: "GPT-5.6 Sol"},
		},
	)
	thinkingOption := testSessionSelectOption(
		"thinking",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"auto",
		[]acp.SessionConfigSelectOption{
			{Value: "off", Name: "Off"},
			{Value: "medium", Name: "Medium"},
			{Value: "auto", Name: "Auto"},
		},
	)
	options := []acp.SessionConfigOption{{Select: modelOption}, {Select: thinkingOption}}
	for _, test := range []struct {
		name       string
		modelInput string
	}{
		{name: "Should retain runtime model for empty model", modelInput: ""},
		{name: "Should retain runtime model for explicit auto", modelInput: " AUTO "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scenario := helperScenario{
				ExpectedCWD:             t.TempDir(),
				ExpectedPrompt:          "retain active model",
				NewSessionConfigOptions: options,
				ExpectedSessionConfig: []helperExpectedSessionConfig{
					{ConfigID: "thinking", Value: "medium"},
				},
				SessionConfigResponses: [][]acp.SessionConfigOption{options},
				ExpectedConfigurationOrder: []string{
					"config:thinking=medium",
				},
				StopReason: string(acp.StopReasonEndTurn),
			}
			client := newOMPTestClient(t, scenario, func(cfg *ClientConfig) {
				cfg.Model = test.modelInput
			})

			session, err := client.CreateSession(context.Background(), SessionRequest{
				WorkingDir: scenario.ExpectedCWD,
				Prompt:     []byte(scenario.ExpectedPrompt),
			})
			if err != nil {
				t.Fatalf("create OMP session: %v", err)
			}
			updates := collectSessionUpdates(t, session)
			if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
				t.Fatalf("OMP session updates = %#v, want one completed update", updates)
			}
		})
	}
}

func TestConfigureSessionUsesAuthoritativeOptionsAfterModelSelection(t *testing.T) {
	t.Parallel()

	modelA := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"provider/model-a",
		[]acp.SessionConfigSelectOption{
			{Value: "provider/model-a", Name: "Model A"},
			{Value: "provider/model-b", Name: "Model B"},
		},
	)
	thinkingA := testSessionSelectOption(
		"thinking-a",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"medium",
		[]acp.SessionConfigSelectOption{{Value: "medium", Name: "Medium A"}},
	)
	modelB := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"provider/model-b",
		[]acp.SessionConfigSelectOption{
			{Value: "provider/model-a", Name: "Model A"},
			{Value: "provider/model-b", Name: "Model B"},
		},
	)
	thinkingB := testSessionSelectOption(
		"thinking-b",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"medium",
		[]acp.SessionConfigSelectOption{{Value: "medium", Name: "Medium B"}},
	)
	refreshedOptions := []acp.SessionConfigOption{{Select: modelB}, {Select: thinkingB}}
	scenario := helperScenario{
		ExpectedCWD:    t.TempDir(),
		ExpectedPrompt: "use model B",
		NewSessionConfigOptions: []acp.SessionConfigOption{
			{Select: modelA},
			{Select: thinkingA},
		},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "provider/model-b"},
			{ConfigID: "thinking-b", Value: "medium"},
		},
		SessionConfigResponses: [][]acp.SessionConfigOption{refreshedOptions, refreshedOptions},
		ExpectedConfigurationOrder: []string{
			"config:model=provider/model-b",
			"config:thinking-b=medium",
		},
		StopReason: string(acp.StopReasonEndTurn),
	}
	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.DefaultModel = model.DefaultOMPModel
			spec.UsesBootstrapModel = false
		},
		func(cfg *ClientConfig) {
			cfg.Model = "provider/model-b"
			cfg.ReasoningEffort = "medium"
		},
	)
	client.(*clientImpl).spec.ID = model.IDEOMP
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create OMP session: %v", err)
	}
	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
		t.Fatalf("OMP session updates = %#v, want one completed update", updates)
	}
}

func TestConfigureSessionRejectsUnsupportedOMPReasoningFromRefreshedOptions(t *testing.T) {
	t.Parallel()

	modelA := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"provider/model-a",
		[]acp.SessionConfigSelectOption{
			{Value: "provider/model-a", Name: "Model A"},
			{Value: "provider/model-b", Name: "Model B"},
		},
	)
	thinkingA := testSessionSelectOption(
		"thinking",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"medium",
		[]acp.SessionConfigSelectOption{{Value: "medium", Name: "Medium"}},
	)
	modelB := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"provider/model-b",
		[]acp.SessionConfigSelectOption{
			{Value: "provider/model-a", Name: "Model A"},
			{Value: "provider/model-b", Name: "Model B"},
		},
	)
	thinkingB := testSessionSelectOption(
		"thinking",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"auto",
		[]acp.SessionConfigSelectOption{
			{Value: "off", Name: "Off"},
			{Value: "auto", Name: "Auto"},
		},
	)
	refreshedOptions := []acp.SessionConfigOption{{Select: modelB}, {Select: thinkingB}}
	scenario := helperScenario{
		ExpectedCWD: t.TempDir(),
		NewSessionConfigOptions: []acp.SessionConfigOption{
			{Select: modelA},
			{Select: thinkingA},
		},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "model", Value: "provider/model-b"},
		},
		SessionConfigResponses: [][]acp.SessionConfigOption{refreshedOptions},
	}
	client := newOMPTestClient(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = "provider/model-b"
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{WorkingDir: scenario.ExpectedCWD})
	if err == nil {
		t.Fatal("expected unsupported OMP reasoning error")
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("error = %T, want SessionSetupError", err)
	}
	if setupErr.Stage != SessionSetupStageSetReasoning {
		t.Fatalf("setup stage = %q, want %q", setupErr.Stage, SessionSetupStageSetReasoning)
	}
	for _, want := range []string{
		`reasoning effort "medium" is not available`,
		"Off (off)",
		"Auto (auto)",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("unsupported reasoning error = %q, want %q", err, want)
		}
	}
}

func TestConfigureSessionRejectsOMPWithoutReasoningOption(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"anthropic/claude-opus-4-6",
		[]acp.SessionConfigSelectOption{{Value: "anthropic/claude-opus-4-6", Name: "Claude Opus 4.6"}},
	)
	scenario := helperScenario{
		ExpectedCWD:             t.TempDir(),
		NewSessionConfigOptions: []acp.SessionConfigOption{{Select: modelOption}},
	}
	client := newOMPTestClient(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = model.DefaultOMPModel
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{WorkingDir: scenario.ExpectedCWD})
	if err == nil {
		t.Fatal("expected missing OMP reasoning option error")
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) || setupErr.Stage != SessionSetupStageSetReasoning {
		t.Fatalf("missing OMP reasoning error = %v, want set_reasoning", err)
	}
	if !strings.Contains(
		err.Error(),
		`did not advertise an ACP reasoning option; cannot apply reasoning effort "medium"`,
	) {
		t.Fatalf("missing OMP reasoning error = %q", err)
	}
}

func TestConfigureSessionRejectsUnadvertisedOMPModelWithoutRequest(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"anthropic/claude-opus-4-6",
		[]acp.SessionConfigSelectOption{
			{Value: "anthropic/claude-opus-4-6", Name: "Claude Opus 4.6"},
			{Value: "openai/gpt-5.6-sol", Name: "GPT-5.6 Sol"},
		},
	)
	scenario := helperScenario{
		ExpectedCWD:             t.TempDir(),
		NewSessionConfigOptions: []acp.SessionConfigOption{{Select: modelOption}},
	}
	client := newOMPTestClient(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = "openai/missing"
	})

	_, err := client.CreateSession(context.Background(), SessionRequest{WorkingDir: scenario.ExpectedCWD})
	if err == nil {
		t.Fatal("expected unavailable OMP model error")
	}
	var setupErr *SessionSetupError
	if !errors.As(err, &setupErr) {
		t.Fatalf("error = %T, want SessionSetupError", err)
	}
	if setupErr.Stage != SessionSetupStageSetModel {
		t.Fatalf("setup stage = %q, want %q", setupErr.Stage, SessionSetupStageSetModel)
	}
	for _, want := range []string{
		`model "openai/missing" is not available`,
		"Claude Opus 4.6 (anthropic/claude-opus-4-6)",
		"GPT-5.6 Sol (openai/gpt-5.6-sol)",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("unavailable model error = %q, want %q", err, want)
		}
	}
}

func TestConfigureSessionDoesNotMapOMPAccessToWorkflowMode(t *testing.T) {
	t.Parallel()

	modelOption := testSessionSelectOption(
		"model",
		acp.SessionConfigOptionCategoryModel,
		"anthropic/claude-opus-4-6",
		[]acp.SessionConfigSelectOption{{Value: "anthropic/claude-opus-4-6", Name: "Claude Opus 4.6"}},
	)
	thinkingOption := testSessionSelectOption(
		"thinking",
		acp.SessionConfigOptionCategoryThoughtLevel,
		"auto",
		[]acp.SessionConfigSelectOption{{Value: "medium", Name: "Medium"}},
	)
	options := []acp.SessionConfigOption{{Select: modelOption}, {Select: thinkingOption}}
	scenario := helperScenario{
		ExpectedCWD:             t.TempDir(),
		ExpectedPrompt:          "use standard permissions",
		NewSessionConfigOptions: options,
		SessionModes: &acp.SessionModeState{
			CurrentModeId: "default",
			AvailableModes: []acp.SessionMode{
				{Id: "default", Name: "Default"},
				{Id: "plan", Name: "Plan"},
			},
		},
		ExpectedSessionConfig: []helperExpectedSessionConfig{
			{ConfigID: "thinking", Value: "medium"},
		},
		SessionConfigResponses:     [][]acp.SessionConfigOption{options},
		ExpectedConfigurationOrder: []string{"config:thinking=medium"},
		StopReason:                 string(acp.StopReasonEndTurn),
	}
	client := newOMPTestClient(t, scenario, func(cfg *ClientConfig) {
		cfg.Model = model.DefaultOMPModel
		cfg.AccessMode = model.AccessModeFull
	})

	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create OMP session: %v", err)
	}
	updates := collectSessionUpdates(t, session)
	if len(updates) != 1 || updates[0].Status != model.StatusCompleted {
		t.Fatalf("OMP session updates = %#v, want one completed update", updates)
	}
}

func newOMPTestClient(t *testing.T, scenario helperScenario, configure func(*ClientConfig)) Client {
	t.Helper()

	client := newTestClientWithSpecConfig(
		t,
		scenario,
		func(spec *Spec) {
			spec.DefaultModel = model.DefaultOMPModel
			spec.UsesBootstrapModel = false
			spec.FullAccessModeID = ""
		},
		configure,
	)
	client.(*clientImpl).spec.ID = model.IDEOMP
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})
	return client
}

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
			name:      "Should resolve exact advertised value",
			requested: "grok-4.5[effort=high,fast=true]",
			want:      "grok-4.5[effort=high,fast=true]",
		},
		{
			name:      "Should resolve friendly model name",
			requested: "grok-4.5",
			want:      "grok-4.5[effort=high,fast=true]",
		},
		{
			name:      "Should resolve normalized friendly model name",
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
	t.Run("Should report advertised choices for unsupported model", func(t *testing.T) {
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
	})
}

func TestSessionSelectValuesSupportsGroupedOptions(t *testing.T) {
	t.Run("Should flatten grouped select options into session values", func(t *testing.T) {
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
	})
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
			name:       "Should force auto for Fable canonical model under full access",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "claude-fable-5",
			accessMode: model.AccessModeFull,
			want:       "auto",
		},
		{
			name:       "Should force auto for Fable alias under default access",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "fable",
			accessMode: model.AccessModeDefault,
			want:       "auto",
		},
		{
			name:       "Should force auto for Fable one million suffix",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "claude-fable-5[1m]",
			accessMode: model.AccessModeFull,
			want:       "auto",
		},
		{
			name:       "Should force auto for Fable provider alias",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "anthropic/fable-5",
			accessMode: model.AccessModeFull,
			want:       "auto",
		},
		{
			name:       "Should preserve bypass for other Claude models",
			spec:       Spec{ID: model.IDEClaude, FullAccessModeID: "bypassPermissions"},
			modelName:  "opus",
			accessMode: model.AccessModeFull,
			want:       "bypassPermissions",
		},
		{
			name:       "Should use advertised full mode for Codex full access",
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
