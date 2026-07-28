package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type stubExecutor struct {
	kind HookExecutorKind
}

func (e stubExecutor) Kind() HookExecutorKind {
	return e.kind
}

func (e stubExecutor) Execute(_ context.Context, _ RegisteredHook, _ []byte) ([]byte, error) {
	return nil, nil
}

func TestHookSourceOrderingAndJSON(t *testing.T) {
	t.Parallel()

	if HookSourceNative >= HookSourceConfig ||
		HookSourceConfig >= HookSourceAgentDefinition ||
		HookSourceAgentDefinition >= HookSourceSkill {
		t.Fatalf("unexpected HookSource ordering: native=%d config=%d agent_definition=%d skill=%d",
			HookSourceNative, HookSourceConfig, HookSourceAgentDefinition, HookSourceSkill)
	}

	data, err := json.Marshal(HookSourceAgentDefinition)
	if err != nil {
		t.Fatalf("json.Marshal(HookSourceAgentDefinition) error = %v", err)
	}
	if string(data) != `"agent_definition"` {
		t.Fatalf("json.Marshal(HookSourceAgentDefinition) = %s, want %q", string(data), `"agent_definition"`)
	}

	var decoded HookSource
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(HookSource) error = %v", err)
	}
	if decoded != HookSourceAgentDefinition {
		t.Fatalf("decoded HookSource = %v, want %v", decoded, HookSourceAgentDefinition)
	}
}

func TestHookSourceInvalid(t *testing.T) {
	t.Parallel()

	if got := HookSource(42).String(); got != "" {
		t.Fatalf("HookSource(42).String() = %q, want empty string", got)
	}
	if _, err := HookSource(42).MarshalText(); err == nil {
		t.Fatal("HookSource(42).MarshalText() error = nil, want non-nil")
	}
}

func TestHookSkillSourceValidate(t *testing.T) {
	t.Parallel()

	if err := HookSkillSourceWorkspace.Validate(); err != nil {
		t.Fatalf("HookSkillSourceWorkspace.Validate() error = %v, want nil", err)
	}
	if err := HookSkillSource("remote").Validate(); err == nil {
		t.Fatal("invalid HookSkillSource.Validate() error = nil, want non-nil")
	}
}

func TestHookModeAndExecutorKindValidate(t *testing.T) {
	t.Parallel()

	if err := HookModeSync.Validate(); err != nil {
		t.Fatalf("HookModeSync.Validate() error = %v, want nil", err)
	}
	if err := HookMode("later").Validate(); err == nil {
		t.Fatal("invalid HookMode.Validate() error = nil, want non-nil")
	}

	if err := HookExecutorSubprocess.Validate(); err != nil {
		t.Fatalf("HookExecutorSubprocess.Validate() error = %v, want nil", err)
	}
	if err := HookExecutorKind("socket").Validate(); !errors.Is(err, ErrInvalidHookExecutorKind) {
		t.Fatalf("invalid HookExecutorKind.Validate() error = %v, want ErrInvalidHookExecutorKind", err)
	}
}

func TestRegisteredHookValidate(t *testing.T) {
	t.Parallel()

	base := RegisteredHook{
		Name:     "test-hook",
		Event:    HookSessionPreCreate,
		Source:   HookSourceConfig,
		Mode:     HookModeSync,
		Required: false,
		Priority: 500,
		Timeout:  5 * time.Second,
	}

	tests := []struct {
		name    string
		hook    RegisteredHook
		wantErr bool
	}{
		{
			name:    "valid sync hook",
			hook:    base,
			wantErr: false,
		},
		{
			name: "required async hook fails",
			hook: func() RegisteredHook {
				hook := base
				hook.Mode = HookModeAsync
				hook.Required = true
				return hook
			}(),
			wantErr: true,
		},
		{
			name: "sync async-only event fails",
			hook: func() RegisteredHook {
				hook := base
				hook.Event = HookMessageDelta
				return hook
			}(),
			wantErr: true,
		},
		{
			name: "sync task status changed hook fails",
			hook: func() RegisteredHook {
				hook := base
				hook.Event = HookTaskStatusChanged
				return hook
			}(),
			wantErr: true,
		},
		{
			name: "negative timeout fails",
			hook: func() RegisteredHook {
				hook := base
				hook.Timeout = -time.Second
				return hook
			}(),
			wantErr: true,
		},
		{
			name: "invalid source fails",
			hook: func() RegisteredHook {
				hook := base
				hook.Source = HookSource(99)
				return hook
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.hook.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("RegisteredHook.Validate() error = nil, want non-nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("RegisteredHook.Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestResolvedHookValidate(t *testing.T) {
	t.Parallel()

	hook := ResolvedHook{
		RegisteredHook: RegisteredHook{
			Name:     "resolved-hook",
			Event:    HookToolPreCall,
			Source:   HookSourceNative,
			Mode:     HookModeSync,
			Executor: stubExecutor{kind: HookExecutorNative},
		},
		Decl: HookDecl{Name: "other-name"},
	}

	if err := hook.Validate(); err == nil {
		t.Fatal("ResolvedHook.Validate() error = nil, want non-nil")
	}
}

func TestResolvedHookValidateSuccess(t *testing.T) {
	t.Parallel()

	hook := ResolvedHook{
		RegisteredHook: RegisteredHook{
			Name:     "resolved-hook",
			Event:    HookToolPreCall,
			Source:   HookSourceNative,
			Mode:     HookModeSync,
			Executor: stubExecutor{kind: HookExecutorNative},
		},
		Decl: HookDecl{Name: "resolved-hook", ExecutorKind: HookExecutorNative},
	}

	if err := hook.Validate(); err != nil {
		t.Fatalf("ResolvedHook.Validate() error = %v, want nil", err)
	}
}

func TestResolvedHookValidateRequiresExecutor(t *testing.T) {
	t.Parallel()

	hook := ResolvedHook{
		RegisteredHook: RegisteredHook{
			Name:   "resolved-hook",
			Event:  HookToolPreCall,
			Source: HookSourceNative,
			Mode:   HookModeSync,
		},
		Decl: HookDecl{Name: "resolved-hook", ExecutorKind: HookExecutorNative},
	}

	if err := hook.Validate(); err == nil {
		t.Fatal("ResolvedHook.Validate() error = nil, want non-nil")
	}
}
