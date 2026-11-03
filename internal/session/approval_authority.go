package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

var ErrApprovalOutsideLineage = errors.New("session_approve_outside_lineage")

// AuthorizeSessionApproval checks a trusted actor against the target's ancestry.
func AuthorizeSessionApproval(
	ctx context.Context,
	actorID, targetID string,
	lookup func(context.Context, string) (*Info, error),
	access workspaceaccess.Policy,
) error {
	if actorID == "" {
		return nil
	}
	actor, err := lookup(ctx, actorID)
	if err != nil {
		return err
	}
	staysOn, err := IsStaysOnLineage(ctx, actorID, lookup)
	if err != nil {
		return err
	}
	if !staysOn {
		return nil
	}
	if actorID == targetID {
		return ErrApprovalOutsideLineage
	}
	target, err := lookup(ctx, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return ErrSessionNotFound
	}
	if actor.EffectiveSessionApproval == "workspace" {
		if access == nil {
			if actor.WorkspaceID == target.WorkspaceID && actor.ProfileID == target.ProfileID {
				return nil
			}
			return ErrApprovalOutsideLineage
		}
		decision, err := access.Authorize(ctx, workspaceaccess.Request{
			Actor: workspaceaccess.ActorRef{
				Kind:        workspaceaccess.ActorAgentSession,
				SessionID:   actor.ID,
				WorkspaceID: actor.WorkspaceID,
				AgentName:   actor.AgentName,
			},
			Target: workspaceaccess.ScopeTarget(
				target.WorkspaceID,
				target.ProfileID,
			),
			Seam: workspaceaccess.SeamCatalog,
		})
		if err != nil {
			return err
		}
		if decision.Allowed {
			return nil
		}
		return ErrApprovalOutsideLineage
	}
	seen := map[string]bool{}
	for target != nil && !seen[target.ID] {
		seen[target.ID] = true
		if target.Lineage == nil || target.Lineage.ParentSessionID == "" {
			break
		}
		if target.Lineage.ParentSessionID == actorID {
			return nil
		}
		target, err = lookup(ctx, target.Lineage.ParentSessionID)
		if err != nil {
			return err
		}
	}
	return ErrApprovalOutsideLineage
}

// IsStaysOnLineage follows persisted parent links, including descendants without an instance ID.
func IsStaysOnLineage(
	ctx context.Context,
	actorID string,
	lookup func(context.Context, string) (*Info, error),
) (bool, error) {
	seen := map[string]bool{}
	for actorID != "" && !seen[actorID] {
		seen[actorID] = true
		current, err := lookup(ctx, actorID)
		if err != nil {
			return false, err
		}
		if current == nil {
			return false, ErrSessionNotFound
		}
		if current.InstanceID != "" {
			return true, nil
		}
		if current.Lineage == nil {
			return false, nil
		}
		actorID = current.Lineage.ParentSessionID
	}
	if actorID != "" {
		return false, ErrApprovalOutsideLineage
	}
	return false, nil
}

// AuthorizeToolApprovalGrantMutation keeps durable grants under operator control in stays-on lineages.
func AuthorizeToolApprovalGrantMutation(
	ctx context.Context,
	actorID string,
	lookup func(context.Context, string) (*Info, error),
) error {
	staysOn, err := IsStaysOnLineage(ctx, actorID, lookup)
	if err != nil {
		return err
	}
	if staysOn {
		return ErrToolApprovalGrantDenied
	}
	return nil
}

var ErrToolApprovalGrantDenied = errors.New(
	"tool_approval_grants_denied: stays-on lineages cannot set or revoke tool approval grants",
)
