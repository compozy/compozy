package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/compozy/compozy/cli/auth/sorting"
	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

func runGenerateJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)

	// Parse flags
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return fmt.Errorf("failed to get name flag: %w", err)
	}
	description, err := cmd.Flags().GetString("description")
	if err != nil {
		return fmt.Errorf("failed to get description flag: %w", err)
	}
	expiresStr, err := cmd.Flags().GetString("expires")
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

	// Generate the key
	req := &GenerateKeyRequest{
		Name:        name,
		Description: description,
	}
	if expires != nil {
		req.Expires = expires.Format("2006-01-02")
	}

	apiKey, err := client.GenerateKey(ctx, req)
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

func runListJSON(ctx context.Context, cmd *cobra.Command, client *Client) error {
	log := logger.FromContext(ctx)
	// Parse flags
	sortBy, err := cmd.Flags().GetString("sort")
	if err != nil {
		return fmt.Errorf("failed to get sort flag: %w", err)
	}
	filter, err := cmd.Flags().GetString("filter")
	if err != nil {
		return fmt.Errorf("failed to get filter flag: %w", err)
	}
	page, err := cmd.Flags().GetInt("page")
	if err != nil {
		return fmt.Errorf("failed to get page flag: %w", err)
	}
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return fmt.Errorf("failed to get limit flag: %w", err)
	}
	log.Debug("listing API keys in JSON mode",
		"sort", sortBy,
		"filter", filter,
		"page", page,
		"limit", limit)
	// Get the keys from the API
	keys, err := client.ListKeys(ctx)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to list API keys: %v", err))
	}
	// Apply client-side filtering if needed
	if filter != "" {
		filtered := make([]models.KeyInfo, 0)
		for _, key := range keys {
			// Filter by prefix or ID
			if contains(key.Prefix, filter) || contains(key.ID, filter) {
				filtered = append(filtered, key)
			}
		}
		keys = filtered
	}
	// Apply client-side sorting
	sorting.SortKeys(keys, sortBy)
	// Apply client-side pagination
	totalKeys := len(keys)
	startIdx := (page - 1) * limit
	endIdx := startIdx + limit
	if startIdx >= totalKeys {
		// Return empty result if page is out of bounds
		keys = []models.KeyInfo{}
	} else {
		if endIdx > totalKeys {
			endIdx = totalKeys
		}
		keys = keys[startIdx:endIdx]
	}
	// Prepare response with pagination metadata
	response := map[string]any{
		"keys":  keys,
		"total": totalKeys,
		"page":  page,
		"limit": limit,
		"pages": func() int {
			if limit == 0 {
				return 0
			}
			return (totalKeys + limit - 1) / limit
		}(),
	}
	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}
	return nil
}

// runRevokeJSON handles JSON mode for key revocation

func runRevokeJSON(ctx context.Context, cmd *cobra.Command, client *Client, keyID string) error {
	log := logger.FromContext(ctx)

	// Get force flag
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}

	log.Debug("revoking API key in JSON mode",
		"key_id", keyID,
		"force", force)

	// If not forced, we should show a warning (in a real implementation,
	// we'd show affected resources)
	if !force {
		return outputJSONError("revocation requires --force flag in JSON mode")
	}

	// Revoke the key
	err = client.RevokeKey(ctx, keyID)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to revoke API key: %v", err))
	}

	// Prepare response
	response := map[string]any{
		"message": "API key revoked successfully",
		"key_id":  keyID,
		"revoked": time.Now().Format(time.RFC3339),
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}

	return nil
}
