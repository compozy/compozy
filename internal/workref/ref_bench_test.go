package workref

import "testing"

func benchmarkPathConstructor(
	b *testing.B,
	constructor func(string, string) PathRef,
) {
	b.Helper()

	for _, tc := range constructorCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				constructor(tc.id, tc.value)
			}
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
			for b.Loop() {
				constructor(tc.id, tc.value)
			}
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
