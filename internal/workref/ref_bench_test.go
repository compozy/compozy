package workref

import (
	"runtime"
	"testing"
)

var (
	benchmarkPathRefSink PathRef
	benchmarkRootRefSink RootRef
)

func benchmarkPathConstructor(
	b *testing.B,
	constructor func(string, string) PathRef,
) {
	b.Helper()

	for _, tc := range constructorCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var result PathRef
			for i := 0; i < b.N; i++ {
				result = constructor(tc.id, tc.value)
			}
			benchmarkPathRefSink = result
			runtime.KeepAlive(benchmarkPathRefSink)
		})
	}
}

func benchmarkRootConstructor(
	b *testing.B,
	constructor func(string, string) RootRef,
) {
	b.Helper()

	for _, tc := range constructorCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var result RootRef
			for i := 0; i < b.N; i++ {
				result = constructor(tc.id, tc.value)
			}
			benchmarkRootRefSink = result
			runtime.KeepAlive(benchmarkRootRefSink)
		})
	}
}

func BenchmarkConstructors(b *testing.B) {
	b.Run("NewPath", func(b *testing.B) {
		benchmarkPathConstructor(b, NewPath)
	})

	b.Run("NewRoot", func(b *testing.B) {
		benchmarkRootConstructor(b, NewRoot)
	})
}
