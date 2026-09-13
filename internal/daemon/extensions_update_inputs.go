package daemon

import (
	"context"
	"errors"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
)

// The outer coordinator holds the package and selected input workspace locks through reload and rollback.
func (s *daemonExtensionService) configureUpdateInputGate(
	ctx context.Context, request *extensionpkg.MarketplaceUpdateRequest, values map[string]extensioninput.Value,
	target extensionMutationTarget,
) map[string]*extensionInputPlan {
	binder := extensionInputBinder{service: s}
	plans := make(map[string]*extensionInputPlan)
	allocations := make(map[string]*extensionMCPAllocationSnapshot)
	preflight, commit, rollback := request.PreflightCandidate, request.CommitCandidate, request.RollbackCandidate
	request.PreflightCandidate = func(info extensionpkg.ExtensionInfo, manifest *extensionpkg.Manifest) error {
		if preflight != nil {
			if err := preflight(info, manifest); err != nil {
				return err
			}
		}
		plan, err := binder.Prepare(
			ctx,
			target.key(info.Name),
			target.profile,
			manifest,
			values,
		)
		if err != nil {
			return err
		}
		snapshot, err := s.snapshotMCPAllocations(ctx, target.key(info.Name))
		if err != nil {
			return err
		}
		allocations[info.Name] = snapshot
		plans[info.Name] = plan
		return nil
	}
	request.CommitCandidate = func(info extensionpkg.ExtensionInfo, manifest *extensionpkg.Manifest) error {
		if commit != nil {
			if err := commit(info, manifest); err != nil {
				return err
			}
		}
		_, err := binder.Commit(ctx, plans[info.Name])
		return err
	}
	request.RollbackCandidate = func(rollbackCtx context.Context, info extensionpkg.ExtensionInfo) error {
		err := binder.Rollback(rollbackCtx, plans[info.Name])
		if rollback != nil {
			err = errors.Join(err, rollback(rollbackCtx, info))
		}
		if err != nil {
			return err
		}
		return s.rollbackExtensionMCPAllocations(rollbackCtx, allocations[info.Name])
	}
	return plans
}
