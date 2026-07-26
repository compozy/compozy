package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const marketplaceCursorVersion = 2

type marketplaceCursor struct {
	Version     int    `json:"v"`
	Kind        string `json:"kind"`
	Query       string `json:"query,omitempty"`
	Scope       string `json:"scope"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Fence       string `json:"fence"`
	Offset      int    `json:"offset"`
}

func marketplaceNextCursor(
	kind string,
	query string,
	scope marketplaceReadScope,
	fence string,
	nextOffset int,
	hasMore bool,
) (string, error) {
	if !hasMore {
		return "", nil
	}
	payload, err := json.Marshal(marketplaceCursor{
		Version: marketplaceCursorVersion, Kind: kind, Query: strings.TrimSpace(query),
		Scope: string(scope.scope), WorkspaceID: scope.workspaceID, Fence: fence, Offset: nextOffset,
	})
	if err != nil {
		return "", fmt.Errorf("marketplace: encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func marketplaceCursorOffset(
	raw string,
	kind string,
	query string,
	scope marketplaceReadScope,
) (int, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, "", nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return 0, "", marketplaceValidationf("cursor is invalid")
	}
	var cursor marketplaceCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return 0, "", marketplaceValidationf("cursor is invalid")
	}
	if cursor.Version != marketplaceCursorVersion || cursor.Offset < 0 || cursor.Fence == "" ||
		cursor.Kind != kind || cursor.Query != strings.TrimSpace(query) ||
		cursor.Scope != string(scope.scope) || cursor.WorkspaceID != scope.workspaceID {
		return 0, "", marketplaceValidationf("cursor does not match the marketplace request")
	}
	if cursor.Offset == 0 {
		return 0, "", fmt.Errorf("%w: cursor offset must be positive", ErrMarketplaceValidation)
	}
	return cursor.Offset, cursor.Fence, nil
}

func validateMarketplaceCursorFence(expected string, actual string) error {
	if expected == "" || expected == actual {
		return nil
	}
	return marketplaceValidationf("cursor catalog revision has changed; restart from the first page")
}
