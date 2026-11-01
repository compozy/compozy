package core

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/doctor"
	"github.com/compozy/compozy/internal/store"
)

func (h *BaseHandlers) doctorPayload(ctx context.Context, opts doctor.RunOptions) (contract.DoctorPayload, error) {
	start := h.nowUTC()
	registry := doctor.NewRegistry()
	if err := registry.Register(&doctor.ProbeFunc{
		ProbeID:       "runtime.status",
		ProbeCategory: contract.CategoryDaemon,
		RunFunc: func(ctx context.Context, _ *doctor.ProbeEnv) ([]contract.DiagnosticItem, error) {
			status, err := h.statusPayload(ctx, "")
			if err != nil {
				return nil, err
			}
			return diagnosticItemsFromStatus(&status, false), nil
		},
	}); err != nil {
		return contract.DoctorPayload{}, err
	}
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
		return contract.DoctorPayload{}, err
	}
	if err := registry.Register(&doctor.ProbeFunc{
		ProbeID: "runtime.adapter_startups", ProbeCategory: contract.CategoryProvider,
		RunFunc: func(ctx context.Context, _ *doctor.ProbeEnv) ([]contract.DiagnosticItem, error) {
			return h.adapterStartupDiagnostics(ctx)
		},
	}); err != nil {
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

func (h *BaseHandlers) adapterStartupDiagnostics(ctx context.Context) ([]contract.DiagnosticItem, error) {
	if h.Sessions == nil {
		return nil, nil
	}
	infos, err := h.Sessions.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	var items []contract.DiagnosticItem
	for _, info := range infos {
		if info == nil || info.Failure == nil {
			continue
		}
		if item, ok := acp.NPXStartupDiagnostic(info.Failure.Summary); ok {
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
