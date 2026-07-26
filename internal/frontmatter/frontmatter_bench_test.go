package frontmatter

import (
	"strings"
	"testing"
)

var (
	benchmarkSplitParts Parts
	benchmarkDecodeBody string

	benchmarkContentLF = []byte(strings.Join([]string{
		"---",
		"name: shared",
		"description: parser benchmark",
		"owner: agh",
		"scope: internal",
		"tags:",
		"  - parser",
		"  - benchmark",
		"---",
		"# Heading",
		"",
		"This package parses markdown frontmatter for several internal callers.",
		"It normalizes line endings and splits metadata from the body.",
		"",
		"Body line 1",
		"Body line 2",
		"Body line 3",
		"Body line 4",
		"Body line 5",
		"Body line 6",
		"Body line 7",
		"Body line 8",
	}, "\n"))
	benchmarkContentCRLF = []byte(strings.ReplaceAll(string(benchmarkContentLF), "\n", "\r\n"))
)

func BenchmarkSplitLF(b *testing.B) {
	benchmarkSplit(b, benchmarkContentLF)
}

func BenchmarkSplitCRLF(b *testing.B) {
	benchmarkSplit(b, benchmarkContentCRLF)
}

func BenchmarkDecodeLF(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchmarkContentLF)))

	for b.Loop() {
		body, err := Decode(benchmarkContentLF, func([]byte) error {
			return nil
		})
		if err != nil {
			b.Fatalf("Decode() error = %v", err)
		}
		benchmarkDecodeBody = body
	}
}

func benchmarkSplit(b *testing.B, content []byte) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(content)))

	for b.Loop() {
		parts, err := Split(content)
		if err != nil {
			b.Fatalf("Split() error = %v", err)
		}
		benchmarkSplitParts = parts
	}
}
