package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// GenerateJSON handles the key generation in JSON mode
func GenerateJSON(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	// Parse flags
	name, err := cobraCmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	description, err := cobraCmd.Flags().GetString("description")
	if err != nil {
		return fmt.Errorf("failed to get description flag: %w", err)
	}
	expiresStr, err := cobraCmd.Flags().GetString("expires")
	if err != nil {
		return fmt.Errorf("failed to get expires flag: %w", err)
	}
	// Validate expiration date if provided
	var expires *time.Time
	if expiresStr != "" {
		t, err := time.Parse("2006-01-02", expiresStr)
		if err != nil {
			return outputJSONError("invalid expiration date format, use YYYY-MM-DD")
		}
		expires = &t
	}
	log.Debug("generating API key in JSON mode",
		"name", name,
		"description", description,
		"expires", expiresStr)
	// Get the auth client from executor
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}
	// Generate the key
	req := &api.GenerateKeyRequest{
		Name:        name,
		Description: description,
	}
	if expires != nil {
		req.Expires = expires.Format("2006-01-02")
	}
	apiKey, err := authClient.GenerateKey(ctx, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to generate API key: %v", err))
	}
	// Prepare response
	response := map[string]any{
		"api_key": apiKey,
		"created": time.Now().Format(time.RFC3339),
	}
	if name != "" {
		response["name"] = name
	}
	if description != "" {
		response["description"] = description
	}
	if expires != nil {
		response["expires"] = expires.Format(time.RFC3339)
	}
	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}
	return nil
}
