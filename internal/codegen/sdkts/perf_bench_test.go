package sdkts

import (
	"reflect"
	"testing"

	"github.com/compozy/agh/internal/hooks"
)

func BenchmarkGenerate(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		out, err := Generate()
		if err != nil {
			b.Fatalf("Generate() error = %v", err)
		}
		if out == "" {
			b.Fatal("Generate() returned empty output")
		}
	}
}

func BenchmarkStructFieldsPromptPayload(b *testing.B) {
	b.ReportAllocs()

	payloadType := reflect.TypeFor[hooks.PromptPayload]()

	for b.Loop() {
		gen, err := newGenerator()
		if err != nil {
			b.Fatalf("newGenerator() error = %v", err)
		}
		gen.prepare()

		fields, err := gen.structFields(payloadType)
		if err != nil {
			b.Fatalf("structFields() error = %v", err)
		}
		if len(fields) == 0 {
			b.Fatal("structFields() returned no fields")
		}
	}
}
