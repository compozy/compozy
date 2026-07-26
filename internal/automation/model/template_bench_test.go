package model

import "testing"

func BenchmarkValidateTriggerPromptTemplate(b *testing.B) {
	benchmarks := []struct {
		name   string
		prompt string
	}{
		{
			name:   "Static",
			prompt: "Summarize the latest session activity.",
		},
		{
			name:   "Template",
			prompt: "Kind={{.Data.kind}} Step={{index .Data.metadata \"step\"}} Workspace={{.WorkspaceID}} Source={{.Source}}",
		},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				if err := ValidateTriggerPromptTemplate(benchmark.prompt); err != nil {
					b.Fatalf("ValidateTriggerPromptTemplate() error = %v", err)
				}
			}
		})
	}
}
