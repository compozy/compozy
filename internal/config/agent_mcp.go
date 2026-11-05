package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/vault"
)

// ResolveAgentMCPReferences binds agent references against the current configured servers.
func ResolveAgentMCPReferences(servers, configured []MCPServer) []MCPServer {
	result := make([]MCPServer, 0, len(servers))
	index := indexMCPServersByName(configured)
	for _, server := range servers {
		if !server.Reference {
			result = append(result, cloneMCPServer(server))
			continue
		}
		at, ok := index[normalizeMCPServerName(server.Name)]
		if ok && !configured[at].Reference {
			result = append(result, cloneMCPServer(configured[at]))
		}
	}
	return result
}

// AgentMCPReferenceBroken reports whether a name reference has no configured target.
func AgentMCPReferenceBroken(server MCPServer, configured []MCPServer) bool {
	return server.Reference && len(ResolveAgentMCPReferences([]MCPServer{server}, configured)) == 0
}

// prepareAgentMCPSidecar validates requested additions without rewriting unrelated sidecar data.
func prepareAgentMCPSidecar(path string, content []byte, additions []MCPServer) ([]byte, []MCPServer, error) {
	if err := validateUniqueMCPJSONServerNames(additions, path); err != nil {
		return nil, nil, errors.Join(ErrInvalidAgentDefinition, err)
	}
	doc, err := loadEditableMCPJSONDocument(content, path)
	if err != nil {
		return nil, nil, err
	}
	for i, server := range additions {
		server.Name = normalizeMCPServerName(server.Name)
		if err := server.Validate(fmt.Sprintf("agent.mcp_servers[%d]", i)); err != nil {
			return nil, nil, errors.Join(ErrInvalidAgentDefinition, err)
		}
		if err := validateAgentMCPSecretRefs(server); err != nil {
			return nil, nil, errors.Join(ErrInvalidAgentDefinition, err)
		}
		if len(server.Headers) > 0 {
			return nil, nil, fmt.Errorf(
				"%w: agent header credentials require secret_headers Vault references",
				ErrInvalidAgentDefinition,
			)
		}
		if err := doc.Put(server); err != nil {
			return nil, nil, err
		}
	}
	rendered, err := doc.Bytes()
	if err != nil {
		return nil, nil, err
	}
	servers, err := parseMCPServersJSON(rendered, path, true)
	return rendered, servers, err
}

// CreateAgentDefWithMCPServers co-ships an authored definition and additive sidecar entries.
func CreateAgentDefWithMCPServers(
	path string,
	draft AgentDefinitionDraft,
	overwrite bool,
	additions []MCPServer,
) (AgentDef, error) {
	return createAgentDefWithMCPServers(
		path,
		draft,
		overwrite,
		additions,
		CreateAgentDefFile,
		writePersistedFileInDirectory,
	)
}

// A non-empty result on error means credential references may remain on disk.
func createAgentDefWithMCPServers(
	path string,
	draft AgentDefinitionDraft,
	overwrite bool,
	additions []MCPServer,
	writeDefinition func(string, AgentDefinitionDraft, bool) (AgentDef, error),
	writeFile func(*fileutil.Directory, string, string, []byte, bool) error,
) (result AgentDef, err error) {
	if len(additions) == 0 {
		return writeDefinition(path, draft, overwrite)
	}
	if _, _, err := RenderAgentDefinition(draft); err != nil {
		return AgentDef{}, err
	}
	directory, name, err := fileutil.OpenOrCreateParentDirectory(path, privateDirMode)
	if err != nil {
		return AgentDef{}, err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	previous, existed, err := readOptionalRegularFileFromDirectory(directory, name, path, "agent definition")
	if err != nil {
		return AgentDef{}, err
	}
	sidecarPath := filepath.Join(filepath.Dir(path), MCPJSONName)
	previousSidecar, sidecarExisted, err := readOptionalRegularFileFromDirectory(
		directory,
		MCPJSONName,
		sidecarPath,
		"agent MCP sidecar",
	)
	if err != nil {
		return AgentDef{}, err
	}
	rendered, servers, err := prepareAgentMCPSidecar(sidecarPath, previousSidecar, additions)
	if err != nil {
		return AgentDef{}, err
	}
	restore := func(file, fullPath string, old []byte, exists bool) error {
		if exists {
			return writeFile(directory, file, fullPath, old, true)
		}
		if err := directory.RemoveRegularFile(file); !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	agent, err := writeDefinition(path, draft, overwrite)
	if err != nil {
		if !errors.Is(err, fileutil.ErrPublishedNotDurable) {
			return AgentDef{}, err
		}
		restoreErr := restore(name, path, previous, existed)
		if restoreErr != nil {
			return AgentDef{}, errors.Join(err, restoreErr)
		}
		return AgentDef{}, err
	}
	if err := writeFile(directory, MCPJSONName, sidecarPath, rendered, true); err != nil {
		restoreErr := errors.Join(
			restore(name, path, previous, existed),
			restore(MCPJSONName, sidecarPath, previousSidecar, sidecarExisted),
		)
		if restoreErr != nil {
			agent.MCPServers = OverrideMCPServers(agent.MCPServers, servers)
			return agent, errors.Join(err, restoreErr)
		}
		return AgentDef{}, err
	}
	agent.MCPServers = OverrideMCPServers(agent.MCPServers, servers)
	return agent, nil
}

// rejectConfiguredMCPReferences keeps name indirection confined to agent definitions.
func rejectConfiguredMCPReferences(servers []MCPServer) error {
	for _, server := range servers {
		if server.Reference {
			return fmt.Errorf("MCP references are only supported in agent sidecars")
		}
	}
	return nil
}

// Agent-local connections may reuse explicitly shared credentials. Use a name
// reference to attach a configured server together with its owned credentials.
func validateAgentMCPSecretRefs(server MCPServer) error {
	refs := []string{server.Auth.ClientSecretRef}
	for _, ref := range server.SecretHeaders {
		refs = append(refs, ref)
	}
	for _, ref := range server.SecretEnv {
		refs = append(refs, ref)
	}
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		if err := vault.ValidateSecretRefNamespace(ref, "mcp"); err != nil {
			return err
		}
		if !strings.HasPrefix(vault.NormalizeRef(ref), vault.MCPSharedRefPrefix) {
			return fmt.Errorf("agent-local MCP credentials require explicitly shared Vault references")
		}
	}
	return nil
}
