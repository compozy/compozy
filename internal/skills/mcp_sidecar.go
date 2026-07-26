package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

func mergeSkillMCPSidecarFile(dir string, skill *Skill) error {
	if skill == nil {
		return errors.New("skills: skill is required")
	}

	sidecarPath := filepath.Join(strings.TrimSpace(dir), aghconfig.MCPJSONName)
	if _, err := os.Lstat(sidecarPath); err == nil {
		if err := ensurePathWithinRoot(dir, sidecarPath); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("skills: stat MCP sidecar %q: %w", sidecarPath, err)
	}
	servers, err := aghconfig.LoadMCPServersJSONFile(sidecarPath)
	if err != nil {
		return err
	}

	skill.MCPServers = overrideSkillMCPServers(skill.MCPServers, toMCPServerDecls(servers))
	return nil
}

func mergeSkillMCPSidecarFS(fsys fs.FS, dir string, skill *Skill) error {
	if skill == nil {
		return errors.New("skills: skill is required")
	}

	sidecarPath := aghconfig.MCPJSONName
	if trimmed := strings.TrimSpace(dir); trimmed != "" {
		sidecarPath = path.Join(trimmed, aghconfig.MCPJSONName)
	}

	content, err := fs.ReadFile(fsys, sidecarPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("skills: read MCP sidecar %q: %w", sidecarPath, err)
	}

	servers, err := aghconfig.ParseMCPServersJSON(content, sidecarPath)
	if err != nil {
		return err
	}

	skill.MCPServers = overrideSkillMCPServers(skill.MCPServers, toMCPServerDecls(servers))
	return nil
}

func overrideSkillMCPServers(base []MCPServerDecl, overlay []MCPServerDecl) []MCPServerDecl {
	merged := cloneMCPServerDecls(base)
	index := make(map[string]int, len(merged))
	for i, server := range merged {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		index[name] = i
	}

	for _, server := range overlay {
		name := strings.TrimSpace(server.Name)
		if idx, ok := index[name]; ok && name != "" {
			merged[idx] = cloneMCPServerDecl(server)
			continue
		}

		merged = append(merged, cloneMCPServerDecl(server))
		if name != "" {
			index[name] = len(merged) - 1
		}
	}

	return merged
}

func toMCPServerDecls(servers []aghconfig.MCPServer) []MCPServerDecl {
	if len(servers) == 0 {
		return nil
	}

	decls := make([]MCPServerDecl, 0, len(servers))
	for _, server := range servers {
		decls = append(decls, MCPServerDecl{
			Name:      server.Name,
			Command:   server.Command,
			Args:      append([]string(nil), server.Args...),
			Env:       cloneStringMap(server.Env),
			SecretEnv: cloneStringMap(server.SecretEnv),
		})
	}
	return decls
}

func cloneMCPServerDecl(decl MCPServerDecl) MCPServerDecl {
	return MCPServerDecl{
		Name:      decl.Name,
		Command:   decl.Command,
		Args:      append([]string(nil), decl.Args...),
		Env:       cloneStringMap(decl.Env),
		SecretEnv: cloneStringMap(decl.SecretEnv),
	}
}
