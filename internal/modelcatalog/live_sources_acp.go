package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerexec"
)

const acpDiscoveryAgentName = "compozy-model-catalog"

// ACPModelProbe reads model options advertised by a short-lived ACP session.
type ACPModelProbe interface {
	InspectModels(context.Context, ACPModelProbeRequest) ([]acp.SessionConfigOption, error)
}

// ACPModelProbeRequest captures one provider ACP discovery invocation.
type ACPModelProbeRequest struct {
	ProviderID string
	Command    string
	Cwd        string
	Env        []string
	Timeout    time.Duration
}

// SessionACPModelProbe reads advertised model options through ACP.
type SessionACPModelProbe struct{}

var _ ACPModelProbe = SessionACPModelProbe{}

// InspectModels creates a short-lived ACP session without changing its configuration.
func (SessionACPModelProbe) InspectModels(
	ctx context.Context,
	req ACPModelProbeRequest,
) ([]acp.SessionConfigOption, error) {
	options, err := acp.InspectSessionConfigOptions(ctx, acp.SessionInspectionRequest{
		AgentName: acpDiscoveryAgentName,
		Command:   strings.TrimSpace(req.Command),
		Cwd:       strings.TrimSpace(req.Cwd),
		Env:       append([]string(nil), req.Env...),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"model catalog: inspect %s ACP model options: %w",
			strings.TrimSpace(req.ProviderID),
			err,
		)
	}
	return options, nil
}

func (s *LiveProviderSource) listACP(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	env []string,
	timeout time.Duration,
	now time.Time,
) ([]ModelRow, error) {
	if s.acpProbe == nil {
		return nil, errors.New("model catalog: ACP model probe is required")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("model catalog: resolve ACP discovery working directory: %w", err)
	}
	env, err = nativeCLIACPEnv(provider, env, cwd)
	if err != nil {
		return nil, err
	}
	options, err := s.acpProbe.InspectModels(ctx, ACPModelProbeRequest{
		ProviderID: s.providerID,
		Command:    strings.TrimSpace(provider.Command),
		Cwd:        cwd,
		Env:        append([]string(nil), env...),
		Timeout:    timeout,
	})
	if err != nil {
		return nil, err
	}
	modelOption, ok := acp.ModelConfigOption(options)
	if !ok {
		return nil, fmt.Errorf(
			"model catalog: %s ACP session did not advertise a model select option",
			s.providerID,
		)
	}
	rows := acpModelRows(s.providerID, modelOption, now)
	if len(rows) == 0 {
		return nil, fmt.Errorf(
			"model catalog: %s ACP model option did not advertise model values",
			s.providerID,
		)
	}
	return rows, nil
}

func nativeCLIACPEnv(
	provider compozyconfig.ProviderConfig,
	env []string,
	cwd string,
) ([]string, error) {
	strategy := providerexec.StrategyFor(provider)
	if strategy.Kind != providerexec.StrategyNativeCLIBridge {
		return append([]string(nil), env...), nil
	}
	executable, err := providerexec.ResolveNativeCLI(strategy, env, cwd)
	if err != nil {
		return nil, fmt.Errorf("model catalog: %w", err)
	}
	if strategy.NativeCLI.BridgeEnvKey == "" {
		return append([]string(nil), env...), nil
	}
	return setDiscoveryEnvValue(env, strategy.NativeCLI.BridgeEnvKey, executable), nil
}

func setDiscoveryEnvValue(env []string, key string, value string) []string {
	prefix := strings.TrimSpace(key) + "="
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+strings.TrimSpace(value))
}

func acpModelRows(providerID string, modelOption acp.SessionConfigOption, now time.Time) []ModelRow {
	rows := make([]ModelRow, 0, len(modelOption.Values))
	seen := make(map[string]struct{}, len(modelOption.Values))
	for _, value := range modelOption.Values {
		modelID := strings.TrimSpace(value.Value)
		if modelID == "" {
			continue
		}
		if _, exists := seen[modelID]; exists {
			continue
		}
		seen[modelID] = struct{}{}
		displayName := strings.TrimSpace(value.Label)
		if displayName == "" {
			displayName = modelID
		}
		available := true
		rows = append(rows, ModelRow{
			ProviderID: providerID, ModelID: modelID, DisplayName: displayName,
			SourceID: SourceKindProviderLiveID(providerID), SourceKind: SourceKindProviderLive,
			Priority: PriorityProviderLive, Available: &available, RefreshedAt: now,
		})
	}
	sortModelRowsByID(rows)
	return rows
}
