package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

const (
	agentFallbackRouteFlag      = "fallback-route"
	agentClearFallbackChainFlag = "clear-fallback-chain"
	fallbackRouteCommandKey     = "command"
	fallbackRouteCommandPrefix  = fallbackRouteCommandKey + "="
	// fallbackRouteReasoningEffortKey matches the route's config/wire field name.
	fallbackRouteReasoningEffortKey = "reasoning_effort"
)

func addAgentFallbackChainFlags(cmd *cobra.Command, flags *agentDefinitionFlags) {
	cmd.Flags().StringArrayVar(
		&flags.fallbackRoutes,
		agentFallbackRouteFlag,
		nil,
		"Fallback route provider=<p>,model=<m>[,reasoning_effort=<r>][,speed=<s>][,command=<c>] "+
			"(repeatable, ordered; command must be last and takes the rest verbatim)",
	)
	cmd.Flags().BoolVar(
		&flags.clearFallbackChain,
		agentClearFallbackChainFlag,
		false,
		"Remove every fallback route from the agent definition",
	)
}

// applyAgentFallbackChainFlags replaces payload.FallbackChain only when a chain flag changed.
func applyAgentFallbackChainFlags(
	cmd *cobra.Command,
	payload *contract.CreateAgentPayload,
	flags agentDefinitionFlags,
) error {
	routesChanged := cmd.Flags().Changed(agentFallbackRouteFlag)
	clearChanged := cmd.Flags().Changed(agentClearFallbackChainFlag) && flags.clearFallbackChain
	switch {
	case routesChanged && clearChanged:
		return fmt.Errorf(
			"cli: use either --%s or --%s, not both",
			agentFallbackRouteFlag,
			agentClearFallbackChainFlag,
		)
	case clearChanged:
		payload.FallbackChain = nil
		return nil
	case !routesChanged:
		return nil
	}
	chain, err := parseAgentFallbackRoutes(payload.Name, flags.fallbackRoutes)
	if err != nil {
		return err
	}
	payload.FallbackChain = chain
	return nil
}

func parseAgentFallbackRoutes(agentName string, values []string) ([]contract.AgentFallbackRoutePayload, error) {
	chain := make([]contract.AgentFallbackRoutePayload, 0, len(values))
	chainPath := compozyconfig.AgentFallbackChainPath(agentName)
	for index, value := range values {
		routePath := fmt.Sprintf("%s[%d]", chainPath, index)
		route, err := parseAgentFallbackRoute(routePath, value)
		if err != nil {
			return nil, err
		}
		if err := compozyconfig.ValidateFallbackRouteCommand(routePath, route.Command); err != nil {
			return nil, err
		}
		chain = append(chain, route)
	}
	return chain, nil
}

// parseAgentFallbackRoute parses key=value pairs; `command` must be the last key and
// takes the remainder of the value verbatim (commas and quotes included).
func parseAgentFallbackRoute(routePath, value string) (contract.AgentFallbackRoutePayload, error) {
	var route contract.AgentFallbackRoutePayload
	rest := strings.TrimSpace(value)
	if rest == "" {
		return route, fmt.Errorf("%s: --%s value is required", routePath, agentFallbackRouteFlag)
	}
	seen := make(map[string]bool, 5)
	for rest != "" {
		if strings.HasPrefix(rest, fallbackRouteCommandPrefix) {
			if seen[fallbackRouteCommandKey] {
				return route, fmt.Errorf("%s: duplicate key %q", routePath, fallbackRouteCommandKey)
			}
			route.Command = strings.TrimSpace(strings.TrimPrefix(rest, fallbackRouteCommandPrefix))
			seen[fallbackRouteCommandKey] = true
			break
		}
		pair, remainder, _ := strings.Cut(rest, ",")
		rest = strings.TrimSpace(remainder)
		key, fieldValue, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		fieldValue = strings.TrimSpace(fieldValue)
		if !ok || key == "" {
			return route, fmt.Errorf("%s: expected key=value, got %q", routePath, pair)
		}
		if seen[key] {
			return route, fmt.Errorf("%s: duplicate key %q", routePath, key)
		}
		seen[key] = true
		if err := setAgentFallbackRouteField(&route, routePath, key, fieldValue); err != nil {
			return route, err
		}
	}
	if route.Provider == "" {
		return route, fmt.Errorf("%s.provider is required", routePath)
	}
	if route.Model == "" {
		return route, fmt.Errorf("%s.model is required", routePath)
	}
	return route, nil
}

func setAgentFallbackRouteField(
	route *contract.AgentFallbackRoutePayload,
	routePath string,
	key string,
	value string,
) error {
	switch key {
	case cliProviderKey:
		route.Provider = value
	case agentModelKey:
		route.Model = value
	case fallbackRouteReasoningEffortKey:
		if err := validateAgentReasoningEffort(value); err != nil {
			return fmt.Errorf("%s.reasoning_effort: %w", routePath, err)
		}
		route.ReasoningEffort = contract.ReasoningEffort(value)
	case "speed":
		speed, err := parseAgentSpeedFlag(value)
		if err != nil {
			return fmt.Errorf("%s.speed: %w", routePath, err)
		}
		route.Speed = speed
	default:
		return fmt.Errorf(
			"%s: unknown key %q (want provider, model, reasoning_effort, speed, command)",
			routePath,
			key,
		)
	}
	return nil
}

// agentFallbackChainFromRecord keeps the authored chain on update; fingerprints are
// read-only projections and never round-trip.
func agentFallbackChainFromRecord(chain []contract.RoleFallbackStatus) []contract.AgentFallbackRoutePayload {
	if len(chain) == 0 {
		return nil
	}
	converted := make([]contract.AgentFallbackRoutePayload, 0, len(chain))
	for _, route := range chain {
		converted = append(converted, contract.AgentFallbackRoutePayload{
			Provider:        route.Provider,
			Model:           route.Model,
			ReasoningEffort: contract.ReasoningEffort(route.ReasoningEffort),
			Speed:           route.Speed,
			ACPOptions:      cloneAgentACPOptions(route.ACPOptions),
			Command:         route.Command,
		})
	}
	return converted
}

func agentFallbackChainRows(chain []contract.RoleFallbackStatus) [][]string {
	rows := make([][]string, 0, len(chain))
	for _, route := range chain {
		rows = append(rows, []string{route.Provider, route.Model, route.ReasoningEffort, route.Command})
	}
	return rows
}
