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

func BenchmarkHostedStreamProjectionFanout(b *testing.B) {
	for _, shared := range []bool{false, true} {
		for _, streams := range []int{1, 16} {
			b.Run(fmt.Sprintf("Shared%t/Streams%d", shared, streams), func(b *testing.B) {
				executable, err := os.Executable()
				if err != nil {
					b.Fatal(err)
				}
				nativeTools := make([]tools.NativeTool, 120)
				for i := range nativeTools {
					nativeTools[i] = hostedRuntimeNativeTool(
						tools.ToolID(fmt.Sprintf("compozy__bench_%03d", i)),
						tools.RiskRead,
						true,
					)
				}
				native, err := tools.NewNativeProvider(tools.BuiltinSource(), nativeTools...)
				if err != nil {
					b.Fatal(err)
				}
				provider := &hostedCountingProvider{Provider: native}
				registry, err := tools.NewRegistry(tools.WithProviders(provider))
				if err != nil {
					b.Fatal(err)
				}
				peer := hostedTestPeer(executable)
				record := &hostedBindRecord{
					bindID:      "bind-0",
					sessionID:   "session-0",
					expectedBin: executable,
					peer:        peer,
					digestMemo:  new(hostedProjectionDigestMemo),
				}
				service := benchmarkHostedService(registry, record, nil)
				if !shared {
					service.registry = func() tools.Registry { return hostedUnsharedRegistry{registry} }
				}
				service.enabled, service.expectedBinary = true, executable
				service.now = func() time.Time { return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC) }
				epoch := NewProjectionEpoch()
				service.projectionEpoch = epoch.Current
				binds := make([]string, streams)
				for i := range streams {
					binds[i] = fmt.Sprintf("bind-%d", i)
					boundRecord := record.clone()
					boundRecord.bindID, boundRecord.sessionID = binds[i], fmt.Sprintf("session-%d", i)
					boundRecord.digestMemo = new(hostedProjectionDigestMemo)
					service.binds[binds[i]] = boundRecord
				}
				b.ReportAllocs()
				for b.Loop() {
					epoch.Advance()
					for _, bind := range binds {
						if _, err := service.StreamProjection(b.Context(), bind, peer, ""); err != nil {
							b.Fatal(err)
						}
					}
				}
				b.ReportMetric(float64(provider.calls.Load())/float64(b.N), "builds/epoch")
			})
		}
	}
}

type hostedUnsharedRegistry struct{ tools.Registry }
