package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/extensioninput"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// extensionSetupToolInput carries no workspace or profile: the instance is the caller's trusted scope.
type extensionSetupToolInput struct {
	Owner  string                          `json:"owner"`
	Name   string                          `json:"name"`
	Inputs map[string]extensioninput.Value `json:"inputs"`
}

func (n *daemonNativeTools) extensionSetupToolBindings(
	availability *toolspkg.NativeAvailabilityGroup,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDExtensionsSetup: {call: n.extensionSetup, availability: availability},
	}
}

// extensionSetup reads input state, or writes inputs when any are given. The result is the
// GET/PUT /api/extensions/{name}/inputs payload.
func (n *daemonNativeTools) extensionSetup(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input extensionSetupToolInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	name, err := requiredNativeExtensionName(req.ToolID, input.Name, input.Owner)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := nativeExtensionScopedActorContext(scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	target := extensionInputsTarget{Name: name, WorkspaceID: actor.Scope.WorkspaceID}
	var payload contract.ExtensionInputsPayload
	if len(input.Inputs) == 0 {
		payload, err = n.extensionService().InputsState(ctx, target, actor)
	} else {
		payload, err = n.extensionService().SetInputs(ctx, target, input.Inputs, actor)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeExtensionToolError(req.ToolID, err)
	}
	return structuredResult(payload, fmt.Sprintf("%s: %d required inputs missing", name, len(payload.Missing)))
}
