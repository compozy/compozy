package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const hostAPIWorkspaceLiteral = "workspace"

func bindHostAPIParams(
	raw json.RawMessage,
	binding workspaceBinding,
	workspaceID string,
	workspaceRoot string,
) (json.RawMessage, error) {
	params := make(map[string]any)
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" && trimmed != jsonNullLiteral {
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, fmt.Errorf("params must be an object: %w", err)
		}
	}

	var err error
	switch binding {
	case workspaceBindingPath:
		err = bindString(params, hostAPIWorkspaceLiteral, workspaceRoot)
	case workspaceBindingID:
		err = bindString(params, "workspace_id", workspaceID)
	case workspaceBindingTask:
		err = errors.Join(
			bindString(params, "scope", hostAPIWorkspaceLiteral),
			bindString(params, hostAPIWorkspaceLiteral, workspaceRoot),
		)
	case workspaceBindingMemory:
		err = errors.Join(
			bindString(params, "scope", hostAPIWorkspaceLiteral),
			bindString(params, hostAPIWorkspaceLiteral, workspaceRoot),
		)
	case workspaceBindingResource:
		err = bindResourceScope(params, workspaceID)
	case workspaceBindingNone:
		return nil, errors.New("projected method has no workspace binding")
	default:
		return nil, fmt.Errorf("unknown workspace binding %d", binding)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(params)
}

func bindString(params map[string]any, key string, canonical string) error {
	if value, ok := params[key]; ok && value != nil {
		provided, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", key)
		}
		provided = strings.TrimSpace(provided)
		if provided != "" && provided != canonical {
			return fmt.Errorf("%s conflicts with the bound workspace", key)
		}
	}
	params[key] = canonical
	return nil
}

func bindResourceScope(params map[string]any, workspaceID string) error {
	workspaceScope := map[string]any{"kind": hostAPIWorkspaceLiteral, "id": workspaceID}
	if records, ok := params["records"].([]any); ok {
		for index, value := range records {
			record, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("records[%d] must be an object", index)
			}
			if err := validateResourceScope(record["scope"], workspaceID); err != nil {
				return fmt.Errorf("records[%d].scope: %w", index, err)
			}
			record["scope"] = workspaceScope
		}
		return nil
	}
	if scope, ok := params["scope"]; ok && scope != nil {
		if err := validateResourceScope(scope, workspaceID); err != nil {
			return err
		}
	}
	params["scope"] = workspaceScope
	return nil
}

func validateResourceScope(value any, workspaceID string) error {
	if value == nil {
		return nil
	}
	scope, ok := value.(map[string]any)
	if !ok {
		return errors.New("scope must be an object")
	}
	kind, kindOK := scope["kind"].(string)
	id, idOK := scope["id"].(string)
	if !kindOK || !idOK ||
		strings.TrimSpace(kind) != hostAPIWorkspaceLiteral || strings.TrimSpace(id) != workspaceID {
		return errors.New("scope conflicts with the bound workspace")
	}
	return nil
}
