package session

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
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
	modelID := strings.TrimSpace(selection.Model)
	view, err := m.providerCatalogView(ctx, cursorRuntimeProvider, modelID, executionContext)
	if err != nil {
		return m.passThroughUnvalidatedModel(cursorRuntimeProvider, modelID, err), nil
	}
	for _, model := range view.startable {
		if strings.TrimSpace(model.ModelID) != modelID {
			continue
		}
		binding, bindingErr := selectCursorTransportBinding(model, selection)
		if bindingErr != nil {
			return "", bindingErr
		}
		return binding.TransportModelID, nil
	}
	return "", unadvertisedCatalogModelError("Cursor", modelID, view.startable)
}

func (m *Manager) resolveClaudeCatalogBinding(
	ctx context.Context,
	selection RuntimeSelection,
	executionContext modelcatalog.CatalogExecutionContext,
) (string, error) {
	modelID := strings.TrimSpace(selection.Model)
	view, err := m.providerCatalogView(ctx, runtimeProviderClaude, modelID, executionContext)
	if err != nil {
		return m.passThroughUnvalidatedModel(runtimeProviderClaude, modelID, err), nil
	}
	for _, model := range view.startable {
		if strings.TrimSpace(model.ModelID) != modelID {
			continue
		}
		return selectClaudeTransportModel(model)
	}
	// Claude accepts its own raw transport values ("sonnet", "opus"), so an id the catalog
	// carries no row for is not a CompozyOS logical id and needs no binding. Cursor is
	// deliberately stricter: its transport ids are compound aliases that must never stand in
	// for a logical model.
	if !view.known {
		return modelID, nil
	}
	return "", unadvertisedCatalogModelError("Claude", modelID, view.startable)
}

// passThroughUnvalidatedModel keeps an unreachable catalog from becoming the only reason a
// session cannot start: the ACP config negotiation still rejects an id the agent will not accept.
func (m *Manager) passThroughUnvalidatedModel(providerID string, modelID string, cause error) string {
	if m != nil && m.logger != nil {
		m.logger.Warn(
			"session.model_catalog.validation_skipped",
			"provider_id", providerID,
			"model", modelID,
			"error", cause.Error(),
		)
	}
	return modelID
}

func unadvertisedCatalogModelError(
	providerLabel string,
	modelID string,
	models []modelcatalog.Model,
) error {
	values := make([]acp.SessionConfigOptionValue, 0, len(models))
	for _, model := range models {
		values = append(values, acp.SessionConfigOptionValue{Value: model.ModelID})
	}
	validationErr := acp.ValidateModelConfigValue([]acp.SessionConfigOption{{
		ID:       sessionModelConfigKey,
		Category: sessionModelConfigKey,
		Kind:     acp.SessionConfigOptionKindSelect,
		Values:   values,
	}}, modelID)
	return fmt.Errorf(
		"session: %s model %q is not advertised by the live ACP catalog: %w",
		providerLabel,
		modelID,
		validationErr,
	)
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

// providerCatalogView is what one provider's catalog says about the wanted model.
type providerCatalogView struct {
	// startable holds the provider rows a session may launch against right now.
	startable []modelcatalog.Model
	// known reports whether the catalog carries any row for the wanted model id.
	known bool
}

// providerCatalogView reads the same catalog rows the model picker renders, so both surfaces
// agree on what can start. A forced refresh runs only as recovery, when the wanted model has
// no live binding in the stored projection.
func (m *Manager) providerCatalogView(
	ctx context.Context,
	providerID string,
	modelID string,
	executionContext modelcatalog.CatalogExecutionContext,
) (providerCatalogView, error) {
	view, err := m.readProviderCatalog(ctx, providerID, modelID, executionContext, false)
	if err != nil {
		return providerCatalogView{}, err
	}
	if catalogModelsContain(view.startable, modelID) {
		return view, nil
	}
	return m.readProviderCatalog(ctx, providerID, modelID, executionContext, true)
}

func (m *Manager) readProviderCatalog(
	ctx context.Context,
	providerID string,
	modelID string,
	executionContext modelcatalog.CatalogExecutionContext,
	refresh bool,
) (providerCatalogView, error) {
	if m == nil || m.modelCatalog == nil {
		return providerCatalogView{}, fmt.Errorf("session: %s model catalog is unavailable", providerID)
	}
	models, err := m.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID:       providerID,
		ExecutionContext: executionContext,
		View:             modelcatalog.CatalogViewAll,
		Refresh:          refresh,
	})
	if err != nil {
		return providerCatalogView{}, fmt.Errorf("session: list %s model catalog: %w", providerID, err)
	}
	view := providerCatalogView{startable: make([]modelcatalog.Model, 0, len(models))}
	for _, model := range models {
		if strings.TrimSpace(model.ProviderID) != providerID {
			continue
		}
		if strings.TrimSpace(model.ModelID) == modelID {
			view.known = true
		}
		if ok, _ := modelcatalog.ModelStartability(model); !ok {
			continue
		}
		view.startable = append(view.startable, model)
	}
	return view, nil
}

func catalogModelsContain(models []modelcatalog.Model, modelID string) bool {
	for _, model := range models {
		if strings.TrimSpace(model.ModelID) == modelID {
			return true
		}
	}
	return false
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
