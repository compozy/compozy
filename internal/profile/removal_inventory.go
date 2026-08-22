package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
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

func countProfileCredentialRows(ctx context.Context, q queryer, profile Profile) (int, error) {
	var count int
	if err := q.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM profile_credential_requirements WHERE profile_id = ?) +
			(SELECT COUNT(*) FROM vault_secrets WHERE ref LIKE ? OR ref LIKE ?)`,
		profile.ID,
		profileVaultRefPrefix(profile.Name)+"%",
		profileMCPVaultRefPrefix(profile.Name)+"%",
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("profile: count credential overrides: %w", err)
	}
	return count, nil
}

func profileVaultRefPrefix(profileName string) string {
	return "vault:profiles/" + strings.TrimSpace(profileName) + "/"
}

func profileMCPVaultRefPrefix(profileName string) string {
	return "vault:mcp/profile/" + strings.TrimSpace(profileName) + "/"
}
