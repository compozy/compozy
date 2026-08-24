package settings

import (
	"context"
	"errors"
	"fmt"
)

type mcpDefinitionMutationOutcome uint8

const (
	mcpDefinitionMutationUnchanged mcpDefinitionMutationOutcome = iota
	mcpDefinitionMutationRestored
	mcpDefinitionMutationCommitted
)

func mcpScopeIdentifier(scope ScopeKind, workspaceID, profileName string) string {
	if scope == ScopeProfile {
		return profileName
	}
	return workspaceID
}

func (s *service) withMCPAuthDefinitionMutation(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	profileName string,
	name string,
	mutate func() error,
	rollback func() error,
) (mcpDefinitionMutationOutcome, error) {
	if mutate == nil {
		return mcpDefinitionMutationUnchanged, errors.New("settings: MCP definition mutation is required")
	}
	if rollback == nil {
		return mcpDefinitionMutationUnchanged, errors.New("settings: MCP definition rollback is required")
	}
	if s.mcpAuth == nil {
		if err := mutate(); err != nil {
			return mcpDefinitionMutationUnchanged, err
		}
		return mcpDefinitionMutationCommitted, nil
	}
	target, err := normalizeMCPAuthTarget(MCPAuthTargetRequest{
		Scope: scope, WorkspaceID: workspaceID, ProfileName: profileName, Name: name,
	})
	if err != nil {
		return mcpDefinitionMutationUnchanged, err
	}
	if err := s.mcpAuth.MCPAuthInvalidate(target); err != nil {
		return mcpDefinitionMutationUnchanged, fmt.Errorf(
			"settings: invalidate pending MCP auth before definition mutation: %w",
			err,
		)
	}
	if err := mutate(); err != nil {
		return mcpDefinitionMutationUnchanged, err
	}
	deleteStateErr := s.mcpAuth.MCPAuthDeleteState(ctx, target)
	if deleteStateErr != nil {
		deleteStateErr = fmt.Errorf("settings: delete MCP auth state after definition mutation: %w", deleteStateErr)
	}
	if deleteStateErr == nil {
		return mcpDefinitionMutationCommitted, nil
	}
	rollbackErr := rollback()
	if rollbackErr != nil {
		rollbackErr = fmt.Errorf("settings: roll back MCP definition mutation: %w", rollbackErr)
		return mcpDefinitionMutationCommitted, errors.Join(deleteStateErr, rollbackErr)
	}
	rollbackInvalidateErr := s.mcpAuth.MCPAuthInvalidate(target)
	if rollbackInvalidateErr != nil {
		rollbackInvalidateErr = fmt.Errorf(
			"settings: invalidate pending MCP auth after definition rollback: %w",
			rollbackInvalidateErr,
		)
	}
	return mcpDefinitionMutationRestored, errors.Join(deleteStateErr, rollbackInvalidateErr)
}

func (s *service) invalidateMCPAuthAfterDefinitionRestore(
	scope ScopeKind,
	workspaceID string,
	profileName string,
	name string,
) error {
	if s.mcpAuth == nil {
		return nil
	}
	target, err := normalizeMCPAuthTarget(MCPAuthTargetRequest{
		Scope: scope, WorkspaceID: workspaceID, ProfileName: profileName, Name: name,
	})
	if err != nil {
		return err
	}
	if err := s.mcpAuth.MCPAuthInvalidate(target); err != nil {
		return fmt.Errorf("settings: invalidate pending MCP auth after definition restore: %w", err)
	}
	return nil
}
