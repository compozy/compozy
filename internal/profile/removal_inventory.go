package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/vault"
	tomltree "github.com/pelletier/go-toml"
)

func profileFileRemovalSummary(profileDir string) (RemovalSummary, error) {
	var summary RemovalSummary
	var err error
	summary.Agents, err = countFiles(filepath.Join(profileDir, compozyconfig.AgentsDirName))
	if err != nil {
		return RemovalSummary{}, err
	}
	summary.Skills, err = countFiles(filepath.Join(profileDir, compozyconfig.SkillsDirName))
	if err != nil {
		return RemovalSummary{}, err
	}
	summary.Loops, err = countFiles(filepath.Join(profileDir, compozyconfig.LoopsDirName))
	if err != nil {
		return RemovalSummary{}, err
	}
	summary.MemoryEntries, err = countFiles(filepath.Join(profileDir, compozyconfig.MemoryDirName))
	if err != nil {
		return RemovalSummary{}, err
	}
	servers, err := compozyconfig.LoadMCPServersJSONFile(filepath.Join(profileDir, compozyconfig.MCPJSONName))
	if err != nil {
		return RemovalSummary{}, fmt.Errorf("profile: inventory MCP sidecar: %w", err)
	}
	summary.MCPServers = len(servers)
	summary.ConfigKeys, err = countProfileConfigKeys(filepath.Join(profileDir, compozyconfig.ConfigName))
	if err != nil {
		return RemovalSummary{}, err
	}
	return summary, nil
}

func countProfileConfigKeys(path string) (int, error) {
	tree, err := tomltree.LoadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("profile: inventory config %q: %w", path, err)
	}
	return countTOMLLeaves(tree), nil
}

func countTOMLLeaves(tree *tomltree.Tree) int {
	if tree == nil {
		return 0
	}
	count := 0
	for _, key := range tree.Keys() {
		switch value := tree.Get(key).(type) {
		case *tomltree.Tree:
			count += countTOMLLeaves(value)
		case []*tomltree.Tree:
			for _, item := range value {
				count += countTOMLLeaves(item)
			}
		default:
			count++
		}
	}
	return count
}

// countDesktopPartitions reports the stored window arrangements a profile owns.
// Without a desktop catalog wired the daemon has no window state at all, so zero
// is the truth rather than a fallback.
func (m *Manager) countDesktopPartitions(ctx context.Context, profileID string) (int, error) {
	if m.desktops == nil {
		return 0, nil
	}
	count, err := m.desktops.CountDesktopPartitions(ctx, profileID)
	if err != nil {
		return 0, fmt.Errorf("profile: count desktop partitions: %w", err)
	}
	return count, nil
}

func countProfileCredentialRows(ctx context.Context, q queryer, profile Profile) (int, error) {
	prefixes, err := vault.ListProfileSecretPrefixes(ctx, q, profile.Name)
	if err != nil {
		return 0, err
	}
	var count int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM profile_credential_requirements WHERE profile_id = ?`, profile.ID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("profile: count credential requirements: %w", err)
	}
	for _, prefix := range prefixes {
		var secrets int
		if err := q.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM vault_secrets WHERE SUBSTR(ref, 1, LENGTH(?)) = ?`, prefix, prefix,
		).Scan(&secrets); err != nil {
			return 0, fmt.Errorf("profile: count credential overrides: %w", err)
		}
		count += secrets
	}
	return count, nil
}
