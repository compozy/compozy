package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const hostAPIWorkspaceLiteral = "workspace"

func bindHostAPIParams(
	ctx context.Context,
	raw json.RawMessage,
	binding workspaceBinding,
	workspaceID string,
	workspaceRoot string,
	workspaces workspacepkg.RuntimeResolver,
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
		err = bindWorkspaceString(
			ctx,
			params,
			hostAPIWorkspaceLiteral,
			workspaceID,
			workspaceRoot,
			workspaces,
		)
	case workspaceBindingID:
		err = bindWorkspaceString(ctx, params, "workspace_id", workspaceID, workspaceID, workspaces)
	case workspaceBindingTask:
		err = errors.Join(
			bindString(params, "scope", hostAPIWorkspaceLiteral),
			bindWorkspaceString(
				ctx,
				params,
				hostAPIWorkspaceLiteral,
				workspaceID,
				workspaceRoot,
				workspaces,
			),
		)
	case workspaceBindingMemory:
		err = errors.Join(
			bindString(params, "scope", hostAPIWorkspaceLiteral),
			bindWorkspaceString(
				ctx,
				params,
				hostAPIWorkspaceLiteral,
				workspaceID,
				workspaceRoot,
				workspaces,
			),
		)
	case workspaceBindingResource:
		err = bindResourceScope(ctx, params, workspaceID, workspaces)
	case workspaceBindingAutomation:
		err = errors.Join(
			bindString(params, "scope", hostAPIWorkspaceLiteral),
			bindWorkspaceString(ctx, params, "workspace_id", workspaceID, workspaceID, workspaces),
		)
	case workspaceBindingActor:
		return raw, nil
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

func bindWorkspaceString(
	ctx context.Context,
	params map[string]any,
	key string,
	workspaceID string,
	canonical string,
	workspaces workspacepkg.RuntimeResolver,
) error {
	value, ok := params[key]
	if !ok || value == nil {
		params[key] = canonical
		return nil
	}
	provided, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a string", key)
	}
	provided = strings.TrimSpace(provided)
	if provided == "" || provided == canonical {
		params[key] = canonical
		return nil
	}
	if workspaces == nil {
		return fmt.Errorf("%s conflicts with the bound workspace", key)
	}
	resolved, err := workspaces.Resolve(ctx, provided)
	if err != nil {
		return fmt.Errorf("resolve %s %q: %w", key, provided, err)
	}
	if strings.TrimSpace(resolved.ID) != strings.TrimSpace(workspaceID) {
		return fmt.Errorf("%s conflicts with the bound workspace", key)
	}
	params[key] = canonical
	return nil
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

func bindResourceScope(
	ctx context.Context,
	params map[string]any,
	workspaceID string,
	workspaces workspacepkg.RuntimeResolver,
) error {
	workspaceScope := map[string]any{"kind": hostAPIWorkspaceLiteral, "id": workspaceID}
	if records, ok := params["records"].([]any); ok {
		for index, value := range records {
			record, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("records[%d] must be an object", index)
			}
			if err := validateResourceScope(ctx, record["scope"], workspaceID, workspaces); err != nil {
				return fmt.Errorf("records[%d].scope: %w", index, err)
			}
			record["scope"] = workspaceScope
		}
		return nil
	}
	if scope, ok := params["scope"]; ok && scope != nil {
		if err := validateResourceScope(ctx, scope, workspaceID, workspaces); err != nil {
			return err
		}
	}
	params["scope"] = workspaceScope
	return nil
}

func validateResourceScope(
	ctx context.Context,
	value any,
	workspaceID string,
	workspaces workspacepkg.RuntimeResolver,
) error {
	if value == nil {
		return nil
	}
	scope, ok := value.(map[string]any)
	if !ok {
		return errors.New("scope must be an object")
	}
	kind, kindOK := scope["kind"].(string)
	id, idOK := scope["id"].(string)
	if !kindOK || !idOK || strings.TrimSpace(kind) != hostAPIWorkspaceLiteral {
		return errors.New("scope conflicts with the bound workspace")
	}
	id = strings.TrimSpace(id)
	if id == workspaceID {
		return nil
	}
	if workspaces == nil {
		return errors.New("scope conflicts with the bound workspace")
	}
	resolved, err := workspaces.Resolve(ctx, id)
	if err != nil {
		return fmt.Errorf("resolve scope workspace %q: %w", id, err)
	}
	if strings.TrimSpace(resolved.ID) != strings.TrimSpace(workspaceID) {
		return errors.New("scope conflicts with the bound workspace")
	}
	return nil
}
