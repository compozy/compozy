package daemon

import (
	"errors"

	toolspkg "github.com/compozy/agh/internal/tools"
)

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
	var sessions func() sessionPermissionRequester
	if _, ok := state.sessions.(sessionPermissionRequester); ok {
		sessions = func() sessionPermissionRequester {
			requester, ok := state.sessions.(sessionPermissionRequester)
			if !ok {
				return nil
			}
			return requester
		}
	}
	return newToolApprovalBridge(
		sessions,
		state.cfg.Tools.Policy.ApprovalTimeout(),
		approvalTokens,
		approvalGrants,
		state.logger,
	)
}
