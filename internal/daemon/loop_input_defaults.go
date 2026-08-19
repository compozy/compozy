package daemon

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
)

func (s *daemonLoopAPIService) GetLoopInputDefaults(
	ctx context.Context,
	workspaceID string,
	name string,
	scope contract.LoopInputDefaultsScope,
) (contract.LoopInputDefaultsResponse, error) {
	ws, loopName, err := validateLoopInputDefaultsTarget(workspaceID, name)
	if err != nil {
		return contract.LoopInputDefaultsResponse{}, err
	}
	values, err := s.loadLoopInputDefaults(ctx, ws, loopName, scope)
	if err != nil {
		return contract.LoopInputDefaultsResponse{}, err
	}
	return contract.LoopInputDefaultsResponse{LoopName: loopName, Scope: scope, Values: values}, nil
}

func (s *daemonLoopAPIService) GetLoopInputDefault(
	ctx context.Context,
	workspaceID string,
	name string,
	key string,
	scope contract.LoopInputDefaultsScope,
) (contract.LoopInputDefaultResponse, error) {
	inputKey, err := validateLoopInputDefaultKey(key)
	if err != nil {
		return contract.LoopInputDefaultResponse{}, err
	}
	response, err := s.GetLoopInputDefaults(ctx, workspaceID, name, scope)
	if err != nil {
		return contract.LoopInputDefaultResponse{}, err
	}
	value, present := response.Values[inputKey]
	return contract.LoopInputDefaultResponse{
		LoopName: response.LoopName,
		Key:      inputKey,
		Scope:    response.Scope,
		Present:  present,
		Value:    value,
	}, nil
}

func (s *daemonLoopAPIService) PutLoopInputDefaults(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.PutLoopInputDefaultsRequest,
) (contract.LoopInputDefaultsResponse, error) {
	ws, loopName, err := validateLoopInputDefaultsTarget(workspaceID, name)
	if err != nil {
		return contract.LoopInputDefaultsResponse{}, err
	}
	values, err := normalizeLoopInputDefaultValues(req.Values)
	if err != nil {
		return contract.LoopInputDefaultsResponse{}, err
	}
	if err := s.validateLoopInputDefaultLayer(ctx, ws, loopName, req.Scope, values); err != nil {
		return contract.LoopInputDefaultsResponse{}, err
	}
	root, target, err := s.loopInputDefaultsWriteTarget(ctx, ws, req.Scope)
	if err != nil {
		return contract.LoopInputDefaultsResponse{}, err
	}
	path := []string{compozyconfig.LoopsConfigKey, loopInputsKey, loopName}
	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		root,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if len(values) == 0 {
				return editor.Delete(path)
			}
			return editor.SetTable(path, values)
		},
	); err != nil {
		return contract.LoopInputDefaultsResponse{}, fmt.Errorf("daemon: replace Loop input defaults: %w", err)
	}
	return s.GetLoopInputDefaults(ctx, workspaceID, loopName, req.Scope)
}

func (s *daemonLoopAPIService) PutLoopInputDefault(
	ctx context.Context,
	workspaceID string,
	name string,
	key string,
	req contract.PutLoopInputDefaultRequest,
) (contract.LoopInputDefaultResponse, error) {
	ws, loopName, err := validateLoopInputDefaultsTarget(workspaceID, name)
	if err != nil {
		return contract.LoopInputDefaultResponse{}, err
	}
	inputKey, err := validateLoopInputDefaultKey(key)
	if err != nil {
		return contract.LoopInputDefaultResponse{}, err
	}
	value, err := normalizeLoopInputDefaultValue(req.Value)
	if err != nil {
		return contract.LoopInputDefaultResponse{}, fmt.Errorf(
			"%w: input default %q: %v",
			looppkg.ErrValidation,
			inputKey,
			err,
		)
	}
	if err := s.validateLoopInputDefaultLayer(
		ctx, ws, loopName, req.Scope, map[string]any{inputKey: value},
	); err != nil {
		return contract.LoopInputDefaultResponse{}, err
	}
	root, target, err := s.loopInputDefaultsWriteTarget(ctx, ws, req.Scope)
	if err != nil {
		return contract.LoopInputDefaultResponse{}, err
	}
	path := []string{compozyconfig.LoopsConfigKey, loopInputsKey, loopName, inputKey}
	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		root,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if table, ok := value.(map[string]any); ok {
				return editor.SetTable(path, table)
			}
			return editor.SetValue(path, value)
		},
	); err != nil {
		return contract.LoopInputDefaultResponse{}, fmt.Errorf("daemon: set Loop input default: %w", err)
	}
	return s.GetLoopInputDefault(ctx, workspaceID, loopName, inputKey, req.Scope)
}

func (s *daemonLoopAPIService) DeleteLoopInputDefault(
	ctx context.Context,
	workspaceID string,
	name string,
	key string,
	scope contract.LoopInputDefaultsScope,
) (contract.DeleteLoopInputDefaultResponse, error) {
	ws, loopName, err := validateLoopInputDefaultsTarget(workspaceID, name)
	if err != nil {
		return contract.DeleteLoopInputDefaultResponse{}, err
	}
	inputKey, err := validateLoopInputDefaultKey(key)
	if err != nil {
		return contract.DeleteLoopInputDefaultResponse{}, err
	}
	root, target, err := s.loopInputDefaultsWriteTarget(ctx, ws, scope)
	if err != nil {
		return contract.DeleteLoopInputDefaultResponse{}, err
	}
	path := []string{compozyconfig.LoopsConfigKey, loopInputsKey, loopName, inputKey}
	deleted := false
	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		root,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			deleted = editor.HasPath(path)
			return editor.Delete(path)
		},
	); err != nil {
		return contract.DeleteLoopInputDefaultResponse{}, fmt.Errorf("daemon: delete Loop input default: %w", err)
	}
	return contract.DeleteLoopInputDefaultResponse{
		LoopName: loopName,
		Key:      inputKey,
		Scope:    scope,
		Deleted:  deleted,
	}, nil
}

func (s *daemonLoopAPIService) loadLoopInputDefaults(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	loopName string,
	scope contract.LoopInputDefaultsScope,
) (map[string]any, error) {
	writeScope, err := loopInputDefaultsWriteScope(scope)
	if err != nil {
		return nil, err
	}
	root := ""
	options := []compozyconfig.LoadOption{}
	if writeScope == compozyconfig.WriteScopeWorkspace {
		root, err = s.loopInputDefaultsWorkspaceRoot(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		options = append(options, compozyconfig.WithWorkspaceRoot(root))
	}
	cfg, err := compozyconfig.LoadForHome(s.homePaths, options...)
	if err != nil {
		return nil, fmt.Errorf("daemon: load Loop input defaults: %w", err)
	}
	global, workspace := cfg.Loops.InputDefaultLayers(loopName)
	if writeScope == compozyconfig.WriteScopeWorkspace {
		return workspace, nil
	}
	return global, nil
}

func (s *daemonLoopAPIService) loopInputDefaultsWriteTarget(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	scope contract.LoopInputDefaultsScope,
) (string, compozyconfig.WriteTarget, error) {
	writeScope, err := loopInputDefaultsWriteScope(scope)
	if err != nil {
		return "", compozyconfig.WriteTarget{}, err
	}
	root := ""
	if writeScope == compozyconfig.WriteScopeWorkspace {
		root, err = s.loopInputDefaultsWorkspaceRoot(ctx, workspaceID)
		if err != nil {
			return "", compozyconfig.WriteTarget{}, err
		}
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, root, writeScope)
	if err != nil {
		return "", compozyconfig.WriteTarget{}, fmt.Errorf("daemon: resolve Loop input defaults target: %w", err)
	}
	return root, target, nil
}

func (s *daemonLoopAPIService) loopInputDefaultsWorkspaceRoot(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
) (string, error) {
	if s.workspaceResolver == nil {
		return "", fmt.Errorf("%w: workspace resolver is required", looppkg.ErrValidation)
	}
	workspace, err := s.workspaceResolver.Get(ctx, string(workspaceID))
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(workspace.RootDir)
	if root == "" {
		return "", fmt.Errorf("%w: workspace root is required", looppkg.ErrValidation)
	}
	return root, nil
}

func loopInputDefaultsWriteScope(scope contract.LoopInputDefaultsScope) (compozyconfig.WriteScope, error) {
	switch scope {
	case contract.LoopInputDefaultsScopeGlobal:
		return compozyconfig.WriteScopeGlobal, nil
	case contract.LoopInputDefaultsScopeWorkspace:
		return compozyconfig.WriteScopeWorkspace, nil
	default:
		return "", fmt.Errorf("%w: input-default scope must be global or workspace", looppkg.ErrValidation)
	}
}

func validateLoopInputDefaultsTarget(workspaceID string, name string) (looppkg.WorkspaceID, string, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return "", "", err
	}
	loopName, err := looppkg.ValidateName(name)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", looppkg.ErrValidation, err)
	}
	return ws, loopName, nil
}

func validateLoopInputDefaultKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" || strings.ContainsAny(key, "./") {
		return "", fmt.Errorf("%w: input-default key must be one nonempty path segment", looppkg.ErrValidation)
	}
	return key, nil
}

func normalizeLoopInputDefaultValues(values map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, rawKey := range keys {
		key, err := validateLoopInputDefaultKey(rawKey)
		if err != nil {
			return nil, err
		}
		value, err := normalizeLoopInputDefaultValue(values[rawKey])
		if err != nil {
			return nil, fmt.Errorf("%w: input default %q: %v", looppkg.ErrValidation, key, err)
		}
		result[key] = value
	}
	return result, nil
}

func normalizeLoopInputDefaultValue(value any) (any, error) {
	if _, ok := value.(map[string]any); ok {
		return compozyconfig.NormalizeToolConfigValue(compozyconfig.ConfigValueTable, value)
	}
	return compozyconfig.NormalizeToolConfigValue(compozyconfig.ConfigValueScalar, value)
}

func (s *daemonLoopAPIService) validateLoopInputDefaultLayer(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	loopName string,
	scope contract.LoopInputDefaultsScope,
	values map[string]any,
) error {
	if s.resolver == nil {
		return fmt.Errorf("%w: Loop definition resolver is required", looppkg.ErrValidation)
	}
	resolved, err := s.resolver.ResolveLoop(ctx, workspaceID, loopName)
	if err != nil {
		return err
	}
	if resolved == nil {
		return fmt.Errorf("%w: Loop definition %q resolved empty", looppkg.ErrValidation, loopName)
	}
	origin := looppkg.InputOriginGlobal
	if scope == contract.LoopInputDefaultsScopeWorkspace {
		origin = looppkg.InputOriginWorkspace
	}
	return looppkg.ValidateInputLayer(resolved.Definition, loopName, values, origin)
}
