package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	userEnablementStateFileName      = ".compozy-state.json"
	workspaceEnablementStateFileName = "workspace-extensions.json"
)

var osUserHomeDir = os.UserHomeDir

// Ref identifies an extension instance for local enablement resolution.
type Ref struct {
	Name          string
	Source        Source
	WorkspaceRoot string
}

// EnablementState captures the resolved local enabled state for an extension.
type EnablementState struct {
	Extension Ref
	Enabled   bool
}

// EnablementStore persists operator-local enablement choices outside the repository.
type EnablementStore struct {
	homeDir string
}

// NewEnablementStore constructs a store rooted at homeDir or the current user's home.
func NewEnablementStore(ctx context.Context, homeDir string) (*EnablementStore, error) {
	if err := contextError(ctx, "create enablement store"); err != nil {
		return nil, err
	}

	resolvedHome := strings.TrimSpace(homeDir)
	if resolvedHome == "" {
		value, err := osUserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		resolvedHome = strings.TrimSpace(value)
	}
	if resolvedHome == "" {
		return nil, fmt.Errorf("create enablement store: home directory is empty")
	}

	return &EnablementStore{homeDir: filepath.Clean(resolvedHome)}, nil
}

// Enabled resolves whether an extension is enabled for this machine.
func (s *EnablementStore) Enabled(ctx context.Context, ref Ref) (bool, error) {
	state, err := s.Load(ctx, ref)
	if err != nil {
		return false, err
	}
	return state.Enabled, nil
}

// Load reads the stored enablement state for an extension or returns the default policy.
func (s *EnablementStore) Load(ctx context.Context, ref Ref) (EnablementState, error) {
	if err := contextError(ctx, "load extension enablement"); err != nil {
		return EnablementState{}, err
	}
	if s == nil {
		return EnablementState{}, fmt.Errorf("load extension enablement: store is nil")
	}
	if err := validateExtensionRef(ref); err != nil {
		return EnablementState{}, err
	}

	state := EnablementState{
		Extension: ref,
		Enabled:   defaultEnabled(ref.Source),
	}

	switch ref.Source {
	case SourceBundled:
		return state, nil
	case SourceUser:
		path := s.userStatePath(ref.Name)
		record, err := loadUserEnablementRecord(path)
		if err != nil {
			return EnablementState{}, err
		}
		if record == nil {
			return state, nil
		}
		state.Enabled = record.Enabled
		return state, nil
	case SourceWorkspace:
		path := s.workspaceStatePath()
		record, err := loadWorkspaceEnablementRecord(path)
		if err != nil {
			return EnablementState{}, err
		}
		if record == nil {
			return state, nil
		}

		workspaceRoot, err := normalizeWorkspaceRoot(ref.WorkspaceRoot)
		if err != nil {
			return EnablementState{}, err
		}
		if names, ok := record.Workspaces[workspaceRoot]; ok {
			if enabled, ok := names[ref.Name]; ok {
				state.Enabled = enabled
			}
		}
		return state, nil
	default:
		return EnablementState{}, fmt.Errorf("load extension enablement: unsupported source %q", ref.Source)
	}
}

// Save persists the provided enablement state.
func (s *EnablementStore) Save(ctx context.Context, state EnablementState) error {
	if err := contextError(ctx, "save extension enablement"); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("save extension enablement: store is nil")
	}
	if err := validateExtensionRef(state.Extension); err != nil {
		return err
	}

	switch state.Extension.Source {
	case SourceBundled:
		return fmt.Errorf("save extension enablement: bundled extensions are always enabled")
	case SourceUser:
		return s.saveUserState(state)
	case SourceWorkspace:
		return s.saveWorkspaceState(state)
	default:
		return fmt.Errorf("save extension enablement: unsupported source %q", state.Extension.Source)
	}
}

// Enable marks an extension as enabled in the local store.
func (s *EnablementStore) Enable(ctx context.Context, ref Ref) error {
	return s.Save(ctx, EnablementState{
		Extension: ref,
		Enabled:   true,
	})
}

// Disable marks an extension as disabled in the local store.
func (s *EnablementStore) Disable(ctx context.Context, ref Ref) error {
	return s.Save(ctx, EnablementState{
		Extension: ref,
		Enabled:   false,
	})
}

func (s *EnablementStore) userStatePath(name string) string {
	return filepath.Join(s.homeDir, ".compozy", "extensions", name, userEnablementStateFileName)
}

func (s *EnablementStore) workspaceStatePath() string {
	return filepath.Join(s.homeDir, ".compozy", "state", workspaceEnablementStateFileName)
}

func defaultEnabled(source Source) bool {
	return source == SourceBundled
}

func validateExtensionRef(ref Ref) error {
	if strings.TrimSpace(ref.Name) == "" {
		return fmt.Errorf("extension reference name is required")
	}

	switch ref.Source {
	case SourceBundled, SourceUser:
		return nil
	case SourceWorkspace:
		if strings.TrimSpace(ref.WorkspaceRoot) == "" {
			return fmt.Errorf("workspace extension reference requires workspace root")
		}
		return nil
	default:
		return fmt.Errorf("unsupported extension source %q", ref.Source)
	}
}

func (s *EnablementStore) saveUserState(state EnablementState) error {
	path := s.userStatePath(state.Extension.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create enablement state directory %q: %w", filepath.Dir(path), err)
	}

	payload, err := json.MarshalIndent(userEnablementRecord{Enabled: state.Enabled}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal user enablement state: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write user enablement state %q: %w", path, err)
	}
	return nil
}

func (s *EnablementStore) saveWorkspaceState(state EnablementState) error {
	workspaceRoot, err := normalizeWorkspaceRoot(state.Extension.WorkspaceRoot)
	if err != nil {
		return err
	}

	path := s.workspaceStatePath()
	record, err := loadWorkspaceEnablementRecord(path)
	if err != nil {
		return err
	}
	if record == nil {
		record = &workspaceEnablementRecord{Workspaces: make(map[string]map[string]bool)}
	}
	if record.Workspaces == nil {
		record.Workspaces = make(map[string]map[string]bool)
	}
	if record.Workspaces[workspaceRoot] == nil {
		record.Workspaces[workspaceRoot] = make(map[string]bool)
	}
	record.Workspaces[workspaceRoot][state.Extension.Name] = state.Enabled

	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workspace enablement state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create workspace enablement state directory %q: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write workspace enablement state %q: %w", path, err)
	}
	return nil
}

func normalizeWorkspaceRoot(root string) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", fmt.Errorf("workspace root is empty")
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root %q: %w", trimmed, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return filepath.Clean(absPath), nil
		}
		return "", fmt.Errorf("resolve workspace root symlinks %q: %w", absPath, err)
	}
	return filepath.Clean(resolvedPath), nil
}

type userEnablementRecord struct {
	Enabled bool `json:"enabled"`
}

func loadUserEnablementRecord(path string) (*userEnablementRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read user enablement state %q: %w", path, err)
	}

	var record userEnablementRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode user enablement state %q: %w", path, err)
	}
	return &record, nil
}

type workspaceEnablementRecord struct {
	Workspaces map[string]map[string]bool `json:"workspaces"`
}

func loadWorkspaceEnablementRecord(path string) (*workspaceEnablementRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workspace enablement state %q: %w", path, err)
	}

	var record workspaceEnablementRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode workspace enablement state %q: %w", path, err)
	}
	return &record, nil
}
