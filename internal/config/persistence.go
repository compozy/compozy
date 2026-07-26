package config

import (
	"errors"
	"fmt"

	"strings"
)

const (
	persistenceValueKey = "value"
)

var (
	// ErrUnsupportedTOMLMutation reports a mutation that would require rewriting
	// unrelated TOML structure instead of editing the targeted document fragment.
	ErrUnsupportedTOMLMutation = errors.New("config: unsupported TOML mutation")
)

// WriteScope identifies the config scope a write should target.
type WriteScope string

const (
	// WriteScopeGlobal targets the global AGH home config.
	WriteScopeGlobal WriteScope = "global"
	// WriteScopeWorkspace targets a workspace-local AGH overlay.
	WriteScopeWorkspace WriteScope = "workspace"
)

// Validate ensures the write scope is supported.
func (s WriteScope) Validate() error {
	switch s {
	case WriteScopeGlobal, WriteScopeWorkspace:
		return nil
	default:
		return fmt.Errorf("config: invalid write scope %q", s)
	}
}

// WriteTargetKind describes the canonical persistence destination without
// exposing filesystem paths to higher layers.
type WriteTargetKind string

const (
	// WriteTargetGlobalConfig writes `~/.agh/config.toml`.
	WriteTargetGlobalConfig WriteTargetKind = "global-config"
	// WriteTargetWorkspaceConfig writes `<workspace>/.agh/config.toml`.
	WriteTargetWorkspaceConfig WriteTargetKind = "workspace-config"
	// WriteTargetGlobalMCPSidecar writes `~/.agh/mcp.json`.
	WriteTargetGlobalMCPSidecar WriteTargetKind = "global-mcp-sidecar"
	// WriteTargetWorkspaceMCPSidecar writes `<workspace>/.agh/mcp.json`.
	WriteTargetWorkspaceMCPSidecar WriteTargetKind = "workspace-mcp-sidecar"
)

// WriteTarget captures a semantic destination while keeping the on-disk path
// internal to the config package.
type WriteTarget struct {
	kind          WriteTargetKind
	scope         WriteScope
	path          string
	workspaceRoot string
}

// Kind returns the semantic destination identifier for the write target.
func (t WriteTarget) Kind() WriteTargetKind {
	return t.kind
}

// Scope returns the write scope for the target.
func (t WriteTarget) Scope() WriteScope {
	return t.scope
}

// Path returns the resolved filesystem path for operator-facing diagnostics and tools.
func (t WriteTarget) Path() string {
	return t.path
}

func (t WriteTarget) isConfigTarget() bool {
	return t.kind == WriteTargetGlobalConfig || t.kind == WriteTargetWorkspaceConfig
}

func (t WriteTarget) isMCPSidecarTarget() bool {
	return t.kind == WriteTargetGlobalMCPSidecar || t.kind == WriteTargetWorkspaceMCPSidecar
}

// ResolveConfigWriteTarget resolves the canonical config overlay destination for
// the requested scope.
func ResolveConfigWriteTarget(homePaths HomePaths, workspaceRoot string, scope WriteScope) (WriteTarget, error) {
	return resolveWriteTarget(homePaths, workspaceRoot, scope, false)
}

// ResolveMCPSidecarWriteTarget resolves the canonical MCP sidecar destination
// for the requested scope.
func ResolveMCPSidecarWriteTarget(homePaths HomePaths, workspaceRoot string, scope WriteScope) (WriteTarget, error) {
	return resolveWriteTarget(homePaths, workspaceRoot, scope, true)
}

func resolveWriteTarget(
	homePaths HomePaths,
	workspaceRoot string,
	scope WriteScope,
	sidecar bool,
) (WriteTarget, error) {
	if err := scope.Validate(); err != nil {
		return WriteTarget{}, err
	}

	switch scope {
	case WriteScopeGlobal:
		if sidecar {
			return WriteTarget{
				kind:  WriteTargetGlobalMCPSidecar,
				scope: scope,
				path:  globalMCPJSONFile(homePaths),
			}, nil
		}
		return WriteTarget{
			kind:  WriteTargetGlobalConfig,
			scope: scope,
			path:  homePaths.ConfigFile,
		}, nil
	case WriteScopeWorkspace:
		resolvedRoot, err := resolveWorkspaceRoot(workspaceRoot)
		if err != nil {
			return WriteTarget{}, err
		}
		if strings.TrimSpace(resolvedRoot) == "" {
			return WriteTarget{}, errors.New("config: workspace write target requires a workspace root")
		}
		if sidecar {
			return WriteTarget{
				kind:          WriteTargetWorkspaceMCPSidecar,
				scope:         scope,
				path:          workspaceMCPJSONFile(resolvedRoot),
				workspaceRoot: resolvedRoot,
			}, nil
		}
		return WriteTarget{
			kind:          WriteTargetWorkspaceConfig,
			scope:         scope,
			path:          workspaceConfigFile(resolvedRoot),
			workspaceRoot: resolvedRoot,
		}, nil
	default:
		return WriteTarget{}, fmt.Errorf("config: invalid write scope %q", scope)
	}
}

// OverlayEditor applies safe, comment-preserving mutations to one TOML overlay
// document.
type OverlayEditor struct {
	content []byte
	source  string
}

// SetValue updates or creates one scalar or array value at the provided path.
func (e *OverlayEditor) SetValue(path []string, value any) error {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}

	normalized, err := normalizeTOMLValue(value)
	if err != nil {
		return fmt.Errorf("config: set TOML value %q: %w", strings.Join(cleanPath, "."), err)
	}

	updated, err := setValueInOverlayDocument(e.content, cleanPath, normalized)
	if err != nil {
		return err
	}
	e.content = updated
	return nil
}

// SetTable replaces or creates a TOML table at the provided path.
func (e *OverlayEditor) SetTable(path []string, values map[string]any) error {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return unsupportedTOMLMutation(cleanPath, "table replacement requires at least one key")
	}

	updated, err := setTableInOverlayDocument(e.content, cleanPath, values)
	if err != nil {
		return fmt.Errorf("config: set TOML table %q: %w", strings.Join(cleanPath, "."), err)
	}
	e.content = updated
	return nil
}

// UpsertArrayTableItem replaces or appends one named entry in an array-of-tables.
func (e *OverlayEditor) UpsertArrayTableItem(
	path []string,
	nameField string,
	name string,
	values map[string]any,
) error {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}
	field := strings.TrimSpace(nameField)
	if field == "" {
		return errors.New("config: array-table name field is required")
	}
	key := strings.TrimSpace(name)
	if key == "" {
		return errors.New("config: array-table item name is required")
	}

	itemValues := cloneStringAnyMap(values)
	itemValues[field] = key

	updated, err := upsertArrayTableItemInOverlayDocument(e.content, cleanPath, field, key, itemValues)
	if err != nil {
		return fmt.Errorf("config: set TOML array-table item %q: %w", strings.Join(cleanPath, "."), err)
	}
	e.content = updated
	return nil
}

// Delete removes one TOML key path when present.
func (e *OverlayEditor) Delete(path []string) error {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return err
	}
	updated, _, err := deletePathInOverlayDocument(e.content, cleanPath)
	if err != nil {
		return err
	}
	e.content = updated
	return nil
}

// DeleteArrayTableItem removes one named entry from an array-of-tables.
func (e *OverlayEditor) DeleteArrayTableItem(path []string, nameField string, name string) (bool, error) {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return false, err
	}
	field := strings.TrimSpace(nameField)
	if field == "" {
		return false, errors.New("config: array-table name field is required")
	}
	key := strings.TrimSpace(name)
	if key == "" {
		return false, errors.New("config: array-table item name is required")
	}

	updated, deleted, err := deleteArrayTableItemInOverlayDocument(e.content, cleanPath, field, key)
	if err != nil {
		return false, err
	}
	e.content = updated
	return deleted, nil
}

// HasPath reports whether the current document already contains the given path.
func (e *OverlayEditor) HasPath(path []string) bool {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return false
	}
	document, err := parseOverlayDocument(e.content)
	if err != nil {
		return false
	}
	return document.findKeyValue(cleanPath) != nil ||
		document.findTable(cleanPath) != nil ||
		len(document.arrayTableBlocks(cleanPath)) > 0
}

func (e *OverlayEditor) Bytes() ([]byte, error) {
	return append([]byte(nil), e.content...), nil
}

func newOverlayEditor(path string, contents []byte) (*OverlayEditor, error) {
	source := strings.TrimSpace(path)
	if source == "" {
		source = ConfigName
	}
	if _, err := parseOverlayDocument(contents); err != nil {
		return nil, fmt.Errorf("config: parse config overlay %q: %w", source, err)
	}
	return &OverlayEditor{content: append([]byte(nil), contents...), source: source}, nil
}

// EditConfigOverlay applies one validated mutation to a canonical TOML overlay
// target and returns the merged effective config after the write.
func EditConfigOverlay(
	homePaths HomePaths,
	workspaceRoot string,
	target WriteTarget,
	mutate func(*OverlayEditor) error,
) (Config, error) {
	if !target.isConfigTarget() {
		return Config{}, fmt.Errorf("config: write target %q is not a config overlay", target.Kind())
	}
	if mutate == nil {
		return Config{}, errors.New("config: config overlay mutation is required")
	}

	contents, _, err := readOptionalRegularFile(target.path, "config overlay")
	if err != nil {
		return Config{}, err
	}

	editor, err := newOverlayEditor(target.path, contents)
	if err != nil {
		return Config{}, err
	}
	if err := mutate(editor); err != nil {
		return Config{}, err
	}

	rendered, err := editor.Bytes()
	if err != nil {
		return Config{}, err
	}

	finalCfg, err := validateEffectiveConfigWrite(homePaths, workspaceRoot, target, rendered)
	if err != nil {
		return Config{}, err
	}
	if err := writePersistedFile(target.path, rendered); err != nil {
		return Config{}, err
	}
	return finalCfg, nil
}
