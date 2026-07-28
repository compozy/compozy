package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNativeProviderDispatch(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve and call native handlers through registry dispatch", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		descriptor.ID = ToolIDSkillList
		descriptor.Backend.NativeName = "skill_list"
		called := false
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Call: func(_ context.Context, scope Scope, req CallRequest) (ToolResult, error) {
				called = true
				if scope.SessionID != "sess-1" || scope.WorkspaceID != "ws-1" || scope.AgentName != "agent-1" {
					t.Fatalf("scope = %#v, want call request scope", scope)
				}
				if req.ToolID != descriptor.ID {
					t.Fatalf("req.ToolID = %q, want %q", req.ToolID, descriptor.ID)
				}
				return ToolResult{Structured: json.RawMessage(`{"ok":true}`)}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewNativeProvider() error = %v", err)
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		result, err := registry.Call(t.Context(), Scope{
			SessionID:   "sess-1",
			WorkspaceID: "ws-1",
			AgentName:   "agent-1",
		}, CallRequest{ToolID: descriptor.ID})
		if err != nil {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want nil", err)
		}
		if !called {
			t.Fatal("native handler was not called")
		}
		if got, want := string(result.Structured), `{"ok":true}`; got != want {
			t.Fatalf("result.Structured = %s, want %s", got, want)
		}
	})

	t.Run("Should reject schema invalid input before native handler invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		called := false
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewNativeProvider() error = %v", err)
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		_, err = registry.Call(
			t.Context(),
			Scope{},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":7}`)},
		)
		if !errors.Is(err, ErrToolInvalidInput) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolInvalidInput", err)
		}
		if called {
			t.Fatal("native handler was called for schema-invalid input")
		}
	})

	t.Run("Should enforce array items and length schema keywords before native handler invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		descriptor.InputSchema = json.RawMessage(`{
			"type":"object",
			"required":["designations"],
			"properties":{
				"designations":{
					"type":"array",
					"minItems":1,
					"items":{
						"type":"object",
						"required":["brief"],
						"properties":{"brief":{"type":"string","minLength":1}},
						"additionalProperties":false
					}
				}
			},
			"additionalProperties":false
		}`)
		calls := 0
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				calls++
				return ToolResult{Structured: json.RawMessage(`{"ok":true}`)}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewNativeProvider() error = %v", err)
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		for _, input := range []json.RawMessage{
			json.RawMessage(`{"designations":[]}`),
			json.RawMessage(`{"designations":[{"brief":""}]}`),
			json.RawMessage(`{"designations":[{"brief":7}]}`),
		} {
			_, err := registry.Call(t.Context(), Scope{}, CallRequest{ToolID: descriptor.ID, Input: input})
			if !errors.Is(err, ErrToolInvalidInput) {
				t.Fatalf("RuntimeRegistry.Call(%s) error = %v, want ErrToolInvalidInput", input, err)
			}
		}
		if calls != 0 {
			t.Fatalf("native handler calls after invalid inputs = %d, want 0", calls)
		}

		if _, err := registry.Call(
			t.Context(),
			Scope{},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"designations":[{"brief":"Investigate"}]}`)},
		); err != nil {
			t.Fatalf("RuntimeRegistry.Call(valid) error = %v, want nil", err)
		}
		if calls != 1 {
			t.Fatalf("native handler calls after valid input = %d, want 1", calls)
		}
	})

	t.Run("Should reject unsupported schema type declarations before native handler registration", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		descriptor.InputSchema = json.RawMessage(`{
			"type":"object",
			"properties":{"query":{"type":["string","strng"]}}
		}`)
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				t.Fatal("native handler was registered for schema-invalid descriptor")
				return ToolResult{}, nil
			},
		})
		if err == nil {
			t.Fatalf("NewNativeProvider() = %#v, nil error; want schema invalid error", provider)
		}
		requireReason(t, err, ReasonSchemaInvalid)
	})

	t.Run("Should enforce enum oneOf and not schema rules before native handler invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		descriptor.InputSchema = json.RawMessage(`{
			"type":"object",
			"required":["surface"],
			"properties":{
				"surface":{"type":"string","enum":["thread","direct"]},
				"thread_id":{"type":"string"},
				"direct_id":{"type":"string"}
			},
			"oneOf":[
				{
					"required":["thread_id"],
					"properties":{"surface":{"enum":["thread"]}},
					"not":{"required":["direct_id"]}
				},
				{
					"required":["direct_id"],
					"properties":{"surface":{"enum":["direct"]}},
					"not":{"required":["thread_id"]}
				}
			],
			"additionalProperties":false
		}`)
		calls := 0
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				calls++
				return ToolResult{Structured: json.RawMessage(`{"ok":true}`)}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewNativeProvider() error = %v", err)
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		invalidInputs := []json.RawMessage{
			json.RawMessage(`{"surface":"thread"}`),
			json.RawMessage(`{"surface":"direct","thread_id":"thread_1"}`),
			json.RawMessage(`{"surface":"thread","thread_id":"thread_1","direct_id":"direct_1"}`),
			json.RawMessage(`{"surface":"legacy","thread_id":"thread_1"}`),
			json.RawMessage(`{"surface":"thread","thread_id":"thread_1","interaction_id":"old"}`),
		}
		for _, input := range invalidInputs {
			_, err := registry.Call(t.Context(), Scope{}, CallRequest{ToolID: descriptor.ID, Input: input})
			if !errors.Is(err, ErrToolInvalidInput) {
				t.Fatalf("RuntimeRegistry.Call(%s) error = %v, want ErrToolInvalidInput", input, err)
			}
		}
		if calls != 0 {
			t.Fatalf("native handler calls after invalid inputs = %d, want 0", calls)
		}

		for _, input := range []json.RawMessage{
			json.RawMessage(`{"surface":"thread","thread_id":"thread_1"}`),
			json.RawMessage(`{"surface":"direct","direct_id":"direct_1"}`),
		} {
			if _, err := registry.Call(
				t.Context(),
				Scope{},
				CallRequest{ToolID: descriptor.ID, Input: input},
			); err != nil {
				t.Fatalf("RuntimeRegistry.Call(%s) error = %v, want nil", input, err)
			}
		}
		if calls != 2 {
			t.Fatalf("native handler calls after valid inputs = %d, want 2", calls)
		}
	})

	t.Run("Should surface unavailable dependencies before native handler invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		called := false
		provider, err := NewNativeProvider(descriptor.Source, NativeTool{
			Descriptor: descriptor,
			Availability: func(context.Context, Scope) Availability {
				return Unavailable(ReasonDependencyMissing)
			},
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewNativeProvider() error = %v", err)
		}
		registry, err := NewRegistry(WithProviders(provider))
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		_, err = registry.Call(t.Context(), Scope{}, CallRequest{ToolID: descriptor.ID})
		if !errors.Is(err, ErrToolUnavailable) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolUnavailable", err)
		}
		if called {
			t.Fatal("native handler was called for unavailable dependency")
		}
	})
}

func TestNativeProviderValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject native tools without handlers", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		_, err := NewNativeProvider(descriptor.Source, NativeTool{Descriptor: descriptor})
		requireReason(t, err, ReasonHandlerMissing)
	})

	t.Run("Should reject native tools whose source differs from provider source", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		source := descriptor.Source
		source.Owner = "other"
		_, err := NewNativeProvider(source, NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				return ToolResult{}, nil
			},
		})
		requireReason(t, err, ReasonSourceDisabled)
	})

	t.Run("Should reject duplicate native tool IDs", func(t *testing.T) {
		t.Parallel()

		descriptor := validDescriptor()
		nativeTool := NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, Scope, CallRequest) (ToolResult, error) {
				return ToolResult{}, nil
			},
		}
		_, err := NewNativeProvider(descriptor.Source, nativeTool, nativeTool)
		requireReason(t, err, ReasonConflictedID)
	})
}
