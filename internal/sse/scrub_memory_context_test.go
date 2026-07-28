package sse

import (
	"strings"
	"testing"
)

func TestScrubMemoryContextStringUnicodePrefixContract(t *testing.T) {
	t.Parallel()

	unicodePrefix := strings.Repeat("Ⱥ", 32)
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Should remove literal memory context fences after unicode prefix",
			in:   unicodePrefix + ` <memory-context>secret</memory-context> tail`,
			want: unicodePrefix + " " + MemoryContextRedaction + " tail",
		},
		{
			name: "Should remove JSON escaped memory context fences after unicode prefix",
			in:   unicodePrefix + ` {"text":"\u003cmemory-context\u003esecret\u003c/memory-context\u003e"}`,
			want: unicodePrefix + ` {"text":"` + MemoryContextRedaction + `"}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ScrubMemoryContextString(tt.in)
			if strings.Contains(got, "secret") ||
				strings.Contains(got, "<memory-context") ||
				strings.Contains(got, "</memory-context") ||
				strings.Contains(got, `\u003cmemory-context`) {
				t.Fatalf("ScrubMemoryContextString() = %q, want no secret or fence fragments", got)
			}
			if got != tt.want {
				t.Fatalf("ScrubMemoryContextString() = %q, want %q", got, tt.want)
			}
		})
	}
}
