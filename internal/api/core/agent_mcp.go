package core

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/gin-gonic/gin"
)

func agentMCPServersFromPayload(payload []contract.AgentMCPServerInput) []compozyconfig.MCPServer {
	servers := make([]compozyconfig.MCPServer, 0, len(payload))
	for _, value := range payload {
		server := compozyconfig.MCPServer{
			Name:          value.Name,
			Reference:     value.Reference,
			Transport:     compozyconfig.MCPServerTransport(value.Transport),
			Command:       value.Command,
			Args:          slices.Clone(value.Args),
			Env:           maps.Clone(value.Env),
			SecretEnv:     maps.Clone(value.SecretEnv),
			URL:           value.URL,
			SecretHeaders: maps.Clone(value.SecretHeaders),
		}
		if value.Auth != nil {
			server.Auth = compozyconfig.MCPAuthConfig{
				Registration: compozyconfig.MCPAuthRegistration(
					value.Auth.Registration,
				),
				IssuerURL:       value.Auth.IssuerURL,
				ClientID:        value.Auth.ClientID,
				ClientSecretRef: value.Auth.ClientSecretRef,
				Scopes:          slices.Clone(value.Auth.Scopes),
			}
		}
		servers = append(servers, server)
	}
	return servers
}

func writeAgentDefinitionWithMCP(
	ctx context.Context,
	secrets VaultService,
	path string,
	draft compozyconfig.AgentDefinitionDraft,
	overwrite bool,
	inputs []contract.AgentMCPServerInput,
) (compozyconfig.AgentDef, error) {
	servers := agentMCPServersFromPayload(inputs)
	written := []string{}
	rollback := func() error {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		var err error
		for _, ref := range written {
			err = errors.Join(err, secrets.DeleteSecret(cleanupCtx, ref))
		}
		return err
	}
	for i, input := range inputs {
		server := &servers[i]
		if server.Reference && len(input.SecretHeaderValues)+len(input.SecretEnvValues) > 0 {
			return compozyconfig.AgentDef{}, errors.Join(
				compozyconfig.ErrInvalidAgentDefinition,
				fmt.Errorf("a reference cannot contain credentials"),
				rollback(),
			)
		}
		if server.SecretHeaders == nil {
			server.SecretHeaders = map[string]string{}
		}
		if server.SecretEnv == nil {
			server.SecretEnv = map[string]string{}
		}
		for _, binding := range []struct{ values, refs map[string]string }{
			{input.SecretHeaderValues, server.SecretHeaders}, {input.SecretEnvValues, server.SecretEnv},
		} {
			for key, value := range binding.values {
				if secrets == nil {
					return compozyconfig.AgentDef{}, errors.Join(
						fmt.Errorf("agent MCP credentials require Vault"),
						rollback(),
					)
				}
				if strings.TrimSpace(value) == "" {
					return compozyconfig.AgentDef{}, errors.Join(
						compozyconfig.ErrInvalidAgentDefinition,
						fmt.Errorf("MCP credential must not be empty"),
						rollback(),
					)
				}
				ref := "vault:mcp/shared/agent-attachments/" + strings.ToLower(rand.Text())
				if _, err := secrets.PutSecret(ctx, ref, "mcp", value); err != nil {
					return compozyconfig.AgentDef{}, errors.Join(err, rollback())
				}
				written = append(written, ref)
				binding.refs[key] = ref
			}
		}
	}
	agent, err := compozyconfig.CreateAgentDefWithMCPServers(path, draft, overwrite, servers)
	if err != nil {
		// A close failure can follow committed writes; their credentials must remain usable.
		if agent.Name != "" {
			return agent, err
		}
		return compozyconfig.AgentDef{}, errors.Join(err, rollback())
	}
	persisted, err := compozyconfig.LoadAgentDefFile(path)
	if err != nil {
		return agent, err
	}
	return persisted, nil
}

func (h *BaseHandlers) writeUpdatedAgentDefinition(
	c *gin.Context,
	path string,
	draft compozyconfig.AgentDefinitionDraft,
	servers []contract.AgentMCPServerInput,
) (compozyconfig.AgentDef, error) {
	return writeAgentDefinitionWithMCP(c.Request.Context(), h.Vault, path, draft, true, servers)
}

// RollbackAgentMCPSecretValues removes only credentials minted by a failed create request.
func RollbackAgentMCPSecretValues(
	ctx context.Context,
	secrets VaultService,
	agent compozyconfig.AgentDef,
	inputs []contract.AgentMCPServerInput,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var result error
	for _, input := range inputs {
		for _, server := range agent.MCPServers {
			if server.Name != strings.TrimSpace(input.Name) {
				continue
			}
			for key := range input.SecretHeaderValues {
				result = errors.Join(result, secrets.DeleteSecret(cleanupCtx, server.SecretHeaders[key]))
			}
			for key := range input.SecretEnvValues {
				result = errors.Join(result, secrets.DeleteSecret(cleanupCtx, server.SecretEnv[key]))
			}
		}
	}
	return result
}

func (h *BaseHandlers) rollbackCreatedAgentWithMCP(
	ctx context.Context,
	agent compozyconfig.AgentDef,
	inputs []contract.AgentMCPServerInput,
) error {
	if err := h.rollbackCreatedAgentDefinition(ctx, agent.SourcePath); err != nil {
		return err
	}
	return RollbackAgentMCPSecretValues(ctx, h.Vault, agent, inputs)
}
