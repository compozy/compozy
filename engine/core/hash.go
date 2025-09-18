package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// WriteStableJSON writes a canonical JSON-like representation of v into b.
// Objects (map[string]any) have keys sorted recursively to ensure stability.
// Arrays preserve order. Primitive values are marshaled using encoding/json.
func WriteStableJSON(b *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			if bs, err := json.Marshal(k); err == nil {
				b.Write(bs)
			} else {
				b.WriteString("\"")
				b.WriteString(k)
				b.WriteString("\"")
			}
			b.WriteByte(':')
			WriteStableJSON(b, t[k])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			WriteStableJSON(b, e)
		}
		b.WriteByte(']')
	case string:
		if bs, err := json.Marshal(t); err == nil {
			b.Write(bs)
		} else {
			b.WriteString("\"")
			b.WriteString(t)
			b.WriteString("\"")
		}
	case float64, bool, nil:
		if bs, err := json.Marshal(t); err == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	default:
		if bs, err := json.Marshal(t); err == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	}
}

// StableJSONBytes returns the canonical JSON-like bytes for v using WriteStableJSON.
func StableJSONBytes(v any) []byte {
	var b bytes.Buffer
	WriteStableJSON(&b, v)
	return b.Bytes()
}

// ETagFromAny returns a deterministic SHA-256 hex digest of the canonical
// JSON-like form of v. This is used to fingerprint resource values.
func ETagFromAny(v any) string {
	sum := sha256.Sum256(StableJSONBytes(v))
	return hex.EncodeToString(sum[:])
}
