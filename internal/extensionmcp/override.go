package extensionmcp

import (
	"errors"
	"maps"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/mcppolicy"
)

// Validate rejects secret-bearing or malformed override values before mutation.
func (o Override) Validate() error {
	for name, value := range o.Env {
		if err := compozyconfig.ValidateMCPStdioEnvName("override.env", name, false); err != nil {
			return err
		}
		if len(value) > 8192 || strings.ContainsRune(value, '\x00') {
			return errors.New("extension: invalid MCP override env value")
		}
	}
	if err := mcppolicy.ValidateHeaders(o.Headers, nil, mcppolicy.SourcePackageFixed, false); err != nil {
		return err
	}
	if o.URL != "" {
		return mcppolicy.ValidateRemoteURL(o.URL)
	}
	return nil
}

// Apply validates the actual transport and auth policy after layering over the declaration.
func (o Override) Apply(server compozyconfig.MCPServer) (compozyconfig.MCPServer, error) {
	if err := o.Validate(); err != nil {
		return compozyconfig.MCPServer{}, err
	}
	server.Env, server.Headers = maps.Clone(server.Env), maps.Clone(server.Headers)
	if len(o.Env) > 0 {
		if server.Env == nil {
			server.Env = make(map[string]string)
		}
		maps.Copy(server.Env, o.Env)
	}
	if len(o.Headers) > 0 {
		if server.Headers == nil {
			server.Headers = make(map[string]string)
		}
		maps.Copy(server.Headers, o.Headers)
	}
	if o.URL != "" {
		server.URL = o.URL
	}
	if err := server.Validate("extension.mcp_server"); err != nil {
		return compozyconfig.MCPServer{}, err
	}
	return server, nil
}
