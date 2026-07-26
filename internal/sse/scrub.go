package sse

import "strings"

// MemoryContextRedaction is emitted when prompt-only memory fences are removed.
const MemoryContextRedaction = "[memory-context redacted]"

var memoryContextOpenMarkers = []string{
	"<memory-context",
	"<memory_context",
	"\\u003cmemory-context",
	"\\u003cmemory_context",
}

var memoryContextCloseMarkers = []string{
	"</memory-context>",
	"</memory_context>",
	"\\u003c/memory-context\\u003e",
	"\\u003c/memory_context\\u003e",
}

// ScrubMemoryContextBytes removes prompt-only memory fences from raw SSE bytes.
func ScrubMemoryContextBytes(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	scrubbed := ScrubMemoryContextString(string(raw))
	if scrubbed == string(raw) {
		return raw
	}
	return []byte(scrubbed)
}

// ScrubMemoryContextString removes literal and JSON-escaped memory context fences.
func ScrubMemoryContextString(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}

	result := value
	for {
		start, ok := nextMemoryContextOpen(result)
		if !ok {
			return result
		}
		end := len(result)
		if closeStart, closeLen, found := nextMemoryContextClose(result[start:]); found {
			end = start + closeStart + closeLen
		}
		result = result[:start] + MemoryContextRedaction + result[end:]
	}
}

func nextMemoryContextOpen(value string) (int, bool) {
	best := -1
	for candidate := 0; candidate < len(value); candidate++ {
		for _, marker := range memoryContextOpenMarkers {
			if asciiEqualFoldPrefix(value[candidate:], marker) &&
				memoryContextOpenBoundary(value, candidate+len(marker)) &&
				(best < 0 || candidate < best) {
				best = candidate
			}
		}
	}
	if best < 0 {
		return 0, false
	}
	return best, true
}

func memoryContextOpenBoundary(value string, after int) bool {
	if after >= len(value) {
		return true
	}
	switch value[after] {
	case '>', '/', ' ', '\t', '\r', '\n':
		return true
	default:
		return asciiEqualFoldPrefix(value[after:], "\\u003e")
	}
}

func nextMemoryContextClose(value string) (int, int, bool) {
	best := -1
	bestLen := 0
	for candidate := 0; candidate < len(value); candidate++ {
		for _, marker := range memoryContextCloseMarkers {
			if asciiEqualFoldPrefix(value[candidate:], marker) && (best < 0 || candidate < best) {
				best = candidate
				bestLen = len(marker)
			}
		}
	}
	if best < 0 {
		return 0, 0, false
	}
	return best, bestLen, true
}

func asciiEqualFoldPrefix(value string, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	for idx := 0; idx < len(prefix); idx++ {
		if asciiLower(value[idx]) != asciiLower(prefix[idx]) {
			return false
		}
	}
	return true
}

func asciiLower(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}
