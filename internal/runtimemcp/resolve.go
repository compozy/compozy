package runtimemcp

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/config"
)

type ImportError struct{ Code, Message string }

func (e *ImportError) Error() string { return e.Message }

// ResolvedServer keeps private values outside inventory snapshots and serialized responses.
type ResolvedServer struct{ value entry }

func (s ResolvedServer) Metadata() Server { return s.value.server }

func (Reader) Resolve(ctx context.Context, req Request, name string, previous *Server) (ResolvedServer, error) {
	if err := ctx.Err(); err != nil {
		return ResolvedServer{}, err
	}
	label := req.Runtime
	for _, descriptor := range Descriptors() {
		if descriptor.Runtime != req.Runtime {
			continue
		}
		label = descriptor.DisplayName
		snapshot, _, entries := readDefinitions(ResolveContext(req.Context), descriptor)
		if value, ok := entries[name]; ok && snapshot.ReadState == "ok" {
			if previous != nil &&
				(previous.DefinitionDigest != value.server.DefinitionDigest || previous.SourcePath != value.server.SourcePath) {
				break
			}
			return ResolvedServer{value: value}, nil
		}
		break
	}
	message := fmt.Sprintf("No MCP server named %s in %s", name, label)
	if previous != nil {
		message = fmt.Sprintf("The server %s is no longer in %s", name, previous.SourceLabel)
	}
	return ResolvedServer{}, &ImportError{Code: "mcp_server_not_found", Message: message}
}

func (s ResolvedServer) CopyDefinition() (config.MCPServer, error) {
	metadata, d := s.value.server, s.value.definition
	if reason := copyBlocked(metadata, d); reason != "" {
		code := "mcp_import_unsupported_definition"
		switch {
		case metadata.Scope == "plugin" || strings.HasPrefix(metadata.Name, "plugin:"):
			code = "mcp_import_unsupported_source"
			if metadata.Runtime == "claude" {
				reason = metadata.Name + " is provided by a Claude Code plugin and can't be copied"
			}
		case strings.Contains(reason, "runtime-owned source"):
			code = "mcp_import_unsupported_source"
		case metadata.Transport != "stdio" && metadata.Transport != "http":
			code = "mcp_import_unsupported_transport"
		}
		return config.MCPServer{}, &ImportError{Code: code, Message: reason}
	}
	server := config.MCPServer{Name: metadata.Name, Transport: config.MCPServerTransport(metadata.Transport),
		Command: str(d["command"]), Args: names(d["args"]), URL: cmp.Or(str(d["url"]), str(d["httpUrl"]))}
	if command := names(d["command"]); len(command) > 0 {
		server.Command = command[0]
		server.Args = append(command[1:], server.Args...)
	}
	server.Env = literalMap(d, "env", "environment")
	server.Headers = literalMap(d, "headers", "http_headers", "header")
	if has(d, "oauth") || str(d["auth"]) == "oauth" {
		server.Auth.Registration = config.MCPAuthRegistrationAuto
	}
	return server, nil
}

func literalMap(d object, keys ...string) map[string]string {
	for _, key := range keys {
		if value := obj(d[key]); value != nil {
			result := make(map[string]string, len(value))
			for name, literal := range value {
				result[name] = str(literal)
			}
			return result
		}
	}
	return nil
}
