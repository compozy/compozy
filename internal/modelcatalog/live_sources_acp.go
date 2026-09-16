package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerexec"
)

const acpDiscoveryAgentName = "compozy-model-catalog"

// ACPModelProbe reads model options advertised by a short-lived ACP session.
type ACPModelProbe interface {
	InspectModels(context.Context, ACPModelProbeRequest) (acp.SessionModelInspection, error)
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

// InspectModels selects each model in a disposable ACP session to read its options.
func (SessionACPModelProbe) InspectModels(
	ctx context.Context,
	req ACPModelProbeRequest,
) (acp.SessionModelInspection, error) {
	inspection, err := acp.InspectSessionModels(ctx, acp.SessionInspectionRequest{
		AgentName: acpDiscoveryAgentName,
		Command:   strings.TrimSpace(req.Command),
		Cwd:       strings.TrimSpace(req.Cwd),
		Env:       append([]string(nil), req.Env...),
	})
	if err != nil {
		return acp.SessionModelInspection{}, fmt.Errorf(
			"model catalog: inspect %s ACP model options: %w",
			strings.TrimSpace(req.ProviderID),
			err,
		)
	}
	return inspection, nil
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
	cwd := strings.TrimSpace(s.workingDir)
	env, err := nativeCLIACPEnv(provider, env, cwd)
	if err != nil {
		return nil, err
	}
	if s.usesNativeCodexDiscovery(provider) {
		return s.listCodex(ctx, env, timeout, now)
	}
	inspection, err := s.acpProbe.InspectModels(ctx, ACPModelProbeRequest{
		ProviderID: s.providerID,
		Command:    strings.TrimSpace(provider.Command),
		Cwd:        cwd,
		Env:        append([]string(nil), env...),
		Timeout:    timeout,
	})
	if err != nil {
		return nil, err
	}
	modelOption, ok := acp.ModelConfigOption(inspection.Options)
	if !ok {
		return nil, fmt.Errorf(
			"model catalog: %s ACP session did not advertise a model select option",
			s.providerID,
		)
	}
	rows := acpModelRows(
		s.providerID,
		liveModelMappings(s.providerID, provider),
		s.adapter.parseACPModelRows,
		modelOption,
		now,
	)
	if len(rows) == 0 {
		return nil, fmt.Errorf(
			"model catalog: %s ACP model option did not advertise model values",
			s.providerID,
		)
	}
	for index := range rows {
		applyACPModelReasoning(&rows[index], inspection.Models)
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

func acpModelRows(
	providerID string,
	models compozyconfig.ProviderModelsConfig,
	parse func(string, compozyconfig.ProviderModelsConfig, acp.SessionConfigOption, time.Time) []ModelRow,
	modelOption acp.SessionConfigOption,
	now time.Time,
) []ModelRow {
	if parse != nil {
		return parse(providerID, models, modelOption, now)
	}
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

func (s *LiveProviderSource) usesNativeCodexDiscovery(provider compozyconfig.ProviderConfig) bool {
	return s.providerID == liveSourcesCodexKey && strings.TrimSpace(provider.Models.Discovery.Command) == "" &&
		providerexec.StrategyFor(provider).NativeCLI.Command == "codex"
}

func liveModelMappings(providerID string, provider compozyconfig.ProviderConfig) compozyconfig.ProviderModelsConfig {
	models := provider.Models
	builtins := compozyconfig.BuiltinProviders()
	if _, builtin := builtins[providerID]; builtin {
		return models
	}
	runtimeProvider := compozyconfig.CanonicalProviderName(provider.RuntimeProvider)
	builtin, ok := builtins[runtimeProvider]
	if !ok {
		return models
	}
	// Seed identities only; the live response remains the sole source of advertised rows.
	seen := make(map[string]bool, len(models.Curated))
	for _, model := range models.Curated {
		seen[model.ID] = true
	}
	models.Curated = append([]compozyconfig.ProviderModelConfig(nil), models.Curated...)
	for _, model := range builtin.Models.Curated {
		if !seen[model.ID] {
			models.Curated = append(models.Curated, model)
		}
	}
	return models
}

func applyACPModelReasoning(row *ModelRow, models map[string][]acp.SessionConfigOption) {
	transportID := row.ModelID
	if len(row.TransportBindings) > 0 {
		transportID = row.TransportBindings[0].TransportModelID
		for _, binding := range row.TransportBindings {
			if binding.TransportModelID != "default" &&
				(transportID == "default" || binding.TransportModelID < transportID) {
				transportID = binding.TransportModelID
			}
		}
	}
	options, inspected := models[transportID]
	if !inspected {
		return
	}
	for _, option := range options {
		if option.ReadOnly || option.Category == "mode" || option.ID == "mode" {
			continue
		}
		descriptor := ModelOptionDescriptor{ID: option.ID, Label: option.Label, Description: option.Description,
			Category: option.Category, Kind: ModelOptionKind(option.Kind), CurrentValueID: option.CurrentValueID,
			CurrentBool: cloneBoolPtr(option.CurrentBool)}
		for index, value := range option.Values {
			descriptor.Values = append(descriptor.Values, ModelOptionValue{ValueID: value.Value, Label: value.Label,
				Description: value.Description, GroupID: value.GroupID, GroupLabel: value.GroupLabel, Order: index})
		}
		row.ConfigOptions = append(row.ConfigOptions, descriptor)
	}
	option, ok := acp.ReasoningConfigOption(options)
	if !ok || option.ReadOnly {
		return
	}
	efforts := make([]string, 0, len(option.Values))
	for _, value := range option.Values {
		if value.Value != "default" {
			efforts = append(efforts, value.Value)
		}
	}
	row.ReasoningEfforts = normalizedReasoningEfforts(efforts)
	row.SupportsReasoning = new(true)
	if option.CurrentValueID != "default" {
		row.DefaultReasoningEffort = normalizedDefaultReasoningEffort(option.CurrentValueID)
	}
}
