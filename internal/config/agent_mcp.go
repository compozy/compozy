package config

import (
	"errors"
	"fmt"
	"path/filepath"

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
		for rangeKey, ref := range server.SecretHeaders {
			if err := vault.ValidateSecretRefNamespace(ref, "mcp"); err != nil {
				return nil, nil, fmt.Errorf(
					"%w: secret_headers.%s requires an MCP Vault reference",
					ErrInvalidAgentDefinition,
					rangeKey,
				)
			}
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
	servers, err := ParseMCPServersJSON(rendered, path)
	return rendered, servers, err
}

// CreateAgentDefWithMCPServers co-ships an authored definition and additive sidecar entries.
func CreateAgentDefWithMCPServers(
	path string,
	draft AgentDefinitionDraft,
	overwrite bool,
	additions []MCPServer,
) (result AgentDef, err error) {
	if len(additions) == 0 {
		return CreateAgentDefFile(path, draft, overwrite)
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
	agent, err := CreateAgentDefFile(path, draft, overwrite)
	if err != nil {
		return AgentDef{}, err
	}
	if err := writePersistedFileInDirectory(directory, MCPJSONName, sidecarPath, rendered, true); err != nil {
		restore := func(file, fullPath string, old []byte, exists bool) error {
			if exists {
				return writePersistedFileInDirectory(directory, file, fullPath, old, true)
			}
			return directory.RemoveRegularFile(file)
		}
		return AgentDef{}, errors.Join(
			err,
			restore(name, path, previous, existed),
			restore(MCPJSONName, sidecarPath, previousSidecar, sidecarExisted),
		)
	}
	agent.MCPServers = OverrideMCPServers(agent.MCPServers, servers)
	return agent, nil
}

func rejectConfiguredMCPReferences(servers []MCPServer) error {
	for _, server := range servers {
		if server.Reference {
			return fmt.Errorf("MCP references are only supported in agent sidecars")
		}
	}
	return nil
}
