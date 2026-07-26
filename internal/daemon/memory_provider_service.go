package daemon

import (
	"context"

	"errors"
	"fmt"

	"os"

	"strings"

	"time"

	"github.com/compozy/agh/internal/api/contract"
	extensionpkg "github.com/compozy/agh/internal/extension"

	localprovider "github.com/compozy/agh/internal/memory/provider/local"

	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type daemonMemoryProviderService struct {
	registry *extensionpkg.MemoryProviderRegistry
}

func (s daemonMemoryProviderService) List(
	ctx context.Context,
	workspaceID string,
) ([]contract.MemoryProviderPayload, error) {
	if s.registry == nil {
		return nil, errors.New("daemon: memory provider registry is not configured")
	}
	active, activeErr := s.registry.Select(ctx, workspaceID, "")
	if activeErr != nil && !isMemoryProviderNotFound(activeErr) {
		return nil, activeErr
	}
	registrations := s.registry.List()
	payloads := make([]contract.MemoryProviderPayload, 0, len(registrations))
	for _, registration := range registrations {
		payloads = append(payloads, memoryProviderPayload(registration, registration.Name == active.Name))
	}
	return payloads, nil
}

func (s daemonMemoryProviderService) Get(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.MemoryProviderPayload, error) {
	if s.registry == nil {
		return contract.MemoryProviderPayload{}, errors.New("daemon: memory provider registry is not configured")
	}
	active, activeErr := s.registry.Select(ctx, workspaceID, "")
	if activeErr != nil && !isMemoryProviderNotFound(activeErr) {
		return contract.MemoryProviderPayload{}, activeErr
	}
	registration, err := s.registry.Select(ctx, workspaceID, name)
	if err != nil {
		if isMemoryProviderNotFound(err) {
			return contract.MemoryProviderPayload{}, fmt.Errorf("%w: %w", os.ErrNotExist, err)
		}
		return contract.MemoryProviderPayload{}, err
	}
	return memoryProviderPayload(registration, registration.Name == active.Name), nil
}

func (s daemonMemoryProviderService) Select(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.MemoryProviderPayload, error) {
	if s.registry == nil {
		return contract.MemoryProviderPayload{}, errors.New("daemon: memory provider registry is not configured")
	}
	if err := s.registry.SetActive(ctx, workspaceID, name); err != nil {
		if isMemoryProviderNotFound(err) {
			return contract.MemoryProviderPayload{}, fmt.Errorf("%w: %w", os.ErrNotExist, err)
		}
		return contract.MemoryProviderPayload{}, err
	}
	return s.Get(ctx, workspaceID, name)
}

func (s daemonMemoryProviderService) Enable(
	ctx context.Context,
	workspaceID string,
	name string,
	_ string,
) (contract.MemoryProviderLifecycleResponse, error) {
	provider, err := s.Get(ctx, workspaceID, name)
	return contract.MemoryProviderLifecycleResponse{Provider: provider, Changed: false}, err
}

func (s daemonMemoryProviderService) Disable(
	ctx context.Context,
	workspaceID string,
	name string,
	_ string,
) (contract.MemoryProviderLifecycleResponse, error) {
	provider, err := s.Get(ctx, workspaceID, name)
	return contract.MemoryProviderLifecycleResponse{Provider: provider, Changed: false}, err
}

func memoryProviderPayload(
	registration extensionpkg.MemoryProviderRegistration,
	active bool,
) contract.MemoryProviderPayload {
	status := contract.MemoryProviderStateStandby
	if active {
		status = contract.MemoryProviderStateActive
	}
	return contract.MemoryProviderPayload{
		Name:    strings.TrimSpace(registration.Name),
		Status:  status,
		Active:  active,
		Builtin: registration.Bundled,
		Tools:   append([]string(nil), registration.ToolNames...),
	}
}

func isMemoryProviderNotFound(err error) bool {
	var typed *extensionpkg.MemoryProviderNotFoundError
	return errors.As(err, &typed)
}

func newDaemonMemoryProviderRegistry(
	ctx context.Context,
	state *bootState,
) (*extensionpkg.MemoryProviderRegistry, error) {
	if state == nil || state.memoryStore == nil || !state.cfg.Memory.Enabled {
		return nil, nil
	}
	opts := []extensionpkg.MemoryProviderRegistryOption{
		extensionpkg.WithMemoryProviderReservedTools(
			toolspkg.ToolIDMemoryList.String(),
			toolspkg.ToolIDMemoryShow.String(),
			toolspkg.ToolIDMemorySearch.String(),
			toolspkg.ToolIDMemoryPropose.String(),
			toolspkg.ToolIDMemoryNote.String(),
		),
		extensionpkg.WithMemoryProviderRegistryClock(func() time.Time {
			return time.Now().UTC()
		}),
	}
	if eventWriter, ok := state.registry.(store.EventSummaryStore); ok {
		opts = append(opts, extensionpkg.WithMemoryProviderEventSummaryStore(eventWriter))
	}
	registry := extensionpkg.NewMemoryProviderRegistry(opts...)
	if state.localMemoryProvider != nil {
		if err := registry.Register(ctx, extensionpkg.MemoryProviderRegistration{
			Name:     localprovider.Name,
			Version:  memoryRuntimeBuiltinKey,
			Provider: state.localMemoryProvider,
			Bundled:  true,
		}); err != nil {
			return nil, err
		}
	}
	activeProvider := firstNonEmptyString(state.cfg.Memory.Provider.Name, localprovider.Name)
	if err := registry.SetActive(ctx, "", activeProvider); err != nil {
		return nil, err
	}
	return registry, nil
}
