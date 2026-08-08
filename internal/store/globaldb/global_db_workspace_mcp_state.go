package globaldb

import (
	"context"
	"fmt"

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
	authSecretRefs := make([]string, 0, len(mcpAuthRefs)*2)
	for _, refs := range mcpAuthRefs {
		authSecretRefs = append(authSecretRefs, refs.AccessTokenRef, refs.RefreshTokenRef)
	}
	for _, ref := range uniqueMCPAuthorizationRefs(authSecretRefs...) {
		if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
			return fmt.Errorf(
				"store: delete MCP auth secret for workspace %q: %w",
				workspaceID,
				err,
			)
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
	registrationSecretRefs := make([]string, 0, len(mcpRegistrationRefs)*2)
	for _, refs := range mcpRegistrationRefs {
		registrationSecretRefs = append(
			registrationSecretRefs,
			refs.ClientSecretRef,
			refs.RegistrationAccessTokenRef,
		)
	}
	for _, ref := range uniqueMCPAuthorizationRefs(registrationSecretRefs...) {
		if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
			return fmt.Errorf(
				"store: delete MCP OAuth registration secret for workspace %q: %w",
				workspaceID,
				err,
			)
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
