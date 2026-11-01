package core

import (
	"context"
	"sync"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/doctor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// Keep independent diagnostic outcomes while reusing one session inventory.
func (h *BaseHandlers) doctorPayload(ctx context.Context, opts doctor.RunOptions) (contract.DoctorPayload, error) {
	start := h.nowUTC()
	registry := doctor.NewRegistry()
	loadSessions := h.doctorSessionLoader()
	if err := registry.Register(&doctor.ProbeFunc{
		ProbeID:       "runtime.status",
		ProbeCategory: contract.CategoryDaemon,
		RunFunc: func(ctx context.Context, _ *doctor.ProbeEnv) ([]contract.DiagnosticItem, error) {
			status, err := h.statusPayloadWithAggregate(ctx, "",
				func(ctx context.Context) (contract.SessionAggregatePayload, error) {
					infos, err := loadSessions(ctx)
					return aggregateSessionInfos(infos), err
				},
			)
			if err != nil {
				return nil, err
			}
			return diagnosticItemsFromStatus(&status, false), nil
		},
	}); err != nil {
		return contract.DoctorPayload{}, err
	}
	if err := h.registerProviderDoctorProbes(registry, loadSessions); err != nil {
		return contract.DoctorPayload{}, err
	}
	if err := h.registerExtensionDoctorProbe(registry); err != nil {
		return contract.DoctorPayload{}, err
	}
	if h.RuntimeMemory != nil {
		if err := registry.Register(&doctor.RuntimeMemoryProbe{Source: h.RuntimeMemory}); err != nil {
			return contract.DoctorPayload{}, err
		}
	}
	if h.HostLoad != nil {
		if err := registry.Register(&doctor.HostLoadProbe{Source: h.HostLoad}); err != nil {
			return contract.DoctorPayload{}, err
		}
	}
	if source, ok := h.Sessions.(doctor.SubprocessHealthSnapshotSource); ok {
		if err := registry.Register(&doctor.SubprocessHealthProbe{Source: source}); err != nil {
			return contract.DoctorPayload{}, err
		}
	}
	if err := h.registerSessionMetadataDoctorProbe(registry); err != nil {
		return contract.DoctorPayload{}, err
	}
	if h.DeadEntities != nil && h.Workspaces != nil {
		for _, kind := range []store.DeadEntityKind{
			store.DeadEntityKindMCPSidecar,
			store.DeadEntityKindExtension,
		} {
			if err := registry.Register(&doctor.DeadEntityProbe{
				Source:     h.DeadEntities,
				Workspaces: h.Workspaces,
				Kind:       kind,
			}); err != nil {
				return contract.DoctorPayload{}, err
			}
		}
	}

	runner := doctor.NewRunner(registry)
	items, err := runner.Run(ctx, opts)
	if err != nil {
		return contract.DoctorPayload{}, err
	}
	generatedAt := h.nowUTC()
	return contract.DoctorPayload{
		SchemaVersion: contract.StatusSchemaVersion,
		GeneratedAt:   generatedAt,
		DurationMS:    max(generatedAt.Sub(start).Milliseconds(), 0),
		Status:        diagnosticStatus(items),
		Summary:       doctorSummary(items),
		Items:         items,
	}, nil
}

func (h *BaseHandlers) registerProviderDoctorProbes(registry *doctor.Registry,
	loadSessions func(context.Context) ([]*session.Info, error)) error {
	if err := registry.Register(&doctor.ProbeFunc{
		ProbeID:       "runtime.providers",
		ProbeCategory: contract.CategoryProvider,
		RunFunc: func(ctx context.Context, _ *doctor.ProbeEnv) ([]contract.DiagnosticItem, error) {
			providers, err := h.providerStatusPayloads(ctx)
			if err != nil {
				return nil, err
			}
			return providerDiagnosticItems(providers), nil
		},
	}); err != nil {
		return err
	}
	if err := registry.Register(&doctor.ProbeFunc{
		ProbeID: "runtime.adapter_startups", ProbeCategory: contract.CategoryProvider,
		RunFunc: func(ctx context.Context, _ *doctor.ProbeEnv) ([]contract.DiagnosticItem, error) {
			return h.adapterStartupDiagnostics(ctx, loadSessions)
		},
	}); err != nil {
		return err
	}
	return nil
}

// Cache only the session inventory; probes retain their own diagnostic outcomes.
func (h *BaseHandlers) doctorSessionLoader() func(context.Context) ([]*session.Info, error) {
	var once sync.Once
	var infos []*session.Info
	var err error
	return func(ctx context.Context) ([]*session.Info, error) {
		once.Do(func() {
			if h.Sessions != nil {
				infos, err = h.Sessions.ListAll(ctx)
			}
		})
		return infos, err
	}
}

// Startup reason codes remain meaningful even when the failure summary is truncated.
func (h *BaseHandlers) adapterStartupDiagnostics(ctx context.Context,
	loadSessions func(context.Context) ([]*session.Info, error)) ([]contract.DiagnosticItem, error) {
	if h.Sessions == nil {
		return nil, nil
	}
	infos, err := loadSessions(ctx)
	if err != nil {
		return nil, err
	}
	var items []contract.DiagnosticItem
	for _, info := range infos {
		if info == nil || info.State != session.StateStopped || info.Failure == nil {
			continue
		}
		if item, ok := acp.NPXStartupDiagnostic(info.Failure); ok {
			item.ID = "doctor.session." + info.ID + ".adapter"
			items = append(items, item)
		}
	}
	return items, nil
}

func (h *BaseHandlers) registerSessionMetadataDoctorProbe(registry *doctor.Registry) error {
	if source, ok := h.Sessions.(doctor.SessionMetadataHealthSource); ok {
		return registry.Register(&doctor.SessionMetadataProbe{Source: source})
	}
	return nil
}
