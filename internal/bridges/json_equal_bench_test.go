package bridges

import "testing"

func BenchmarkSemanticJSONEqual(b *testing.B) {
	cases := []struct {
		name  string
		left  []byte
		right []byte
	}{
		{
			name:  "Canonical",
			left:  []byte(`{"features":{"beta":true},"tenant":"acme"}`),
			right: []byte(`{"features":{"beta":true},"tenant":"acme"}`),
		},
		{
			name:  "Equivalent",
			left:  []byte(`{"tenant":"acme","features":{"beta":true}}`),
			right: []byte(`{"features":{"beta":true},"tenant":"acme"}`),
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				if !semanticJSONEqual(tc.left, tc.right) {
					b.Fatal("semanticJSONEqual() = false, want true")
				}
			}
		})
	}
}
