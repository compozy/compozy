package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// WithHostAPIViewService injects the daemon-owned programmable-view authority.
func WithHostAPIViewService(service cmdpalette.ViewService) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.views = service
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
	if h.views == nil {
		return nil, errors.New("extension: command palette view service is unavailable")
	}
	var frame cmdpalette.ViewFrame
	if err := decodeHostAPIParams(params, &frame); err != nil {
		return nil, err
	}
	extensionName := hostAPIExtensionNameFromContext(ctx)
	if extensionName == "" {
		return nil, errors.New("extension: host api extension identity is required")
	}
	if err := h.views.PublishFrame(ctx, cmdpalette.SessionToken{
		ViewSession: frame.ViewSession,
		Extension:   extensionName,
	}, frame); err != nil {
		return nil, fmt.Errorf("extension: publish command palette view frame: %w", err)
	}
	return extensioncontract.EmptyResult{}, nil
}
