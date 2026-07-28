package testutil

import "testing"

func BenchmarkEqualStringSlices(b *testing.B) {
	b.ReportAllocs()

	benchmarks := []struct {
		name  string
		left  []string
		right []string
	}{
		{
			name:  "equal-small",
			left:  []string{"alpha", "beta", "gamma"},
			right: []string{"alpha", "beta", "gamma"},
		},
		{
			name:  "equal-large",
			left:  makeSequenceStrings(256),
			right: makeSequenceStrings(256),
		},
		{
			name:  "mismatch-tail",
			left:  makeSequenceStrings(256),
			right: append(makeSequenceStrings(255), "mismatch"),
		},
		{
			name:  "different-length",
			left:  []string{"alpha", "beta"},
			right: []string{"alpha", "beta", "gamma"},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for b.Loop() {
				if EqualStringSlices(bm.left, bm.right) && len(bm.left) != len(bm.right) {
					b.Fatal("EqualStringSlices reported equality for mismatched lengths")
				}
			}
		})
	}
}

func BenchmarkFreeTCPPort(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		port := FreeTCPPort(b)
		if port <= 0 {
			b.Fatalf("FreeTCPPort() = %d, want positive port", port)
		}
	}
}

func makeSequenceStrings(n int) []string {
	seq := make([]string, n)
	for i := range n {
		seq[i] = "item-" + string(rune('a'+(i%26)))
	}
	return seq
}
