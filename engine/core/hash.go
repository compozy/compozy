package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"
)

// WriteStableJSON writes a canonical JSON-like representation of v into b.
// Objects (map[string]any) have keys sorted recursively to ensure stability.
// Arrays preserve order. Primitive values are marshaled using encoding/json.
func WriteStableJSON(b *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		writeMapStringAny(b, t)
	case []any:
		writeSliceAny(b, t)
	case string:
		bs, err := json.Marshal(t)
		if err == nil {
			b.Write(bs)
		} else {
			b.WriteString("\"")
			b.WriteString(t)
			b.WriteString("\"")
		}
	case float64, bool, nil:
		bs, err := json.Marshal(t)
		if err == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	default:
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			b.WriteString("null")
			return
		}
		if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
			writeReflectedMap(b, rv)
			return
		}
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			writeReflectedSlice(b, rv)
			return
		}
		bs, err := json.Marshal(t)
		if err == nil {
			b.Write(bs)
		} else {
			b.WriteString("null")
		}
	}
}

func writeMapStringAny(b *bytes.Buffer, m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err == nil {
			b.Write(kb)
		} else {
			b.WriteString("\"")
			b.WriteString(k)
			b.WriteString("\"")
		}
		b.WriteByte(':')
		WriteStableJSON(b, m[k])
	}
	b.WriteByte('}')
}

func writeSliceAny(b *bytes.Buffer, s []any) {
	b.WriteByte('[')
	for i, e := range s {
		if i > 0 {
			b.WriteByte(',')
		}
		WriteStableJSON(b, e)
	}
	b.WriteByte(']')
}

func writeReflectedMap(b *bytes.Buffer, rv reflect.Value) {
	keys := rv.MapKeys()
	sk := make([]string, 0, len(keys))
	for i := range keys {
		sk = append(sk, keys[i].String())
	}
	sort.Strings(sk)
	b.WriteByte('{')
	for i, k := range sk {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err == nil {
			b.Write(kb)
		} else {
			b.WriteString("\"")
			b.WriteString(k)
			b.WriteString("\"")
		}
		b.WriteByte(':')
		WriteStableJSON(b, rv.MapIndex(reflect.ValueOf(k)).Interface())
	}
	b.WriteByte('}')
}

func writeReflectedSlice(b *bytes.Buffer, rv reflect.Value) {
	b.WriteByte('[')
	n := rv.Len()
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		WriteStableJSON(b, rv.Index(i).Interface())
	}
	b.WriteByte(']')
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
