package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// listParams holds the parsed command line parameters
type listParams struct {
	sortBy string
	filter string
	page   int
	limit  int
}

// ListJSON handles key listing in JSON mode using the unified executor pattern
func ListJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	log := logger.FromContext(ctx)
	authClient := executor.GetAuthClient()

	params, err := parseListParams(cobraCmd)
	if err != nil {
		return err
	}

	log.Debug("listing API keys in JSON mode",
		"sort", params.sortBy,
		"filter", params.filter,
		"page", params.page,
		"limit", params.limit)

	keys, err := authClient.ListKeys(ctx)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to list API keys: %v", err))
	}

	keys = applyFiltering(keys, params.filter)
	keys = applySorting(keys, params.sortBy)
	keys, totalKeys := applyPagination(keys, params.page, params.limit)

	response := buildResponse(keys, totalKeys, params)
	return outputJSONResponse(response)
}

// parseListParams extracts and validates command line parameters
func parseListParams(cmd *cobra.Command) (*listParams, error) {
	sortBy, err := cmd.Flags().GetString("sort")
	if err != nil {
		return nil, fmt.Errorf("failed to get sort flag: %w", err)
	}
	filter, err := cmd.Flags().GetString("filter")
	if err != nil {
		return nil, fmt.Errorf("failed to get filter flag: %w", err)
	}
	page, err := cmd.Flags().GetInt("page")
	if err != nil {
		return nil, fmt.Errorf("failed to get page flag: %w", err)
	}
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return nil, fmt.Errorf("failed to get limit flag: %w", err)
	}
	return &listParams{
		sortBy: sortBy,
		filter: filter,
		page:   page,
		limit:  limit,
	}, nil
}

// applyFiltering filters keys based on the filter string
func applyFiltering(keys []api.KeyInfo, filter string) []api.KeyInfo {
	if filter == "" {
		return keys
	}
	filtered := make([]api.KeyInfo, 0, len(keys))
	for _, key := range keys {
		if contains(key.Prefix, filter) || contains(key.ID, filter) {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

// applySorting sorts keys using the sorting package
func applySorting(keys []api.KeyInfo, sortBy string) []api.KeyInfo {
	keyPtrs := make([]*api.KeyInfo, len(keys))
	for i := range keys {
		keyPtrs[i] = &keys[i]
	}
	cliutils.SortKeys(keyPtrs, sortBy)
	for i, ptr := range keyPtrs {
		keys[i] = *ptr
	}
	return keys
}

// applyPagination applies pagination to the keys
func applyPagination(keys []api.KeyInfo, page, limit int) ([]api.KeyInfo, int) {
	totalKeys := len(keys)
	startIdx := (page - 1) * limit
	endIdx := startIdx + limit
	if startIdx >= totalKeys {
		return []api.KeyInfo{}, totalKeys
	}
	if endIdx > totalKeys {
		endIdx = totalKeys
	}
	return keys[startIdx:endIdx], totalKeys
}

// buildResponse creates the JSON response structure
func buildResponse(keys []api.KeyInfo, totalKeys int, params *listParams) map[string]any {
	pages := 0
	if params.limit > 0 {
		pages = (totalKeys + params.limit - 1) / params.limit
	}
	return map[string]any{
		"keys":  keys,
		"total": totalKeys,
		"page":  params.page,
		"limit": params.limit,
		"pages": pages,
	}
}

// outputJSONResponse outputs the response as JSON
func outputJSONResponse(response map[string]any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("failed to encode JSON response: %w", err)
	}
	return nil
}
