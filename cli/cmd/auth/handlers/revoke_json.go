package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// RevokeJSON handles key revocation in JSON mode using the unified executor pattern

func RevokeJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, args []string) error {
	log := logger.FromContext(ctx)
	keyID, err := resolveKeyID(args)
	if err != nil {
		return outputJSONError(err.Error())
	}
	force, err := cobraCmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}
	log.Debug("revoking API key in JSON mode",
		"key_id", keyID,
		"force", force)
	if err := ensureForceRevocation(force); err != nil {
		return outputJSONError(err.Error())
	}
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}
	if err := authClient.RevokeKey(ctx, keyID); err != nil {
		return outputJSONError(fmt.Sprintf("failed to revoke API key: %v", err))
	}
	return writeRevokeResponse(keyID)
}

func resolveKeyID(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("key ID required in JSON mode")
	}
	id := strings.TrimSpace(args[0])
	if id == "" {
		return "", fmt.Errorf("key ID cannot be empty or whitespace")
	}
	return id, nil
}

func ensureForceRevocation(force bool) error {
	if force {
		return nil
	}
	return fmt.Errorf("revocation requires --force flag in JSON mode")
}

func writeRevokeResponse(keyID string) error {
	return writeJSONResponse(map[string]any{
		"data": map[string]any{
			"key_id":  keyID,
			"revoked": time.Now().UTC().Format(time.RFC3339),
		},
		"message": "Success",
	})
}
