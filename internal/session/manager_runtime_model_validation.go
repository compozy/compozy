package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/modelcatalog"
)

const cursorRuntimeProvider = "cursor"

func (m *Manager) validateRuntimeModelAtAdmission(
	ctx context.Context,
	session *Session,
	selection RuntimeSelection,
) error {
	if !isCursorRuntimeSelection(selection) {
		return nil
	}
	if session != nil {
		snapshot := session.runtimeBindingSnapshot()
		if snapshot.process != nil &&
			strings.TrimSpace(snapshot.selection.Provider) == cursorRuntimeProvider {
			if err := acp.ValidateModelConfigValue(snapshot.acpCaps.ConfigOptions, selection.Model); err != nil {
				return fmt.Errorf("session: Cursor ACP model %q is not advertised: %w", selection.Model, err)
			}
			return nil
		}
	}
	return m.validateCursorCatalogModel(ctx, selection.Model)
}

func (m *Manager) validateReservedRuntimeModel(
	session *Session,
	reservedProcess *AgentProcess,
	selection *RuntimeSelection,
) error {
	if session == nil || selection == nil || !isCursorRuntimeSelection(*selection) {
		return nil
	}
	snapshot := session.runtimeBindingSnapshot()
	if snapshot.process == nil || snapshot.process != reservedProcess ||
		strings.TrimSpace(snapshot.selection.Provider) != cursorRuntimeProvider {
		return nil
	}
	if err := acp.ValidateModelConfigValue(snapshot.acpCaps.ConfigOptions, selection.Model); err != nil {
		return fmt.Errorf("session: Cursor ACP model %q is not advertised: %w", selection.Model, err)
	}
	return nil
}

// validateActiveCursorRuntimeModel rechecks an already-bound Cursor ACP process
// without consulting the catalog. It is safe after an admission reservation
// because it cannot fail due to a provider refresh.
func (m *Manager) validateActiveCursorRuntimeModel(session *Session, selection *RuntimeSelection) error {
	if session == nil || selection == nil || !isCursorRuntimeSelection(*selection) {
		return nil
	}
	snapshot := session.runtimeBindingSnapshot()
	if snapshot.process == nil || strings.TrimSpace(snapshot.selection.Provider) != cursorRuntimeProvider {
		return nil
	}
	if err := acp.ValidateModelConfigValue(snapshot.acpCaps.ConfigOptions, selection.Model); err != nil {
		return fmt.Errorf("session: Cursor ACP model %q is not advertised: %w", selection.Model, err)
	}
	return nil
}

func (m *Manager) validateCursorCatalogModel(ctx context.Context, modelID string) error {
	if m == nil || m.modelCatalog == nil {
		return errors.New("session: Cursor model catalog is unavailable")
	}
	models, err := m.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID: cursorRuntimeProvider,
		Refresh:    true,
	})
	if err != nil {
		return fmt.Errorf("session: refresh Cursor model catalog: %w", err)
	}
	for _, model := range models {
		if strings.TrimSpace(model.ProviderID) != cursorRuntimeProvider ||
			strings.TrimSpace(model.ModelID) != strings.TrimSpace(modelID) {
			continue
		}
		if model.AvailabilityState == modelcatalog.AvailabilityStateAvailableLive && cursorLiveSourcePresent(model) {
			return nil
		}
	}
	return fmt.Errorf("session: Cursor model %q is not advertised by the live ACP catalog", modelID)
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
	if providerID != cursorRuntimeProvider {
		return nil
	}
	modelID := preferredACPModel(runtime.agent, startModelSelectionIsExplicit(spec, runtime.agent))
	if strings.TrimSpace(modelID) == "" {
		return nil
	}
	if err := m.validateCursorCatalogModel(ctx, modelID); err != nil {
		return fmt.Errorf("session: validate Cursor create model: %w", err)
	}
	return nil
}

func isCursorRuntimeSelection(selection RuntimeSelection) bool {
	return strings.TrimSpace(selection.Provider) == cursorRuntimeProvider &&
		strings.TrimSpace(selection.Model) != ""
}

func cursorLiveSourcePresent(model modelcatalog.Model) bool {
	for _, source := range model.Sources {
		if source.SourceID == modelcatalog.SourceKindProviderLiveID(cursorRuntimeProvider) && !source.Stale {
			return true
		}
	}
	return false
}
