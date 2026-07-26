package extensiontest

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
)

var (
	benchmarkMatrixEntriesSink int
	benchmarkPromptEventsSink  int
	benchmarkReadLinesSink     int
)

func BenchmarkBuildConformanceMatrix(b *testing.B) {
	entries := benchmarkProviderSummaries()

	b.ReportAllocs()

	for b.Loop() {
		matrix := BuildConformanceMatrix(entries...)
		benchmarkMatrixEntriesSink += len(matrix)
	}
}

func BenchmarkScriptedPromptDriverPrompt(b *testing.B) {
	driver := NewScriptedPromptDriver(
		time.Date(2026, 4, 11, 9, 30, 0, 0, time.UTC),
		benchmarkPromptScript(),
	)
	driver.prompts = make([]acp.PromptRequest, 0, b.N)

	proc, err := driver.Start(context.Background(), acp.StartOpts{AgentName: "coder"})
	if err != nil {
		b.Fatalf("ScriptedPromptDriver.Start() error = %v", err)
	}
	defer func() {
		if err := driver.Stop(context.Background(), proc); err != nil {
			b.Fatalf("ScriptedPromptDriver.Stop() error = %v", err)
		}
	}()

	request := acp.PromptRequest{TurnID: "turn-bench"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		events, err := driver.Prompt(context.Background(), proc, request)
		if err != nil {
			b.Fatalf("ScriptedPromptDriver.Prompt() error = %v", err)
		}
		eventCount := 0
		for range events {
			eventCount++
		}
		benchmarkPromptEventsSink += eventCount
	}
}

func BenchmarkReadJSONLinesFileStateRecord(b *testing.B) {
	path := filepath.Join(b.TempDir(), "states.jsonl")
	for idx := range 128 {
		appendJSONLine(b, path, StateRecord{
			BridgeInstanceID: fmt.Sprintf("brg-%03d", idx),
			Status:           testBridgeInstance().Status,
			Instance:         testBridgeInstanceWithID(fmt.Sprintf("brg-%03d", idx)),
		})
	}

	b.ReportAllocs()

	for b.Loop() {
		records, err := readJSONLinesFile[StateRecord](path)
		if err != nil {
			b.Fatalf("readJSONLinesFile() error = %v", err)
		}
		benchmarkReadLinesSink += len(records)
	}
}

func benchmarkPromptScript() []ScriptedPromptEvent {
	return []ScriptedPromptEvent{
		{Type: acp.EventTypeAgentMessage, Text: "hello"},
		{Type: acp.EventTypeAgentMessage, Text: " world"},
		{Type: acp.EventTypeDone},
	}
}

func benchmarkProviderSummaries() []ProviderConformanceSummary {
	entries := make([]ProviderConformanceSummary, 0, 48)
	for providerIdx := range 12 {
		provider := fmt.Sprintf("provider-%02d", providerIdx%4)
		platform := fmt.Sprintf("platform-%02d", providerIdx%3)
		for variantIdx := range 4 {
			entries = append(entries, ProviderConformanceSummary{
				Provider: provider,
				Platform: platform,
				Targets: []CoverageTarget{
					CoverageTargetMultiInstance,
					CoverageTargetRestartRecovery,
					CoverageTargetAuthDegradation,
				},
				ManagedInstances: []ManagedInstanceOutcome{
					{
						InstanceID:  fmt.Sprintf("%s-%s-a-%d", provider, platform, variantIdx),
						FinalStatus: testBridgeInstance().Status,
					},
					{
						InstanceID:  fmt.Sprintf("%s-%s-b-%d", provider, platform, variantIdx),
						FinalStatus: testBridgeInstance().Status,
					},
				},
			})
		}
	}
	return entries
}
