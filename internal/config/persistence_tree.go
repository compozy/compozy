package config

import (
	"errors"
	"fmt"
	"strings"

	tomltree "github.com/pelletier/go-toml"
)

func validateEffectiveConfigWrite(
	homePaths HomePaths,
	workspaceRoot string,
	target WriteTarget,
	rendered []byte,
) (Config, error) {
	dotenvLookup, err := loadDotEnvLookup(workspaceRoot)
	if err != nil {
		return Config{}, err
	}
	lookup := layeredEnvLookup(processEnvLookup, dotenvLookup)

	cfg := DefaultWithHome(homePaths)

	globalOverlay, err := loadConfigOverlayForWrite(homePaths.ConfigFile, target, rendered)
	if err != nil {
		return Config{}, err
	}
	if err := applyConfigOverlay(&cfg, &globalOverlay, RoleFieldSourceGlobal); err != nil {
		return Config{}, fmt.Errorf("apply global config overlay: %w", err)
	}
	if err := applyConfigMCPSidecarContent(globalMCPJSONFile(homePaths), target, rendered, &cfg); err != nil {
		return Config{}, fmt.Errorf("load global MCP JSON: %w", err)
	}

	resolvedWorkspaceRoot, err := resolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return Config{}, err
	}
	if err := applyWorkspaceConfigWrite(resolvedWorkspaceRoot, target, rendered, &cfg); err != nil {
		return Config{}, err
	}

	if err := normalizeConfigPaths(&cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.validateWithEnv(lookup); err != nil {
		return Config{}, fmt.Errorf("validate config write for %q: %w", target.Kind(), err)
	}
	return cfg, nil
}

func loadConfigOverlayForWrite(path string, target WriteTarget, rendered []byte) (configOverlay, error) {
	if target.isConfigTarget() && samePath(target.path, path) {
		return loadConfigOverlayBytes(rendered, path)
	}
	return loadConfigOverlayFile(path)
}

func applyConfigMCPSidecarContent(path string, target WriteTarget, rendered []byte, cfg *Config) error {
	if cfg == nil {
		return errors.New("config: config is required")
	}

	var (
		servers []MCPServer
		err     error
	)
	if target.isMCPSidecarTarget() && samePath(target.path, path) {
		servers, err = parseOptionalMCPServersJSON(rendered, path)
	} else {
		servers, err = LoadMCPServersJSONFile(path)
	}
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return nil
	}

	cfg.MCPServers = OverrideMCPServers(cfg.MCPServers, servers)
	return nil
}

func parseOptionalMCPServersJSON(content []byte, source string) ([]MCPServer, error) {
	if strings.TrimSpace(string(content)) == "" {
		return nil, nil
	}
	return ParseMCPServersJSON(content, source)
}

func newEmptyTree() (*tomltree.Tree, error) {
	tree, err := tomltree.TreeFromMap(map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("config: create empty TOML tree: %w", err)
	}
	return tree, nil
}

func newTreeFromMap(values map[string]any) (*tomltree.Tree, error) {
	normalized, err := normalizeTreeValue(values)
	if err != nil {
		return nil, err
	}

	tree, err := tomltree.TreeFromMap(normalized)
	if err != nil {
		return nil, fmt.Errorf("config: create TOML tree: %w", err)
	}
	return tree, nil
}

func normalizeTreeValue(values map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(values))
	for _, key := range sortedStringKeys(values) {
		value, err := normalizeTreeEntry(key, values[key])
		if err != nil {
			return nil, err
		}
		normalized[key] = value
	}
	return normalized, nil
}

func normalizeTreeEntry(key string, value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeTreeValue(typed)
	case map[string]string:
		return normalizeTreeValue(stringMapToAny(typed))
	case []map[string]any:
		if len(typed) == 0 {
			return []any{}, nil
		}
		items := make([]map[string]any, 0, len(typed))
		for idx := range typed {
			normalized, err := normalizeTreeValue(typed[idx])
			if err != nil {
				return nil, fmt.Errorf("key %q array-table item %d: %w", key, idx, err)
			}
			items = append(items, normalized)
		}
		return items, nil
	case []map[string]string:
		if len(typed) == 0 {
			return []any{}, nil
		}
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, stringMapToAny(item))
		}
		return normalizeTreeEntry(key, items)
	case []any:
		return normalizeUntypedTreeSlice(key, typed)
	default:
		normalized, err := normalizeTOMLValue(typed)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		return normalized, nil
	}
}

func normalizeUntypedTreeSlice(key string, values []any) (any, error) {
	if len(values) == 0 {
		return []any{}, nil
	}

	tables := make([]map[string]any, 0, len(values))
	for index, value := range values {
		table, ok := value.(map[string]any)
		if !ok {
			if len(tables) > 0 {
				return nil, fmt.Errorf("key %q contains mixed scalar and table array values", key)
			}
			return normalizeTOMLValue(values)
		}
		normalized, err := normalizeTreeValue(table)
		if err != nil {
			return nil, fmt.Errorf("key %q array-table item %d: %w", key, index, err)
		}
		tables = append(tables, normalized)
	}
	return tables, nil
}

func normalizeMutationPath(path []string) ([]string, error) {
	if len(path) == 0 {
		return nil, errors.New("config: TOML mutation path is required")
	}

	clean := make([]string, 0, len(path))
	for _, element := range path {
		trimmed := strings.TrimSpace(element)
		if trimmed == "" {
			return nil, errors.New("config: TOML mutation path contains an empty segment")
		}
		clean = append(clean, trimmed)
	}
	return clean, nil
}

func normalizeTOMLValue(value any) (any, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return typed, nil
	case nil:
		return nil, errors.New("nil TOML values are not supported")
	case map[string]any, map[string]string, []map[string]any, []map[string]string:
		return nil, errors.New("tables and array-of-tables must use table helpers")
	default:
		if normalized, ok := normalizeIntegerValue(typed); ok {
			return normalized, nil
		}
		if normalized, ok := normalizeUnsignedValue(typed); ok {
			return normalized, nil
		}
		if normalized, ok := normalizeFloatValue(typed); ok {
			return normalized, nil
		}
		if normalized, ok, err := normalizeSliceValue(typed); ok || err != nil {
			return normalized, err
		}
		return nil, fmt.Errorf("unsupported TOML value type %T", value)
	}
}

func normalizeIntegerValue(value any) (any, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	default:
		return nil, false
	}
}

func normalizeUnsignedValue(value any) (any, bool) {
	switch typed := value.(type) {
	case uint:
		return uint64(typed), true
	case uint8:
		return uint64(typed), true
	case uint16:
		return uint64(typed), true
	case uint32:
		return uint64(typed), true
	case uint64:
		return typed, true
	default:
		return nil, false
	}
}

func normalizeFloatValue(value any) (any, bool) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return nil, false
	}
}

func normalizeSliceValue(value any) (any, bool, error) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), true, nil
	case []bool:
		return append([]bool(nil), typed...), true, nil
	case []int:
		values := make([]int64, 0, len(typed))
		for _, item := range typed {
			values = append(values, int64(item))
		}
		return values, true, nil
	case []int64:
		return append([]int64(nil), typed...), true, nil
	case []uint64:
		return append([]uint64(nil), typed...), true, nil
	case []float64:
		return append([]float64(nil), typed...), true, nil
	case []any:
		values := make([]any, 0, len(typed))
		for _, item := range typed {
			normalized, err := normalizeTOMLValue(item)
			if err != nil {
				return nil, true, err
			}
			switch normalized.(type) {
			case map[string]any, []map[string]any, []map[string]string:
				return nil, true, errors.New("inline tables and array-of-tables must use table helpers")
			}
			values = append(values, normalized)
		}
		return values, true, nil
	default:
		return nil, false, nil
	}
}
