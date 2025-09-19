package core

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMemoryReference_UnmarshalYAML_Scalar(t *testing.T) {
	var m MemoryReference
	if err := yaml.Unmarshal([]byte("conversation"), &m); err != nil {
		t.Fatalf("unmarshal scalar: %v", err)
	}
	if m.ID != "conversation" {
		t.Fatalf("id mismatch: %q", m.ID)
	}
	if m.Key != "" {
		t.Fatalf("expected empty key on scalar form, got %q", m.Key)
	}
}

func TestMemoryReference_UnmarshalYAML_Object(t *testing.T) {
	data := []byte("id: conv\nkey: user-1\nmode: read-write\n")
	var m MemoryReference
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	if m.ID != "conv" || m.Key != "user-1" || m.Mode != "read-write" {
		t.Fatalf("unexpected values: %+v", m)
	}
}

func TestMemoryReference_UnmarshalJSON_Scalar(t *testing.T) {
	var m MemoryReference
	if err := json.Unmarshal([]byte("\"conversation\""), &m); err != nil {
		t.Fatalf("json scalar: %v", err)
	}
	if m.ID != "conversation" || m.Key != "" {
		t.Fatalf("unexpected: %+v", m)
	}
}
