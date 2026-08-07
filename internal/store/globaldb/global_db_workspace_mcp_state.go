package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func deleteWorkspaceMCPState(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID string,
) error {
	mcpAuthRefs, err := queries.ListMCPAuthTokenRefsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("store: list MCP auth token refs for workspace %q: %w", workspaceID, err)
	}
	for _, refs := range mcpAuthRefs {
		for _, ref := range []string{refs.AccessTokenRef, refs.RefreshTokenRef} {
			if strings.TrimSpace(ref) == "" {
				continue
			}
			if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
				return fmt.Errorf(
					"store: delete MCP auth secret for workspace %q: %w",
					workspaceID,
					err,
				)
			}
		}
	}
	if _, err := queries.DeleteMCPAuthTokensByWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf("store: delete MCP auth tokens for workspace %q: %w", workspaceID, err)
	}
	mcpRegistrationRefs, err := queries.ListMCPOAuthRegistrationRefsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf(
			"store: list MCP OAuth registration refs for workspace %q: %w",
			workspaceID,
			err,
		)
	}
	for _, refs := range mcpRegistrationRefs {
		for _, ref := range []string{refs.ClientSecretRef, refs.RegistrationAccessTokenRef} {
			if strings.TrimSpace(ref) == "" {
				continue
			}
			if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
				return fmt.Errorf(
					"store: delete MCP OAuth registration secret for workspace %q: %w",
					workspaceID,
					err,
				)
			}
		}
	}
	if _, err := queries.DeleteMCPOAuthRegistrationsByWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf(
			"store: delete MCP OAuth registrations for workspace %q: %w",
			workspaceID,
			err,
		)
	}
	return nil
}
