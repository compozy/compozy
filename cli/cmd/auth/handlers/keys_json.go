package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

const (
	dateFormat = "2006-01-02"
)

// GenerateJSON handles the key generation in JSON mode
func GenerateJSON(
	ctx context.Context,
	cobraCmd *cobra.Command,
	executor *cmd.CommandExecutor,
	_ []string,
) error {
	log := logger.FromContext(ctx)
	req, err := parseGenerateKeyFlags(cobraCmd)
	if err != nil {
		return outputJSONError(err.Error())
	}
	log.Debug("generating API key in JSON mode",
		"name", req.Name,
		"description", req.Description,
		"expires", req.Expires)
	authClient := executor.GetAuthClient()
	if authClient == nil {
		return outputJSONError("auth client not available")
	}
	apiKey, err := authClient.GenerateKey(ctx, req)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to generate API key: %v", err))
	}
	response := buildGenerateKeyResponse(apiKey, req)
	return outputJSONResponse(response)
}

// parseGenerateKeyFlags extracts and validates flags for key generation
func parseGenerateKeyFlags(cobraCmd *cobra.Command) (*api.GenerateKeyRequest, error) {
	name, err := cobraCmd.Flags().GetString("name")
	if err != nil {
		return nil, fmt.Errorf("failed to get name flag: %w", err)
	}
	description, err := cobraCmd.Flags().GetString("description")
	if err != nil {
		return nil, fmt.Errorf("failed to get description flag: %w", err)
	}
	expiresStr, err := cobraCmd.Flags().GetString("expires")
	if err != nil {
		return nil, fmt.Errorf("failed to get expires flag: %w", err)
	}
	req := &api.GenerateKeyRequest{
		Name:        name,
		Description: description,
	}
	if expiresStr != "" {
		if _, err := time.Parse(dateFormat, expiresStr); err != nil {
			return nil, fmt.Errorf("invalid expiration date format, use YYYY-MM-DD")
		}
		req.Expires = expiresStr
	}
	return req, nil
}

// buildGenerateKeyResponse constructs the JSON response for key generation
func buildGenerateKeyResponse(apiKey string, req *api.GenerateKeyRequest) map[string]any {
	data := map[string]any{
		"api_key": apiKey,
		// Note: Using current time as the API doesn't return creation timestamp
		// This is a limitation of the current API design
		"created": time.Now().Format(time.RFC3339),
	}
	if req.Name != "" {
		data["name"] = req.Name
	}
	if req.Description != "" {
		data["description"] = req.Description
	}
	if req.Expires != "" {
		if expires, err := time.Parse(dateFormat, req.Expires); err == nil {
			data["expires"] = expires.Format(time.RFC3339)
		}
	}
	return map[string]any{
		"data":    data,
		"message": "Success",
	}
}
