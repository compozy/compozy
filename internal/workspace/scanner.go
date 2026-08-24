package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/skillscan"
)

const (
	agentDefinitionFile = "AGENT.md"
	skillDefinitionFile = "SKILL.md"
)

type workspaceScan struct {
	snapshots           map[string]filesnap.Snapshot
	agents              []agentCandidate
	skills              []skillCandidate
	profileDeclarations []ProfileDeclaration
}

type agentCandidate struct {
	path   string
	source compozyconfig.WorkspaceDiscoverySource
}

type skillCandidate struct {
	name      string
	dir       string
	source    string
	rootOrder int
}

func (r *Resolver) scanWorkspace(ctx context.Context, ws Workspace, profileName string) (workspaceScan, error) {
	if err := checkContext(ctx); err != nil {
		return workspaceScan{}, err
	}

	scan := workspaceScan{
		snapshots:           make(map[string]filesnap.Snapshot),
		agents:              make([]agentCandidate, 0),
		skills:              make([]skillCandidate, 0),
		profileDeclarations: make([]ProfileDeclaration, 0),
	}
	if err := validatePersonalProfileRoot(ws.RootDir, r.homePaths.ProfilesDir, profileName); err != nil {
		return workspaceScan{}, err
	}

	if err := addSnapshotIfExists(r.homePaths.ConfigFile, scan.snapshots); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot global config %q: %w", r.homePaths.ConfigFile, err)
	}
	if err := addSnapshotIfExists(
		filepath.Join(r.homePaths.HomeDir, compozyconfig.MCPJSONName),
		scan.snapshots,
	); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot global MCP JSON %q: %w", r.homePaths.HomeDir, err)
	}
	if err := addSnapshotIfExists(
		filepath.Join(ws.RootDir, compozyconfig.DirName, compozyconfig.ConfigName),
		scan.snapshots,
	); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot workspace config %q: %w", ws.RootDir, err)
	}
	if err := addDependencySnapshot(compozyconfig.WorkspaceDotEnvFile(ws.RootDir), scan.snapshots); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot workspace dotenv %q: %w", ws.RootDir, err)
	}
	if err := addSnapshotIfExists(
		filepath.Join(ws.RootDir, compozyconfig.DirName, compozyconfig.MCPJSONName),
		scan.snapshots,
	); err != nil {
		return workspaceScan{}, fmt.Errorf("workspace: snapshot workspace MCP JSON %q: %w", ws.RootDir, err)
	}
	if trimmedProfile := strings.TrimSpace(profileName); trimmedProfile != "" {
		profileRoot := filepath.Join(r.homePaths.ProfilesDir, trimmedProfile)
		workspaceProfileRoot := filepath.Join(
			ws.RootDir, compozyconfig.DirName, compozyconfig.ProfilesDirName, trimmedProfile,
		)
		for _, path := range []string{
			filepath.Join(profileRoot, compozyconfig.ConfigName),
			filepath.Join(profileRoot, compozyconfig.MCPJSONName),
			filepath.Join(workspaceProfileRoot, compozyconfig.ConfigName),
			filepath.Join(workspaceProfileRoot, compozyconfig.MCPJSONName),
		} {
			if err := addSnapshotIfExists(path, scan.snapshots); err != nil {
				return workspaceScan{}, fmt.Errorf("workspace: snapshot profile config dependency %q: %w", path, err)
			}
		}
	}
	declarations, err := scanWorkspaceProfileDeclarations(ws.RootDir, scan.snapshots)
	if err != nil {
		return workspaceScan{}, err
	}
	scan.profileDeclarations = declarations

	for rootOrder, root := range compozyconfig.WorkspaceDiscoveryRoots(
		ws.RootDir,
		ws.AdditionalDirs,
		r.homePaths,
		profileName,
	) {
		if err := checkContext(ctx); err != nil {
			return workspaceScan{}, err
		}

		if err := scanAgentSource(root, scan.snapshots, &scan.agents); err != nil {
			return workspaceScan{}, err
		}
		if err := scanSkillSource(root, rootOrder, scan.snapshots, &scan.skills); err != nil {
			return workspaceScan{}, err
		}
	}

	return scan, nil
}

func validatePersonalProfileRoot(workspaceRoot, profilesRoot, profileName string) error {
	trimmedName := strings.TrimSpace(profileName)
	if trimmedName == "" {
		return nil
	}
	personalRoot := filepath.Join(profilesRoot, trimmedName)
	canonicalWorkspaceRoot, err := fileutil.CanonicalPathWithExistingPrefix(workspaceRoot)
	if err != nil {
		return fmt.Errorf("workspace: canonicalize workspace root: %w", err)
	}
	canonicalPersonalRoot, err := fileutil.CanonicalPathWithExistingPrefix(personalRoot)
	if err != nil {
		return fmt.Errorf("workspace: canonicalize personal profile root: %w", err)
	}
	contained, err := fileutil.PathWithinRoot(canonicalWorkspaceRoot, canonicalPersonalRoot)
	if err != nil {
		return fmt.Errorf("workspace: compare personal profile root with workspace: %w", err)
	}
	if contained {
		return fmt.Errorf(
			"workspace: personal profile resource root %q cannot be inside registered workspace %q",
			personalRoot,
			workspaceRoot,
		)
	}
	return nil
}

func scanAgentSource(
	root compozyconfig.WorkspaceDiscoveryRoot,
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
		if err := addSnapshotIfExists(filepath.Join(agentDir, compozyconfig.MCPJSONName), snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot agent MCP sidecar %q: %w", agentDir, err)
		}
		if err := addSnapshotIfExists(agentPath, snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot agent definition %q: %w", agentPath, err)
		}
		if _, ok := snapshots[agentPath]; !ok {
			continue
		}
		if compozyconfig.IsReservedAgentName(entry.Name()) {
			*dst = append(*dst, agentCandidate{path: agentPath, source: root.Source})
			continue
		}
		if err := scanAgentCapabilityCatalog(agentDir, snapshots); err != nil {
			return err
		}

		*dst = append(*dst, agentCandidate{
			path:   agentPath,
			source: root.Source,
		})
	}

	return nil
}

func scanAgentCapabilityCatalog(agentDir string, snapshots map[string]filesnap.Snapshot) error {
	paths, err := compozyconfig.AgentCapabilityCatalogDependencyPaths(agentDir)
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
	root compozyconfig.WorkspaceDiscoveryRoot,
	rootOrder int,
	snapshots map[string]filesnap.Snapshot,
	dst *[]skillCandidate,
) error {
	skillsDir := root.SkillsDir()
	if err := addSnapshotIfExists(skillsDir, snapshots); err != nil {
		return fmt.Errorf("workspace: snapshot skills directory %q: %w", skillsDir, err)
	}

	result, err := skillscan.ScanDirectory(skillsDir)
	if err != nil {
		return fmt.Errorf("workspace: scan skills directory %q: %w", skillsDir, err)
	}
	maps.Copy(snapshots, result.Snapshots)

	for _, skillFile := range result.Paths {
		skillDir := filepath.Dir(skillFile)
		skillName, err := loadWorkspaceSkillName(skillFile)
		if err != nil {
			skillName = filepath.Base(skillDir)
			slog.Warn(
				"workspace: falling back to skill directory name",
				"path",
				skillFile,
				"name",
				skillName,
				"error",
				err,
			)
		}

		if err := addSnapshotIfExists(skillDir, snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot skill directory %q: %w", skillDir, err)
		}
		if err := addSnapshotIfExists(filepath.Join(skillDir, compozyconfig.MCPJSONName), snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot skill MCP sidecar %q: %w", skillDir, err)
		}
		*dst = append(*dst, skillCandidate{
			name:      skillName,
			dir:       skillDir,
			source:    string(root.Source),
			rootOrder: rootOrder,
		})
	}

	return nil
}

func loadAgents(ctx context.Context, candidates []agentCandidate) ([]compozyconfig.AgentDef, []AgentDiagnostic, error) {
	if len(candidates) == 0 {
		return nil, nil, nil
	}

	agents := make([]compozyconfig.AgentDef, 0, len(candidates))
	diagnostics := make([]AgentDiagnostic, 0)
	seen := make(map[string]int, len(candidates))

	for _, candidate := range candidates {
		if err := checkContext(ctx); err != nil {
			return nil, nil, err
		}

		candidateName := filepath.Base(filepath.Dir(candidate.path))
		if compozyconfig.IsReservedAgentName(candidateName) {
			diagnostics = append(diagnostics, reservedAgentDiagnostic(candidate.path, candidateName))
			continue
		}

		agent, err := compozyconfig.LoadAgentDefFile(candidate.path)
		if err != nil {
			diagnostics = append(diagnostics, agentDiagnosticFromError(candidate.path, err))
			continue
		}
		if compozyconfig.IsReservedAgentName(agent.Name) {
			diagnostics = append(diagnostics, reservedAgentDiagnostic(candidate.path, agent.Name))
			continue
		}
		agent.SourceLayer = compozyconfig.AgentLayerName(candidate.source)

		if winnerIndex, ok := seen[agent.Name]; ok {
			agents[winnerIndex].ShadowedDefinitions = append(
				agents[winnerIndex].ShadowedDefinitions,
				compozyconfig.AgentDefinitionRef{Layer: agent.SourceLayer, Path: agent.SourcePath},
			)
			continue
		}

		seen[agent.Name] = len(agents)
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
	case errors.Is(err, compozyconfig.ErrMissingAgentFrontmatter):
		return "frontmatter.missing"
	case errors.Is(err, compozyconfig.ErrUnterminatedAgentFrontmatter):
		return "frontmatter.unterminated"
	case errors.Is(err, compozyconfig.ErrBOMAgentFrontmatter):
		return "frontmatter.bom"
	case errors.Is(err, compozyconfig.ErrInvalidAgentFrontmatterKey):
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
	type winner struct {
		index     int
		rootOrder int
	}
	winners := make(map[string]winner, len(candidates))

	for _, candidate := range candidates {
		current, exists := winners[candidate.name]
		if exists && current.rootOrder != candidate.rootOrder {
			continue
		}

		resolved := SkillPath{
			Name:   candidate.name,
			Dir:    candidate.dir,
			Source: candidate.source,
		}
		if exists {
			skills[current.index] = resolved
			continue
		}

		winners[candidate.name] = winner{index: len(skills), rootOrder: candidate.rootOrder}
		skills = append(skills, resolved)
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
