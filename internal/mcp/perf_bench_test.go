package mcp

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

func BenchmarkHostedStreamProjectionUnchanged(b *testing.B) {
	executable, err := os.Executable()
	if err != nil {
		b.Fatal(err)
	}
	registry := benchmarkHostedRegistry(b, 120)
	peer := hostedTestPeer(executable)
	record := &hostedBindRecord{
		digestMemo: new(hostedProjectionDigestMemo), bindID: "bench-bind", sessionID: "bench-session",
		expectedBin: executable, peer: peer,
	}
	for _, versioned := range []bool{false, true} {
		b.Run(fmt.Sprintf("Versioned%t", versioned), func(b *testing.B) {
			service := benchmarkHostedService(registry, record, func(context.Context, tools.Scope) (string, bool) {
				return "stable", versioned
			})
			service.enabled = true
			service.expectedBinary = executable
			service.now = func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) }
			first, err := service.StreamProjection(b.Context(), record.bindID, peer, "")
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				response, err := service.StreamProjection(b.Context(), record.bindID, peer, first.Digest)
				if err != nil || response.Digest != first.Digest {
					b.Fatalf("stable projection digest = %q, error = %v", response.Digest, err)
				}
			}
		})
	}
}

func BenchmarkHostedProjectionGenerationCache(b *testing.B) {
	registry := benchmarkHostedRegistry(b, 120)
	record := &hostedBindRecord{
		digestMemo:  new(hostedProjectionDigestMemo),
		bindID:      "bench-bind",
		sessionID:   "bench-session",
		workspaceID: "bench-workspace",
		agentName:   "bench-agent",
	}

	b.Run("Uncached", func(b *testing.B) {
		service := benchmarkHostedService(registry, record, nil)
		b.ReportAllocs()
		for b.Loop() {
			if _, err := service.projection(b.Context(), record); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("GenerationCacheHit", func(b *testing.B) {
		service := benchmarkHostedService(
			registry,
			record,
			func(context.Context, tools.Scope) (string, bool) { return "stable", true },
		)
		if _, err := service.projection(b.Context(), record); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			if _, err := service.projection(b.Context(), record); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func benchmarkHostedRegistry(b *testing.B, count int) tools.Registry {
	b.Helper()
	nativeTools := make([]tools.NativeTool, 0, count)
	for index := range count {
		id := tools.ToolID(fmt.Sprintf("compozy__bench_%03d", index))
		nativeTools = append(nativeTools, hostedRuntimeNativeTool(id, tools.RiskRead, true))
	}
	provider, err := tools.NewNativeProvider(tools.BuiltinSource(), nativeTools...)
	if err != nil {
		b.Fatalf("NewNativeProvider() error = %v", err)
	}
	registry, err := tools.NewRegistry(
		tools.WithProviders(provider),
		tools.WithPolicyInputs(tools.DefaultPolicyInputs(), tools.ToolsetCatalog{}),
	)
	if err != nil {
		b.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

func benchmarkHostedService(
	registry tools.Registry,
	record *hostedBindRecord,
	generation HostedProjectionGeneration,
) *HostedService {
	service := &HostedService{
		registry:             func() tools.Registry { return registry },
		projectionGeneration: generation,
		binds:                map[string]*hostedBindRecord{record.bindID: record.clone()},
		projectionCache:      newHostedProjectionCache(),
	}
	return service
}
