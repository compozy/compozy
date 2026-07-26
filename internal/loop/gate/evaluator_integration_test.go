//go:build integration

package gate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/tools"
)

type integrationToolProvider struct {
	source     tools.SourceRef
	descriptor tools.Descriptor
	handle     tools.Handle
}

func (p integrationToolProvider) ID() tools.SourceRef {
	return p.source
}

func (p integrationToolProvider) List(context.Context, tools.Scope) ([]tools.Descriptor, error) {
	return []tools.Descriptor{p.descriptor}, nil
}

func (p integrationToolProvider) Resolve(
	_ context.Context,
	_ tools.Scope,
	id tools.ToolID,
) (tools.Handle, bool, error) {
	if id != p.descriptor.ID {
		return nil, false, nil
	}
	return p.handle, true, nil
}

type integrationToolHandle struct {
	descriptor tools.Descriptor
	result     tools.ToolResult
	request    *tools.CallRequest
}

func (h *integrationToolHandle) Descriptor() tools.Descriptor {
	return h.descriptor
}

func (h *integrationToolHandle) Availability(context.Context, tools.Scope) tools.Availability {
	return tools.Availability{
		Registered: true,
		Enabled:    true,
		Available:  true,
		Authorized: true,
		Executable: true,
	}
}

func (h *integrationToolHandle) Call(_ context.Context, req tools.CallRequest) (tools.ToolResult, error) {
	h.request = &req
	return h.result, nil
}

type integrationAllowPolicy struct{}

func (integrationAllowPolicy) Evaluate(
	context.Context,
	tools.Scope,
	tools.Descriptor,
) (tools.EffectiveToolDecision, error) {
	return tools.EffectiveToolDecision{
		VisibleToOperator: true,
		VisibleToSession:  true,
		Callable:          true,
	}, nil
}

func TestEvaluatorExtensionGateIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should run extension gate check via tools call", func(t *testing.T) {
		t.Parallel()

		descriptor := validExtensionDescriptor()
		handle := &integrationToolHandle{
			descriptor: descriptor,
			result: tools.ToolResult{
				Structured: json.RawMessage(`{"verdict":"pass","evidence":{"tool":"checked"}}`),
			},
		}
		registry, err := tools.NewRegistry(
			tools.WithProviders(integrationToolProvider{
				source:     descriptor.Source,
				descriptor: descriptor,
				handle:     handle,
			}),
			tools.WithPolicyEvaluator(integrationAllowPolicy{}),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		evaluator := NewEvaluator(WithToolCaller(registry))
		verdict, err := evaluator.Evaluate(context.Background(), Gate{
			ID:            "extension_gate",
			VerdictPolicy: dsl.VerdictPolicyFixedPasses,
			Criteria: []dsl.GateCriterion{{
				ID:     "extension_quality",
				Type:   dsl.CriterionExtension,
				Tool:   string(descriptor.ID),
				Inputs: map[string]any{"query": "quality"},
			}},
		}, GateInput{
			Placement: PlacementInBody,
			ToolScope: tools.Scope{
				WorkspaceID: "ws-1",
				SessionID:   "sess-1",
				AgentName:   "agent-1",
				ActorKind:   "automation",
			},
			ToolCallCorrelationID: "corr-1",
		})
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		requireOutcome(t, verdict, VerdictOutcomeApproved)
		if verdict.Route.Action != RouteContinue {
			t.Fatalf("Route.Action = %q, want %q", verdict.Route.Action, RouteContinue)
		}
		if handle.request == nil {
			t.Fatal("extension handle request = nil, want call")
		}
		if handle.request.WorkspaceID != "ws-1" {
			t.Fatalf("CallRequest.WorkspaceID = %q, want ws-1", handle.request.WorkspaceID)
		}
		if handle.request.CorrelationID != "corr-1" {
			t.Fatalf("CallRequest.CorrelationID = %q, want corr-1", handle.request.CorrelationID)
		}
	})
}

func validExtensionDescriptor() tools.Descriptor {
	return tools.Descriptor{
		ID:           "ext__quality__gate",
		DisplayTitle: "Quality Gate",
		Description:  "Evaluates loop gate quality",
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Backend: tools.BackendRef{
			Kind:        tools.BackendExtensionHost,
			ExtensionID: "quality",
			Handler:     "gate",
		},
		Source: tools.SourceRef{
			Kind:  tools.SourceExtension,
			Owner: "quality",
		},
		Visibility:      tools.VisibilityModel,
		Risk:            tools.RiskRead,
		ReadOnly:        true,
		ConcurrencySafe: true,
		MaxResultBytes:  1024,
	}
}
