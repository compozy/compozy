package vectordb

import (
	"math"
	"slices"
)

const defaultTopK = 5

func cosineSimilarity(vecA, vecB []float32) float64 {
	var dot float64
	var magA float64
	var magB float64
	for i := range vecA {
		av := float64(vecA[i])
		bv := float64(vecB[i])
		dot += av * bv
		magA += av * av
		magB += bv * bv
	}
	denom := math.Sqrt(magA) * math.Sqrt(magB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func metadataMatches(meta map[string]any, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	for key, expected := range filters {
		val, ok := meta[key]
		if !ok {
			return false
		}
		switch actual := val.(type) {
		case string:
			if actual != expected {
				return false
			}
		case []string:
			if !containsString(actual, expected) {
				return false
			}
		case []any:
			if !containsAnyString(actual, expected) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func containsString(values []string, expected string) bool {
	return slices.Contains(values, expected)
}

func containsAnyString(values []any, expected string) bool {
	for i := range values {
		s, ok := values[i].(string)
		if !ok {
			continue
		}
		if s == expected {
			return true
		}
	}
	return false
}
