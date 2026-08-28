package session

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

const cursorRuntimeProvider = "cursor"

func (m *Manager) validateRuntimeModelAtAdmission(
	ctx context.Context,
	session *Session,
	selection RuntimeSelection,
) error {
	providerID := strings.TrimSpace(selection.Provider)
	if providerID != cursorRuntimeProvider {
		return nil
	}
	if session != nil {
		snapshot := session.runtimeBindingSnapshot()
		if snapshot.process != nil &&
			strings.TrimSpace(snapshot.selection.Provider) == providerID &&
			runtimeSelectionsEqual(snapshot.selection, selection) {
			return nil
		}
	}
	_, err := m.resolveCatalogTransportModel(
		ctx,
		selection,
		modelCatalogExecutionContextForSession(session),
	)
	return err
}

func (m *Manager) resolveCursorCatalogBinding(
	ctx context.Context,
	selection RuntimeSelection,
	executionContext modelcatalog.CatalogExecutionContext,
) (string, error) {
	models, err := m.listLiveProviderModels(ctx, cursorRuntimeProvider, executionContext)
	if err != nil {
		return "", err
	}
	logicalModels := make([]acp.SessionConfigOptionValue, 0, len(models))
	for _, model := range models {
		logicalModels = append(logicalModels, acp.SessionConfigOptionValue{Value: model.ModelID})
		if strings.TrimSpace(model.ModelID) == strings.TrimSpace(selection.Model) {
			binding, bindingErr := selectCursorTransportBinding(model, selection)
			if bindingErr != nil {
				return "", bindingErr
			}
			return binding.TransportModelID, nil
		}
	}
	validationErr := acp.ValidateModelConfigValue([]acp.SessionConfigOption{{
		ID:       sessionModelConfigKey,
		Category: sessionModelConfigKey,
		Kind:     acp.SessionConfigOptionKindSelect,
		Values:   logicalModels,
	}}, selection.Model)
	return "", fmt.Errorf(
		"session: Cursor model %q is not advertised by the live ACP catalog: %w",
		selection.Model,
		validationErr,
	)
}

func (m *Manager) resolveClaudeCatalogBinding(
	ctx context.Context,
	selection RuntimeSelection,
	executionContext modelcatalog.CatalogExecutionContext,
) (string, error) {
	modelID := strings.TrimSpace(selection.Model)
	models, err := m.listLiveProviderModels(ctx, runtimeProviderClaude, executionContext)
	if err != nil {
		if !claudeLogicalModelRequiresCatalogBinding(modelID) {
			return modelID, nil
		}
		return "", err
	}
	logicalModels := make([]acp.SessionConfigOptionValue, 0, len(models))
	for _, model := range models {
		logicalModels = append(logicalModels, acp.SessionConfigOptionValue{Value: model.ModelID})
		if strings.TrimSpace(model.ModelID) != modelID {
			continue
		}
		return selectClaudeTransportModel(model)
	}
	if !claudeLogicalModelRequiresCatalogBinding(modelID) {
		return modelID, nil
	}
	validationErr := acp.ValidateModelConfigValue([]acp.SessionConfigOption{{
		ID:       sessionModelConfigKey,
		Category: sessionModelConfigKey,
		Kind:     acp.SessionConfigOptionKindSelect,
		Values:   logicalModels,
	}}, selection.Model)
	return "", fmt.Errorf(
		"session: Claude model %q is not advertised by the live ACP catalog: %w",
		selection.Model,
		validationErr,
	)
}

func claudeLogicalModelRequiresCatalogBinding(modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	provider, ok := compozyconfig.BuiltinProviders()[runtimeProviderClaude]
	if !ok {
		return false
	}
	for _, model := range provider.Models.Curated {
		if strings.TrimSpace(model.ID) == modelID {
			return true
		}
	}
	return false
}

func (m *Manager) resolveCatalogTransportModel(
	ctx context.Context,
	selection RuntimeSelection,
	executionContext modelcatalog.CatalogExecutionContext,
) (string, error) {
	switch strings.TrimSpace(selection.Provider) {
	case cursorRuntimeProvider:
		return m.resolveCursorCatalogBinding(ctx, selection, executionContext)
	case runtimeProviderClaude:
		return m.resolveClaudeCatalogBinding(ctx, selection, executionContext)
	default:
		return strings.TrimSpace(selection.Model), nil
	}
}

func (m *Manager) listLiveProviderModels(
	ctx context.Context,
	providerID string,
	executionContext modelcatalog.CatalogExecutionContext,
) ([]modelcatalog.Model, error) {
	if m == nil || m.modelCatalog == nil {
		return nil, fmt.Errorf("session: %s model catalog is unavailable", providerID)
	}
	models, err := m.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID:       providerID,
		ExecutionContext: executionContext,
		View:             modelcatalog.CatalogViewAll,
		Refresh:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("session: refresh %s model catalog: %w", providerID, err)
	}
	live := make([]modelcatalog.Model, 0, len(models))
	for _, model := range models {
		if strings.TrimSpace(model.ProviderID) != providerID ||
			model.AvailabilityState != modelcatalog.AvailabilityStateAvailableLive ||
			!providerLiveSourcePresent(model, providerID) {
			continue
		}
		live = append(live, model)
	}
	return live, nil
}

func selectClaudeTransportModel(model modelcatalog.Model) (string, error) {
	bindings := append([]modelcatalog.ModelTransportBinding(nil), model.TransportBindings...)
	slices.SortFunc(bindings, func(left, right modelcatalog.ModelTransportBinding) int {
		leftDefault := strings.EqualFold(strings.TrimSpace(left.TransportModelID), "default")
		rightDefault := strings.EqualFold(strings.TrimSpace(right.TransportModelID), "default")
		if leftDefault != rightDefault {
			if leftDefault {
				return 1
			}
			return -1
		}
		return cmp.Compare(strings.TrimSpace(left.TransportModelID), strings.TrimSpace(right.TransportModelID))
	})
	for _, binding := range bindings {
		if transportModel := strings.TrimSpace(binding.TransportModelID); transportModel != "" {
			return transportModel, nil
		}
	}
	return "", fmt.Errorf("session: Claude model %q has no live transport binding", model.ModelID)
}

func selectCursorTransportBinding(
	model modelcatalog.Model,
	selection RuntimeSelection,
) (modelcatalog.ModelTransportBinding, error) {
	effort := strings.TrimSpace(selection.ReasoningEffort)
	if effort == "" && model.DefaultReasoningEffort != nil {
		effort = string(*model.DefaultReasoningEffort)
	}
	wantFast := selection.Speed == speedpkg.SpeedFast
	wantedOptions, err := cursorModelOptionSelections(model, selection.ACPOptions)
	if err != nil {
		return modelcatalog.ModelTransportBinding{}, err
	}
	matches := make([]modelcatalog.ModelTransportBinding, 0, 1)
	for _, binding := range model.TransportBindings {
		if !cursorBindingMatches(binding, effort, wantFast, wantedOptions) {
			continue
		}
		matches = append(matches, binding)
	}
	if len(matches) != 1 {
		return modelcatalog.ModelTransportBinding{}, fmt.Errorf(
			"session: Cursor model %q has %d live transport bindings for reasoning %q and fast=%t",
			model.ModelID,
			len(matches),
			effort,
			wantFast,
		)
	}
	return matches[0], nil
}

func cursorBindingMatches(
	binding modelcatalog.ModelTransportBinding,
	effort string,
	wantFast bool,
	wantedOptions []modelcatalog.ModelOptionSelection,
) bool {
	if effort == "" {
		if binding.ReasoningEffort != nil {
			return false
		}
	} else if binding.ReasoningEffort == nil || string(*binding.ReasoningEffort) != effort {
		return false
	}
	if binding.Fast == nil {
		if wantFast {
			return false
		}
	} else if *binding.Fast != wantFast {
		return false
	}
	if !cursorBindingOptionSelectionsMatch(binding.OptionSelections, wantedOptions) {
		return false
	}
	thinking, hasThinking := cursorBooleanOptionSelection(wantedOptions, "thinking")
	if binding.Thinking != nil {
		return *binding.Thinking == thinking
	}
	return !hasThinking || !thinking
}

func cursorModelOptionSelections(
	model modelcatalog.Model,
	requested []acp.SessionConfigOptionSelection,
) ([]modelcatalog.ModelOptionSelection, error) {
	selected := make(map[string]modelcatalog.ModelOptionSelection, len(model.ConfigOptions))
	descriptors := make(map[string]modelcatalog.ModelOptionDescriptor, len(model.ConfigOptions))
	for _, descriptor := range model.ConfigOptions {
		id := strings.TrimSpace(descriptor.ID)
		if id == "" {
			continue
		}
		descriptors[id] = descriptor
		switch {
		case descriptor.Kind == modelcatalog.ModelOptionKindSelect && strings.TrimSpace(descriptor.CurrentValueID) != "":
			selected[id] = modelcatalog.ModelOptionSelection{
				ID:      id,
				ValueID: strings.TrimSpace(descriptor.CurrentValueID),
			}
		case descriptor.Kind == modelcatalog.ModelOptionKindBoolean && descriptor.CurrentBool != nil:
			selected[id] = modelcatalog.ModelOptionSelection{ID: id, BoolValue: new(*descriptor.CurrentBool)}
		}
	}
	for _, option := range requested {
		id := strings.TrimSpace(option.ID)
		if isDedicatedCursorOptionID(id) {
			return nil, fmt.Errorf("session: Cursor ACP option %q duplicates a dedicated runtime setting", id)
		}
		descriptor, ok := descriptors[id]
		if !ok {
			return nil, fmt.Errorf("session: Cursor ACP option %q is not advertised for model %q", id, model.ModelID)
		}
		candidate := modelcatalog.ModelOptionSelection{ID: id, ValueID: strings.TrimSpace(option.ValueID)}
		if option.BoolValue != nil {
			candidate.BoolValue = new(*option.BoolValue)
		}
		if err := validateCursorModelOptionSelection(descriptor, candidate); err != nil {
			return nil, err
		}
		selected[id] = candidate
	}
	result := make([]modelcatalog.ModelOptionSelection, 0, len(selected))
	for _, option := range selected {
		result = append(result, option)
	}
	slices.SortFunc(result, func(left, right modelcatalog.ModelOptionSelection) int {
		return strings.Compare(left.ID, right.ID)
	})
	return result, nil
}

func validateCursorModelOptionSelection(
	descriptor modelcatalog.ModelOptionDescriptor,
	selection modelcatalog.ModelOptionSelection,
) error {
	if err := modelcatalog.ValidateModelOptionSelection(selection); err != nil {
		return err
	}
	switch descriptor.Kind {
	case modelcatalog.ModelOptionKindBoolean:
		if selection.BoolValue == nil {
			return fmt.Errorf("session: Cursor ACP option %q requires a boolean value", descriptor.ID)
		}
	case modelcatalog.ModelOptionKindSelect:
		for _, value := range descriptor.Values {
			if strings.TrimSpace(value.ValueID) == selection.ValueID {
				return nil
			}
		}
		return fmt.Errorf(
			"session: Cursor ACP option %q does not allow value %q",
			descriptor.ID,
			selection.ValueID,
		)
	default:
		return fmt.Errorf("session: Cursor ACP option %q has unsupported kind %q", descriptor.ID, descriptor.Kind)
	}
	return nil
}

func isDedicatedCursorOptionID(id string) bool {
	switch strings.TrimSpace(id) {
	case "model", "reasoning_effort", "effort", "fast", "speed":
		return true
	default:
		return false
	}
}

func cursorBindingOptionSelectionsMatch(
	binding []modelcatalog.ModelOptionSelection,
	wanted []modelcatalog.ModelOptionSelection,
) bool {
	if len(binding) != len(wanted) {
		return false
	}
	for _, expected := range wanted {
		matched := false
		for _, candidate := range binding {
			if candidate.ID != expected.ID || candidate.ValueID != expected.ValueID ||
				(candidate.BoolValue == nil) != (expected.BoolValue == nil) {
				continue
			}
			if candidate.BoolValue == nil || *candidate.BoolValue == *expected.BoolValue {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func cursorBooleanOptionSelection(
	selections []modelcatalog.ModelOptionSelection,
	id string,
) (bool, bool) {
	for _, selection := range selections {
		if selection.ID == id && selection.BoolValue != nil {
			return *selection.BoolValue, true
		}
	}
	return false, false
}

func (m *Manager) validateExplicitStartModel(
	ctx context.Context,
	runtime *sessionStartRuntime,
	spec *sessionStartSpec,
) error {
	if runtime == nil || spec == nil {
		return nil
	}
	providerID := strings.TrimSpace(runtime.agent.RuntimeProvider)
	if providerID == "" {
		providerID = strings.TrimSpace(runtime.agent.Provider)
	}
	if providerID != cursorRuntimeProvider && providerID != runtimeProviderClaude {
		return nil
	}
	modelID := preferredACPModel(runtime.agent, startModelSelectionIsExplicit(spec, runtime.agent))
	if strings.TrimSpace(modelID) == "" {
		return nil
	}
	transportModel, err := m.resolveCatalogTransportModel(ctx, RuntimeSelection{
		Provider:        providerID,
		Model:           modelID,
		ReasoningEffort: spec.reasoningEffort,
		Speed:           spec.speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(spec.acpOptions),
	}, modelCatalogExecutionContext(spec.profileID, spec.workspace.ID))
	if err != nil {
		return fmt.Errorf("session: validate %s create model: %w", providerID, err)
	}
	spec.transportModel = transportModel
	return nil
}

func modelCatalogExecutionContextForSession(session *Session) modelcatalog.CatalogExecutionContext {
	if session == nil {
		return modelCatalogExecutionContext("", "")
	}
	return modelCatalogExecutionContext(session.ProfileID, session.WorkspaceID)
}

func modelCatalogExecutionContext(profileID string, workspaceID string) modelcatalog.CatalogExecutionContext {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return modelcatalog.CatalogExecutionContext{
			Scope:     modelcatalog.ExecutionScopeProfile,
			ProfileID: profileID,
		}
	}
	return modelcatalog.CatalogExecutionContext{
		Scope:       modelcatalog.ExecutionScopeWorkspace,
		ProfileID:   profileID,
		WorkspaceID: workspaceID,
	}
}

func isCursorRuntimeSelection(selection RuntimeSelection) bool {
	return strings.TrimSpace(selection.Provider) == cursorRuntimeProvider &&
		strings.TrimSpace(selection.Model) != ""
}

func providerLiveSourcePresent(model modelcatalog.Model, providerID string) bool {
	for _, source := range model.Sources {
		if source.SourceID == modelcatalog.SourceKindProviderLiveID(providerID) && !source.Stale {
			return true
		}
	}
	return false
}
