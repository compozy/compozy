package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// ViewPatchPublisher fans out one declarative command-palette view patch.
type ViewPatchPublisher interface {
	PublishViewPatch(
		context.Context,
		cmdpalette.WorkspaceID,
		string,
		cmdpalette.ViewPatch,
	) error
}

// WithHostAPIViewService injects the daemon-owned programmable-view authority.
func WithHostAPIViewService(service cmdpalette.ViewService) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.views = service
	}
}

// WithHostAPIViewPatchPublisher injects the daemon-owned declarative patch hub.
func WithHostAPIViewPatchPublisher(publisher ViewPatchPublisher) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.viewPatches = publisher
	}
}

func registerHostAPIViewMethodHandler(
	handler *HostAPIHandler,
	handlers map[string]hostAPIMethodFunc,
) {
	handlers[string(extensioncontract.HostAPIMethodViewPatch)] = handler.handleViewPatch
}

func (h *HostAPIHandler) handleViewPatch(
	ctx context.Context,
	params json.RawMessage,
) (any, error) {
	var frame cmdpalette.ViewFrame
	if err := decodeHostAPIParams(params, &frame); err != nil {
		return nil, err
	}
	extensionName := hostAPIExtensionNameFromContext(ctx)
	if extensionName == "" {
		return nil, errors.New("extension: host api extension identity is required")
	}
	if strings.TrimSpace(frame.ViewSession) != "" {
		return h.publishProgramViewFrame(ctx, extensionName, frame)
	}
	return h.publishDeclarativeViewPatch(ctx, extensionName, frame)
}

func (h *HostAPIHandler) publishProgramViewFrame(
	ctx context.Context,
	extensionName string,
	frame cmdpalette.ViewFrame,
) (any, error) {
	if h.views == nil {
		return nil, errors.New("extension: command palette view service is unavailable")
	}
	if err := h.views.PublishFrame(ctx, cmdpalette.SessionToken{
		ViewSession: frame.ViewSession,
		Extension:   extensionName,
	}, frame); err != nil {
		return nil, fmt.Errorf("extension: publish command palette view frame: %w", err)
	}
	return extensioncontract.EmptyResult{}, nil
}

func (h *HostAPIHandler) publishDeclarativeViewPatch(
	ctx context.Context,
	extensionName string,
	frame cmdpalette.ViewFrame,
) (any, error) {
	if frame.Patch == nil {
		return nil, errors.New("extension: view patch requires a session or a declarative patch")
	}
	if h.viewPatches == nil {
		return nil, errors.New("extension: command palette view patch publisher is unavailable")
	}
	workspaceID, bound, err := hostAPIBoundWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	if !bound || strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("extension: declarative view patch requires a workspace-bound session")
	}
	if err := h.viewPatches.PublishViewPatch(
		ctx, cmdpalette.WorkspaceID(workspaceID), extensionName, *frame.Patch,
	); err != nil {
		return nil, fmt.Errorf("extension: publish command palette view patch: %w", err)
	}
	return extensioncontract.EmptyResult{}, nil
}
