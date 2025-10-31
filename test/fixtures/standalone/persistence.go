package standalone

import (
	"fmt"
	"strconv"
)

// KVPair represents a simple key/value record used in fixtures.
type KVPair struct {
	Key   string
	Value string
}

// GenerateKVData builds a deterministic dataset with the given size and prefix.
func GenerateKVData(prefix string, n int) []KVPair {
	out := make([]KVPair, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, KVPair{Key: fmt.Sprintf("%s:%d", prefix, i), Value: "v:" + strconv.Itoa(i)})
	}
	return out
}

// ToMap converts a slice of KVPair to map for quick lookups in assertions.
func ToMap(items []KVPair) map[string]string {
	m := make(map[string]string, len(items))
	for _, it := range items {
		m[it.Key] = it.Value
	}
	return m
}
