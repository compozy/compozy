package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/compozy/agh/internal/resources"
)

var (
	benchmarkCanonicalToolJSON = []byte(`{
		"id":"agh__skill_view",
		"display_title":"Skill View",
		"description":"View one skill",
		"backend":{"kind":"native_go","native_name":"skill_view"},
		"input_schema":{"type":"object","properties":{"path":{"type":"string"}}},
		"source":{"kind":"builtin","owner":"daemon"},
		"visibility":"model",
		"risk":"read",
		"read_only":true,
		"concurrency_safe":true
	}`)
	benchmarkToolScope = resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	benchmarkToolSpec  = Tool{
		ID:           "ext__linear__search",
		DisplayTitle: "Search",
		Description:  "Search files",
		Backend: BackendRef{
			Kind:        BackendExtensionHost,
			ExtensionID: "linear",
			Handler:     "search",
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		Source: SourceRef{
			Kind:  SourceExtension,
			Owner: "linear",
		},
		Visibility: VisibilityOperator,
		Risk:       RiskRead,
		ReadOnly:   true,
	}
)

func BenchmarkSourceKindString(b *testing.B) {
	b.ReportAllocs()

	source := SourceExtension
	for b.Loop() {
		if source.String() == "" {
			b.Fatal("SourceKind.String() returned empty string")
		}
	}
}

func BenchmarkToolIDMarshalText(b *testing.B) {
	b.ReportAllocs()

	id := ToolID("mcp__github__create_issue")
	for b.Loop() {
		data, err := id.MarshalText()
		if err != nil {
			b.Fatalf("ToolID.MarshalText() error = %v", err)
		}
		if len(data) == 0 {
			b.Fatal("ToolID.MarshalText() returned empty data")
		}
	}
}

func BenchmarkToolIDUnmarshalText(b *testing.B) {
	b.ReportAllocs()

	text := []byte("mcp__github__create_issue")
	for b.Loop() {
		var id ToolID
		if err := (&id).UnmarshalText(text); err != nil {
			b.Fatalf("ToolID.UnmarshalText() error = %v", err)
		}
	}
}

func BenchmarkToolUnmarshalJSON(b *testing.B) {
	b.ReportAllocs()

	benchmarks := []struct {
		name string
		data []byte
	}{
		{name: "canonical", data: benchmarkCanonicalToolJSON},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var tool Tool
				if err := json.Unmarshal(bench.data, &tool); err != nil {
					b.Fatalf("json.Unmarshal(Tool) error = %v", err)
				}
			}
		})
	}
}

func BenchmarkValidateToolSpec(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	for b.Loop() {
		if _, err := validateToolSpec(ctx, benchmarkToolScope, benchmarkToolSpec); err != nil {
			b.Fatalf("validateToolSpec() error = %v", err)
		}
	}
}
