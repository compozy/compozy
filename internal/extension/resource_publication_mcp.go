package extensionpkg

import (
	"fmt"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"slices"
	"strings"
)

// ResolvedManifestMCPServer keeps the install default separate from runtime transport configuration.
type ResolvedManifestMCPServer struct {
	compozyconfig.MCPServer
	DefaultScope string
}

// ResolveManifestMCPServerResources converts manifest MCP declarations into MCP server specs.
func ResolveManifestMCPServerResources(
	rootDir string,
	manifest *Manifest,
	state InputState,
	getenv func(string) string,
) ([]ResolvedManifestMCPServer, error) {
	if manifest == nil || len(manifest.Resources.MCPServers) == 0 {
		return nil, nil
	}
	if err := ValidateManifestInputs(manifest); err != nil {
		return nil, err
	}
	if len(manifest.Inputs) > 0 {
		ready := InputReadiness(manifest, state, getenv)
		if err := ready.RequiredError(manifest); err != nil {
			return nil, err
		}
	}

	names := make([]string, 0, len(manifest.Resources.MCPServers))
	for name := range manifest.Resources.MCPServers {
		names = append(names, name)
	}
	slices.Sort(names)

	servers := make([]ResolvedManifestMCPServer, 0, len(names))
	for _, name := range names {
		decl := manifest.Resources.MCPServers[name]
		if err := validateManifestMCPAuth(decl, "resources.mcp_servers."+name+".auth"); err != nil {
			return nil, err
		}
		var err error
		server := manifestMCPServerSpec(manifest, name, decl)
		switch strings.TrimSpace(decl.Transport) {
		case "", string(compozyconfig.MCPServerTransportStdio):
			server.Transport = compozyconfig.MCPServerTransportStdio
			server.Command, err = resolveManifestCommand(rootDir, decl.Command, getenv, nil)
			if err != nil {
				return nil, err
			}
			server.Args, err = resolveManifestStringSlice(rootDir, decl.Args, getenv, nil)
			if err != nil {
				return nil, err
			}
			server.Env, err = resolveManifestStringMap(rootDir, decl.Env, getenv, nil)
			if err != nil {
				return nil, err
			}
		case string(compozyconfig.MCPServerTransportHTTP):
			server.Transport = compozyconfig.MCPServerTransportHTTP
			server.URL, err = resolveManifestString(rootDir, decl.URL, getenv, nil)
			if err != nil {
				return nil, fmt.Errorf("extension: resolve mcp server %q URL: %w", name, err)
			}
			server.Headers, err = resolveManifestStringMap(rootDir, decl.Headers, getenv, nil)
			if err != nil {
				return nil, fmt.Errorf("extension: resolve mcp server %q headers: %w", name, err)
			}
		default:
			return nil, fmt.Errorf(
				"extension: mcp server %q has unsupported transport %q",
				name,
				decl.Transport,
			)
		}
		if err := applyManifestMCPInputs(manifest.Inputs, decl, &server, state, getenv); err != nil {
			return nil, err
		}
		if err := server.Validate("extension.resources.mcp_servers[" + name + "]"); err != nil {
			return nil, err
		}
		servers = append(servers, ResolvedManifestMCPServer{MCPServer: server, DefaultScope: decl.DefaultScope})
	}
	return servers, nil
}

func manifestMCPServerSpec(manifest *Manifest, name string, decl MCPServerConfig) compozyconfig.MCPServer {
	server := compozyconfig.MCPServer{
		Name: strings.TrimSpace(name), Auth: manifestMCPAuthConfig(decl.Auth),
		CWD: strings.TrimSpace(decl.CWD), SecretEnv: normalizeStringMap(decl.SecretEnv),
		Headers: normalizeStringMap(decl.Headers),
	}
	if extensionName := strings.TrimSpace(manifest.Name); extensionName != "" {
		server.Owner = "extension:" + extensionName
	}
	return server
}
