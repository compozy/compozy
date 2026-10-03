package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

type boundToolApprovalIssuer struct {
	tokens   toolspkg.ApprovalTokenIssuer
	registry toolspkg.Registry
	binder   toolspkg.CallInputBinder
}

var _ toolspkg.ApprovalTokenIssuer = (*boundToolApprovalIssuer)(nil)

func (i *boundToolApprovalIssuer) CreateToolApproval(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.ApprovalTokenRequest,
) (toolspkg.ApprovalTokenGrant, error) {
	if len(req.Input) == 0 && strings.TrimSpace(req.InputDigest) != "" {
		return i.tokens.CreateToolApproval(ctx, scope, req)
	}
	if _, err := toolspkg.ApprovalInputDigest(req.Input, req.InputDigest); err != nil {
		return toolspkg.ApprovalTokenGrant{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput, req.ToolID, "input digest is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err), toolspkg.ReasonSchemaInvalid,
		)
	}
	view, err := i.registry.Get(ctx, scope, req.ToolID)
	if err != nil {
		return toolspkg.ApprovalTokenGrant{}, err
	}
	if len(req.Input) == 0 {
		req.Input = json.RawMessage(`{}`)
	}
	boundInput, err := i.binder.BindCallInput(ctx, scope, view.Descriptor, req.Input)
	if err != nil {
		return toolspkg.ApprovalTokenGrant{}, err
	}
	req.Input = boundInput
	req.InputDigest = ""
	return i.tokens.CreateToolApproval(ctx, scope, req)
}

func (d *Daemon) bootToolApprovalServices(
	state *bootState,
) (*toolspkg.ApprovalTokenStore, *toolApprovalBridge, error) {
	grantStore, ok := state.registry.(toolspkg.ApprovalGrantStore)
	if !ok {
		return nil, nil, errors.New("daemon: global registry does not support durable tool approval grants")
	}
	approvalGrants := newToolApprovalGrantService(
		grantStore,
		extensionEventSummaryStore(state.registry),
		state.logger,
		d.now,
	)
	state.deps.ApprovalGrants = approvalGrants
	approvalTokens := toolspkg.NewApprovalTokenStore(state.cfg.Tools.Policy.ApprovalTimeout())
	return approvalTokens, newBootToolApprovalBridge(state, approvalTokens, approvalGrants), nil
}

func newBootToolApprovalBridge(
	state *bootState,
	approvalTokens toolspkg.ApprovalTokenConsumer,
	approvalGrants toolspkg.ApprovalGrantStore,
) *toolApprovalBridge {
	return newToolApprovalBridge(
		sessionPermissionRequesterProvider(state),
		state.cfg.Tools.Policy.ApprovalTimeout(),
		approvalTokens,
		approvalGrants,
		state.logger,
	)
}

func sessionPermissionRequesterProvider(state *bootState) func() sessionPermissionRequester {
	if state == nil {
		return nil
	}
	if _, ok := state.sessions.(sessionPermissionRequester); !ok {
		return nil
	}
	return func() sessionPermissionRequester {
		requester, ok := state.sessions.(sessionPermissionRequester)
		if !ok {
			return nil
		}
		return requester
	}
}
