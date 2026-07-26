package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeLoopValidationKey = "validation"
	nativeLoopOKKey         = "ok"
)

func (n *daemonNativeTools) loopToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDGoalGet: {
			call:         n.goalGet,
			availability: n.goalGetAvailability,
		},
		toolspkg.ToolIDGoalReport: {
			call:         n.goalReport,
			availability: n.goalReportAvailability,
		},
		toolspkg.ToolIDLoopList: {
			call:         n.loopList,
			availability: availability,
		},
		toolspkg.ToolIDLoopInspect: {
			call:         n.loopInspect,
			availability: availability,
		},
		toolspkg.ToolIDLoopValidate: {
			call:         n.loopValidate,
			availability: availability,
		},
		toolspkg.ToolIDLoopCreate: {
			call:         n.loopCreate,
			availability: availability,
		},
		toolspkg.ToolIDLoopRun: {
			call:         n.loopRun,
			availability: availability,
		},
		toolspkg.ToolIDLoopStatus: {
			call:         n.loopStatus,
			availability: availability,
		},
		toolspkg.ToolIDLoopRuns: {
			call:         n.loopRuns,
			availability: availability,
		},
		toolspkg.ToolIDLoopTurns: {
			call:         n.loopTurns,
			availability: availability,
		},
		toolspkg.ToolIDLoopStop: {
			call:         n.loopStop,
			availability: availability,
		},
		toolspkg.ToolIDLoopPause: {
			call:         n.loopPause,
			availability: availability,
		},
		toolspkg.ToolIDLoopResume: {
			call:         n.loopResume,
			availability: availability,
		},
		toolspkg.ToolIDLoopConfigure: {
			call:         n.loopConfigure,
			availability: availability,
		},
		toolspkg.ToolIDLoopApprove: {
			call:         n.loopApprove,
			availability: availability,
		},
		toolspkg.ToolIDLoopDelete: {
			call:         n.loopDelete,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) loopList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopWorkspaceInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeLoopWorkspaceID(req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().ListLoops(ctx, workspaceID, looppkg.CatalogQuery{
		Search:   input.Q,
		Kind:     input.Kind,
		Category: input.Category,
		Status:   input.Status,
		Sort:     input.Sort,
		Cursor:   input.Cursor,
		Limit:    input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	preview := fmt.Sprintf("%d of %d loops", len(response.Loops), response.Page.Total)
	if response.Page.HasMore {
		preview += "; more available"
	}
	return structuredResult(response, preview)
}

func (n *daemonNativeTools) loopInspect(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, name, err := nativeLoopWorkspaceAndName(req.ToolID, input.WorkspaceID, input.Name, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().GetLoop(ctx, workspaceID, name)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	loop := response.Loop
	return structuredResult(map[string]any{
		nativeConfigHookToolsNameKey: loop.Name,
		"inputs":                     loop.Definition.Inputs,
		"contract":                   loop.Definition.Contract,
		"start":                      loop.Definition.Start,
		"version":                    loop.Version,
	}, fmt.Sprintf("loop %s v%d", loop.Name, loop.Version))
}

func (n *daemonNativeTools) loopValidate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopValidateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeLoopWorkspaceID(req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	document, err := loopDefinitionDocument(input.Definition)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	response, err := n.loopService().ValidateLoop(
		ctx,
		workspaceID,
		strings.TrimSpace(input.Name),
		contract.ValidateLoopRequest{Definition: document},
	)
	if err != nil {
		if lint, ok := errors.AsType[*core.LoopLintFailedError](err); ok {
			return structuredResult(map[string]any{
				nativeLoopValidationKey: contract.LoopValidationResponse{
					Valid:  false,
					Errors: lint.Errors,
				},
			}, "loop validation failed")
		}
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeLoopValidationKey: response}, "loop validation passed")
}

func (n *daemonNativeTools) loopCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeLoopWorkspaceID(req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.ExpectedVersion != nil {
		if input.Definition == nil {
			return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "definition")
		}
		name, err := requiredNativeString(req.ToolID, "definition.meta.name", input.Definition.Meta.Name)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		document, err := loopDefinitionDocument(*input.Definition)
		if err != nil {
			return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
		}
		response, err := n.loopService().PatchLoop(ctx, workspaceID, name, contract.PatchLoopRequest{
			ExpectedVersion: input.ExpectedVersion,
			Definition:      document,
		})
		if err != nil {
			return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
		}
		return structuredResult(response, fmt.Sprintf("loop %s v%d", response.Loop.Name, response.Loop.Version))
	}

	var document *contract.LoopDefinitionDocument
	if input.Definition != nil {
		converted, err := loopDefinitionDocument(*input.Definition)
		if err != nil {
			return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
		}
		document = &converted
	}
	response, err := n.loopService().CreateLoop(ctx, workspaceID, contract.CreateLoopRequest{
		Definition:   document,
		ForkFromName: strings.TrimSpace(input.ForkFromName),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}

	return structuredResult(response, fmt.Sprintf("loop %s v%d", response.Loop.Name, response.Loop.Version))
}

func (n *daemonNativeTools) loopRun(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRunInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, name, err := nativeLoopWorkspaceAndName(req.ToolID, input.WorkspaceID, input.Name, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	configOverrides, err := loopConfigPayload(input.ConfigOverrides)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	response, err := n.loopService().RunLoop(ctx, workspaceID, name, contract.RunLoopRequest{
		Inputs:               input.Inputs,
		ParentLoopRunID:      strings.TrimSpace(input.ParentLoopRunID),
		ConfigOverrides:      configOverrides,
		NetworkParticipation: participation.CloneRequest(input.NetworkParticipation),
	}, dsl.StartNativeTool, actor, input.Dry)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	preview := fmt.Sprintf("loop %s started", name)
	if input.Dry {
		preview = fmt.Sprintf("loop %s dry run", name)
	}
	return structuredResult(response, preview)
}

func (n *daemonNativeTools) loopStatus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRunIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := nativeLoopWorkspaceAndRunID(req.ToolID, input.WorkspaceID, input.RunID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().GetLoopRun(ctx, workspaceID, runID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("loop run %s is %s", runID, response.Run.Status))
}

func (n *daemonNativeTools) loopRuns(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRunsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeLoopWorkspaceID(req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().ListLoopRuns(ctx, workspaceID, core.LoopRunListQuery{
		LoopName: strings.TrimSpace(input.LoopName),
		Status:   strings.TrimSpace(input.Status),
		Limit:    input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("%d loop runs", len(response.Runs)))
}

func (n *daemonNativeTools) loopStop(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.loopRunMutation(ctx, scope, req, n.loopService().StopLoopRun, "stopped")
}

func (n *daemonNativeTools) loopPause(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.loopRunMutation(ctx, scope, req, n.loopService().PauseLoopRun, "pause requested")
}

func (n *daemonNativeTools) loopResume(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.loopRunMutation(ctx, scope, req, n.loopService().ResumeLoopRun, "resumed")
}

func (n *daemonNativeTools) loopConfigure(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopConfigureInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, name, err := nativeLoopWorkspaceAndName(req.ToolID, input.WorkspaceID, input.Name, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	config, err := loopConfigPayload(&input.Config)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	response, err := n.loopService().PutLoopConfig(ctx, workspaceID, name, contract.PutLoopConfigRequest{
		Config: *config,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("loop %s configured", name))
}

func (n *daemonNativeTools) loopApprove(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopApproveInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := validateNativeLoopApprovalHash(req.ToolID, input.ApprovalTokenHash); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := nativeLoopWorkspaceAndRunID(req.ToolID, input.WorkspaceID, input.RunID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	gateID, err := requiredNativeString(req.ToolID, "gate_id", input.GateID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	decision := looppkg.GateDecision(strings.TrimSpace(input.Decision))
	if !nativeLoopGateDecisionValid(decision) {
		return toolspkg.ToolResult{}, nativeLoopToolError(
			req.ToolID,
			fmt.Errorf("%w: gate decision is invalid: %q", looppkg.ErrValidation, decision),
		)
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	if err := n.loopService().ApproveLoopRun(ctx, workspaceID, runID, contract.ApproveLoopRunRequest{
		GateID:   gateID,
		Decision: contract.LoopGateDecision(decision),
	}, actor); err != nil {
		return toolspkg.ToolResult{}, nativeLoopApproveError(req.ToolID, err)
	}
	message := fmt.Sprintf("loop run %s gate %s decided", runID, gateID)
	return structuredResult(map[string]any{nativeLoopOKKey: true}, message)
}

func (n *daemonNativeTools) loopDelete(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopNameInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, name, err := nativeLoopWorkspaceAndName(req.ToolID, input.WorkspaceID, input.Name, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := n.loopService().DeleteLoop(ctx, workspaceID, name); err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeLoopOKKey: true}, fmt.Sprintf("loop %s deleted", name))
}

func (n *daemonNativeTools) loopRunMutation(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	mutate func(context.Context, string, string, taskpkg.ActorContext) error,
	preview string,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRunIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := nativeLoopWorkspaceAndRunID(req.ToolID, input.WorkspaceID, input.RunID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := mutate(ctx, workspaceID, runID, actor); err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeLoopOKKey: true}, fmt.Sprintf("loop run %s %s", runID, preview))
}
