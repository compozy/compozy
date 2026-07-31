package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/marketplace"
)

const (
	mcpCatalogLaunchNPM    = "npm"
	mcpCatalogLaunchUVX    = "uvx"
	mcpCatalogLaunchDocker = "docker"
	mcpCatalogLaunchRemote = "remote"
	mcpCatalogInputSecret  = "secret"
	mcpCatalogBindingEnv   = "env"
	mcpCatalogBindingQuery = "url_query"
	mcpCatalogCommandNPX   = "npx"
)

func mcpServerFromCatalogLaunch(
	name string,
	entry *marketplace.Entry,
	detail *marketplace.MCPEntryDetails,
) (compozyconfig.MCPServer, error) {
	if entry == nil || detail == nil {
		return compozyconfig.MCPServer{}, errors.New("settings: MCP catalog launch is required")
	}
	server := compozyconfig.MCPServer{
		Name:           strings.TrimSpace(name),
		CatalogEntry:   strings.TrimSpace(entry.EntryID),
		CatalogVersion: strings.TrimSpace(entry.Version),
	}
	launch := detail.Launch
	switch launch.Type {
	case mcpCatalogLaunchNPM:
		server.Command = mcpCatalogCommandNPX
		server.Args = append([]string{"-y", launch.Package + "@" + launch.Version}, launch.Args...)
	case mcpCatalogLaunchUVX:
		server.Command = mcpCatalogLaunchUVX
		server.Args = append(
			[]string{"--from", launch.Package + "==" + strings.TrimPrefix(launch.Version, "v"), launch.Package},
			launch.Args...)
	case mcpCatalogLaunchDocker:
		server.Command = mcpCatalogLaunchDocker
	case mcpCatalogLaunchRemote:
		server.Transport = compozyconfig.MCPServerTransportHTTP
		server.URL = strings.TrimSpace(launch.URL)
	default:
		return compozyconfig.MCPServer{}, validationError(
			fmt.Errorf("settings: catalog entry %q has unsupported launch type %q", entry.EntryID, launch.Type),
		)
	}
	if detail.Auth != nil {
		server.Auth = compozyconfig.MCPAuthConfig{
			Registration: compozyconfig.MCPAuthRegistrationAuto,
			Scopes:       append([]string(nil), detail.Auth.Scopes...),
		}
	}
	return server, nil
}

func finalizeMCPCatalogLaunch(server *compozyconfig.MCPServer, detail *marketplace.MCPEntryDetails) error {
	if server == nil || detail == nil {
		return errors.New("settings: MCP catalog launch target is required")
	}
	if detail.Launch.Type != mcpCatalogLaunchDocker {
		return nil
	}
	names := make([]string, 0, len(server.Env)+len(server.SecretEnv))
	for name := range server.Env {
		names = append(names, name)
	}
	for name := range server.SecretEnv {
		names = append(names, name)
	}
	sort.Strings(names)
	args := []string{"run", "-i", "--rm"}
	for _, name := range names {
		args = append(args, "--env", name)
	}
	image := strings.TrimSpace(detail.Launch.Image) + "@" + strings.TrimSpace(detail.Launch.Digest)
	args = append(args, image)
	args = append(args, detail.Launch.Args...)
	server.Args = args
	return nil
}

func (s *service) applyCatalogInputs(
	ctx context.Context,
	server *compozyconfig.MCPServer,
	detail *marketplace.MCPEntryDetails,
	req MCPCatalogInstallRequest,
	values map[string]MCPSecretInput,
) (MCPSecretValues, error) {
	if server == nil || detail == nil {
		return MCPSecretValues{}, errors.New("settings: MCP catalog input target is required")
	}
	normalized, err := normalizeMCPCatalogInputs(values)
	if err != nil {
		return MCPSecretValues{}, err
	}
	declared := make(map[string]struct{}, len(detail.Inputs))
	secrets := MCPSecretValues{}
	for _, input := range detail.Inputs {
		id := strings.TrimSpace(input.ID)
		declared[id] = struct{}{}
		value, supplied := normalized[id]
		if input.Type == mcpCatalogInputSecret {
			if err := s.applyCatalogSecretInput(ctx, server, &secrets, req, input, value, supplied); err != nil {
				return MCPSecretValues{}, err
			}
			continue
		}
		resolved, present, err := resolveCatalogPlainInput(input, value, supplied)
		if err != nil {
			return MCPSecretValues{}, err
		}
		if err := bindCatalogPlainInput(server, input, resolved, present); err != nil {
			return MCPSecretValues{}, err
		}
	}
	for id := range normalized {
		if _, exists := declared[id]; !exists {
			return MCPSecretValues{}, validationError(
				fmt.Errorf("settings: values.inputs.%s is not declared by the catalog entry", id),
			)
		}
	}
	return secrets, nil
}

func normalizeMCPCatalogInputs(values map[string]MCPSecretInput) (map[string]MCPSecretInput, error) {
	normalized := make(map[string]MCPSecretInput, len(values))
	for rawID, input := range values {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, validationError(errors.New("settings: values.inputs key is required"))
		}
		if _, exists := normalized[id]; exists {
			return nil, validationError(fmt.Errorf("settings: values.inputs key %q is duplicated", id))
		}
		normalized[id] = input
	}
	return normalized, nil
}

func (s *service) applyCatalogSecretInput(
	ctx context.Context,
	server *compozyconfig.MCPServer,
	secrets *MCPSecretValues,
	req MCPCatalogInstallRequest,
	input marketplace.MCPInputDetails,
	value MCPSecretInput,
	supplied bool,
) error {
	id := strings.TrimSpace(input.ID)
	bindingType := strings.TrimSpace(input.Binding.Type)
	if bindingType != mcpCatalogBindingEnv {
		return validationError(fmt.Errorf(
			"settings: unsupported catalog secret input binding %q",
			bindingType,
		))
	}
	bindingName := strings.TrimSpace(input.Binding.Name)
	if !supplied {
		if input.Required {
			return validationError(fmt.Errorf("settings: values.inputs.%s is required by the catalog entry", id))
		}
		return nil
	}
	mode, err := validateMCPSecretInput("values.inputs."+id, value)
	if err != nil {
		return err
	}
	if mode == mcpSecretInputVaultRef {
		ref, err := s.validateCatalogInputVaultRef(
			ctx,
			"values.inputs."+id,
			req.Scope,
			req.WorkspaceID,
			server.Name,
			value.VaultRef,
		)
		if err != nil {
			return err
		}
		setMCPSecretEnvRef(server, bindingName, ref)
		return nil
	}
	if secrets.SecretEnv == nil {
		secrets.SecretEnv = make(map[string]string)
	}
	secrets.SecretEnv[bindingName] = value.Value
	return nil
}

func resolveCatalogPlainInput(
	input marketplace.MCPInputDetails,
	provided MCPSecretInput,
	supplied bool,
) (string, bool, error) {
	id := strings.TrimSpace(input.ID)
	if supplied {
		mode, err := validateMCPSecretInput("values.inputs."+id, provided)
		if err != nil {
			return "", false, err
		}
		if mode != mcpSecretInputValue {
			return "", false, validationError(
				fmt.Errorf("settings: values.inputs.%s must use value for a non-secret input", id),
			)
		}
		value, err := marketplace.NormalizeMCPInputValue(input.Type, provided.Value)
		if err != nil {
			return "", false, validationError(fmt.Errorf("settings: values.inputs.%s: %w", id, err))
		}
		return value, true, nil
	}
	if len(input.Default) == 0 {
		if input.Required {
			return "", false, validationError(
				fmt.Errorf("settings: values.inputs.%s is required by the catalog entry", id),
			)
		}
		return "", false, nil
	}
	value, err := catalogDefaultInputValue(input.Type, input.Default)
	if err != nil {
		return "", false, validationError(fmt.Errorf("settings: catalog default for input %q: %w", id, err))
	}
	return value, true, nil
}

func catalogDefaultInputValue(inputType string, raw json.RawMessage) (string, error) {
	switch inputType {
	case "string", "identifier":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("decode string: %w", err)
		}
		return marketplace.NormalizeMCPInputValue(inputType, value)
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("decode boolean: %w", err)
		}
		return strconv.FormatBool(value), nil
	default:
		return "", fmt.Errorf("unsupported input type %q", inputType)
	}
}

func bindCatalogPlainInput(
	server *compozyconfig.MCPServer,
	input marketplace.MCPInputDetails,
	value string,
	present bool,
) error {
	if !present {
		return nil
	}
	switch input.Binding.Type {
	case mcpCatalogBindingEnv:
		if server.Env == nil {
			server.Env = make(map[string]string)
		}
		server.Env[strings.TrimSpace(input.Binding.Name)] = value
		return nil
	case mcpCatalogBindingQuery:
		if strings.TrimSpace(server.URL) == "" {
			return validationError(fmt.Errorf(
				"settings: catalog input %q uses a url_query binding on a non-remote launch",
				strings.TrimSpace(input.ID),
			))
		}
		parsed, err := url.Parse(server.URL)
		if err != nil {
			return validationError(fmt.Errorf("settings: parse catalog remote URL: %w", err))
		}
		query := parsed.Query()
		query.Set(strings.TrimSpace(input.Binding.Name), value)
		parsed.RawQuery = query.Encode()
		server.URL = parsed.String()
		return nil
	default:
		return validationError(fmt.Errorf("settings: unsupported catalog input binding %q", input.Binding.Type))
	}
}
