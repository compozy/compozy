package contract

import "testing"

var (
	benchmarkJSONSafetyFound bool
	benchmarkJSONSafetyErr   error
)

func BenchmarkContainsRawClaimTokenFieldNestedPayload(b *testing.B) {
	payload := jsonSafetyBenchmarkPayload()
	b.ReportAllocs()

	var found bool
	var err error
	for b.Loop() {
		found, err = ContainsRawClaimTokenField(payload)
		if err != nil {
			b.Fatalf("ContainsRawClaimTokenField() error = %v", err)
		}
	}
	if found {
		b.Fatal("ContainsRawClaimTokenField() found unsafe claim token in safe benchmark payload")
	}
	benchmarkJSONSafetyFound = found
	benchmarkJSONSafetyErr = err
}

func BenchmarkValidateAuthoredContextRedactedNestedPayload(b *testing.B) {
	payload := jsonSafetyBenchmarkPayload()
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = ValidateAuthoredContextRedacted(payload)
		if err != nil {
			b.Fatalf("ValidateAuthoredContextRedacted() error = %v", err)
		}
	}
	benchmarkJSONSafetyErr = err
}

func jsonSafetyBenchmarkPayload() map[string]any {
	sections := make([]any, 0, 24)
	for i := 0; i < cap(sections); i++ {
		sections = append(sections, map[string]any{
			"section_id": "public-section",
			"summary":    "bounded public agent context",
			"items": []any{
				map[string]any{
					"kind": "status",
					"attributes": map[string]any{
						"task_id":    "task-1",
						"run_id":     "run-1",
						"claim_hash": "sha256:public",
						"labels":     []string{"agent", "contract", "public"},
					},
				},
				map[string]any{
					"kind": "peer",
					"attributes": map[string]any{
						"peer_id":      "peer-1",
						"display_name": "codex worker",
						"capabilities": []string{"read", "write", "review"},
					},
				},
			},
		})
	}

	return map[string]any{
		"agent": map[string]any{
			"id":       "agent-1",
			"provider": "openai",
			"model":    "gpt-5.4",
		},
		"workspace": map[string]any{
			"id":       "workspace-1",
			"root_dir": "/workspace/agh",
		},
		"sections": sections,
		"metadata": map[string]any{
			"source":       "benchmark",
			"generated_at": "2026-05-06T00:00:00Z",
			"public_refs": []any{
				map[string]string{"kind": "task", "ref": "task-1"},
				map[string]string{"kind": "session", "ref": "session-1"},
			},
		},
	}
}
