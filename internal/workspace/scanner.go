package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/filesnap"
)

const (
	agentDefinitionFile = "AGENT.md"
	skillDefinitionFile = "SKILL.md"
)

type workspaceScan struct {
	snapshots map[string]filesnap.Snapshot
	agents    []agentCandidate
	skills    []skillCandidate
}

type agentCandidate struct {
	path string
}

type skillCandidate struct {
	name   string
	dir    string
	source string
}

func (r *Resolver) scanWorkspace(ctx context.Context, ws Workspace) (workspaceScan, error) {
	if err := checkContext(ctx); err != nil {
		return workspaceScan{}, err
	}

	scan := workspaceScan{
		snapshots: make(map[string]filesnap.Snapshot),
		agents:    make([]agentCandidate, 0),
		skills:    make([]skillCandidate, 0),
	}

	if err := addSnapshotIfExists(r.homePaths.ConfigFile, scan.snapshots); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot global config %q: %w", r.homePaths.ConfigFile, err)
	}
	if err := addSnapshotIfExists(
		filepath.Join(r.homePaths.HomeDir, aghconfig.MCPJSONName),
		scan.snapshots,
	); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot global MCP JSON %q: %w", r.homePaths.HomeDir, err)
	}
	if err := addSnapshotIfExists(
		filepath.Join(ws.RootDir, aghconfig.DirName, aghconfig.ConfigName),
		scan.snapshots,
	); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot workspace config %q: %w", ws.RootDir, err)
	}
	if err := addDependencySnapshot(aghconfig.WorkspaceDotEnvFile(ws.RootDir), scan.snapshots); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot workspace dotenv %q: %w", ws.RootDir, err)
	}
	if err := addSnapshotIfExists(
		filepath.Join(ws.RootDir, aghconfig.DirName, aghconfig.MCPJSONName),
		scan.snapshots,
	); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot workspace MCP JSON %q: %w", ws.RootDir, err)
	}

	for _, root := range aghconfig.WorkspaceDiscoveryRoots(ws.RootDir, ws.AdditionalDirs, r.homePaths) {
		if err := checkContext(ctx); err != nil {
			return workspaceScan{}, err
		}

		if err := scanAgentSource(root, scan.snapshots, &scan.agents); err != nil {
			return workspaceScan{}, err
		}
		if err := scanSkillSource(root, scan.snapshots, &scan.skills); err != nil {
			return workspaceScan{}, err
		}
	}

	return scan, nil
}

func scanAgentSource(
	root aghconfig.WorkspaceDiscoveryRoot,
	snapshots map[string]filesnap.Snapshot,
	dst *[]agentCandidate,
) error {
	agentsDir := root.AgentsDir()
	if err := addSnapshotIfExists(agentsDir, snapshots); err != nil {
		return fmt.Errorf("workspace: snapshot agents directory %q: %w", agentsDir, err)
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("workspace: read agents directory %q: %w", agentsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agentDir := filepath.Join(agentsDir, entry.Name())
		agentPath := filepath.Join(agentsDir, entry.Name(), agentDefinitionFile)
		if err := addSnapshotIfExists(filepath.Join(agentDir, aghconfig.MCPJSONName), snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot agent MCP sidecar %q: %w", agentDir, err)
		}
		if err := addSnapshotIfExists(agentPath, snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot agent definition %q: %w", agentPath, err)
		}
		if _, ok := snapshots[agentPath]; !ok {
			continue
		}
		if aghconfig.IsReservedAgentName(entry.Name()) {
			*dst = append(*dst, agentCandidate{path: agentPath})
			continue
		}
		if err := scanAgentCapabilityCatalog(agentDir, snapshots); err != nil {
			return err
		}

		*dst = append(*dst, agentCandidate{
			path: agentPath,
		})
	}

	return nil
}

func scanAgentCapabilityCatalog(agentDir string, snapshots map[string]filesnap.Snapshot) error {
	paths, err := aghconfig.AgentCapabilityCatalogDependencyPaths(agentDir)
	if err != nil {
		return fmt.Errorf("workspace: enumerate agent capability catalog %q: %w", agentDir, err)
	}
	for _, path := range paths {
		if err := addDependencySnapshot(path, snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot agent capability catalog %q: %w", path, err)
		}
	}
	return nil
}

func scanSkillSource(
	root aghconfig.WorkspaceDiscoveryRoot,
	snapshots map[string]filesnap.Snapshot,
	dst *[]skillCandidate,
) error {
	skillsDir := root.SkillsDir()
	if err := addSnapshotIfExists(skillsDir, snapshots); err != nil {
		return fmt.Errorf("workspace: snapshot skills directory %q: %w", skillsDir, err)
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("workspace: read skills directory %q: %w", skillsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(skillsDir, entry.Name())
		skillFile := filepath.Join(skillDir, skillDefinitionFile)
		if err := addSnapshotIfExists(skillDir, snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot skill directory %q: %w", skillDir, err)
		}
		if err := addSnapshotIfExists(filepath.Join(skillDir, aghconfig.MCPJSONName), snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot skill MCP sidecar %q: %w", skillDir, err)
		}
		if err := addSnapshotIfExists(skillFile, snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot skill definition %q: %w", skillFile, err)
		}
		if _, ok := snapshots[skillFile]; !ok {
			continue
		}

		*dst = append(*dst, skillCandidate{
			name:   entry.Name(),
			dir:    skillDir,
			source: string(root.Source),
		})
	}

	return nil
}

func loadAgents(ctx context.Context, candidates []agentCandidate) ([]aghconfig.AgentDef, []AgentDiagnostic, error) {
	if len(candidates) == 0 {
		return nil, nil, nil
	}

	agents := make([]aghconfig.AgentDef, 0, len(candidates))
	diagnostics := make([]AgentDiagnostic, 0)
	seen := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		if err := checkContext(ctx); err != nil {
			return nil, nil, err
		}

		candidateName := filepath.Base(filepath.Dir(candidate.path))
		if aghconfig.IsReservedAgentName(candidateName) {
			diagnostics = append(diagnostics, reservedAgentDiagnostic(candidate.path, candidateName))
			continue
		}

		agent, err := aghconfig.LoadAgentDefFile(candidate.path)
		if err != nil {
			diagnostics = append(diagnostics, agentDiagnosticFromError(candidate.path, err))
			continue
		}
		if aghconfig.IsReservedAgentName(agent.Name) {
			diagnostics = append(diagnostics, reservedAgentDiagnostic(candidate.path, agent.Name))
			continue
		}

		if _, ok := seen[agent.Name]; ok {
			continue
		}

		seen[agent.Name] = struct{}{}
		agents = append(agents, agent)
	}

	return agents, diagnostics, nil
}

const reservedAgentDiagnosticKind = "agent.name_reserved"

func reservedAgentDiagnostic(path string, name string) AgentDiagnostic {
	return AgentDiagnostic{
		Name:      name,
		Path:      filepath.Clean(path),
		ErrorKind: reservedAgentDiagnosticKind,
		Message:   fmt.Sprintf("workspace: skipped reserved agent %q definition %q", name, path),
	}
}

func agentDiagnosticFromError(path string, err error) AgentDiagnostic {
	return AgentDiagnostic{
		Name:      filepath.Base(filepath.Dir(path)),
		Path:      filepath.Clean(path),
		ErrorKind: agentDiagnosticKind(err),
		Message:   err.Error(),
	}
}

func agentDiagnosticKind(err error) string {
	switch {
	case errors.Is(err, aghconfig.ErrMissingAgentFrontmatter):
		return "frontmatter.missing"
	case errors.Is(err, aghconfig.ErrUnterminatedAgentFrontmatter):
		return "frontmatter.unterminated"
	case errors.Is(err, aghconfig.ErrBOMAgentFrontmatter):
		return "frontmatter.bom"
	case errors.Is(err, aghconfig.ErrInvalidAgentFrontmatterKey):
		return "frontmatter.invalid_key"
	default:
		return "frontmatter.invalid"
	}
}

func mergeSkillPaths(candidates []skillCandidate) []SkillPath {
	if len(candidates) == 0 {
		return nil
	}

	skills := make([]SkillPath, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		if _, ok := seen[candidate.name]; ok {
			continue
		}

		seen[candidate.name] = struct{}{}
		skills = append(skills, SkillPath{
			Dir:    candidate.dir,
			Source: candidate.source,
		})
	}

	return skills
}

func addSnapshotIfExists(path string, snapshots map[string]filesnap.Snapshot) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	snapshot, err := filesnap.FromPath(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	snapshots[path] = snapshot
	return nil
}

func addDependencySnapshot(path string, snapshots map[string]filesnap.Snapshot) error {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return nil
	}

	snapshot, err := filesnap.FromPath(normalized)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			snapshots[normalized] = filesnap.Snapshot{Size: -1}
			return nil
		}
		return err
	}

	snapshots[normalized] = snapshot
	return nil
}
