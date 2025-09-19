package core

import (
	"testing"
)

func TestETag_TypedMapStringString_IsStable(t *testing.T) {
	a := map[string]string{"b": "2", "a": "1", "c": "3"}
	b := map[string]string{"c": "3", "b": "2", "a": "1"}
	ea := ETagFromAny(a)
	eb := ETagFromAny(b)
	if ea != eb {
		t.Fatalf("expected stable ETag for typed maps, got %s vs %s", ea, eb)
	}
}

func TestETag_TypedMapStringInt_IsStable(t *testing.T) {
	a := map[string]int{"x": 1, "y": 2}
	b := map[string]int{"y": 2, "x": 1}
	ea := ETagFromAny(a)
	eb := ETagFromAny(b)
	if ea != eb {
		t.Fatalf("expected stable ETag for typed map[string]int, got %s vs %s", ea, eb)
	}
}

func TestETag_NestedTypedMaps_AreStable(t *testing.T) {
	a := map[string]map[string]string{"outer": {"b": "2", "a": "1"}}
	b := map[string]map[string]string{"outer": {"a": "1", "b": "2"}}
	ea := ETagFromAny(a)
	eb := ETagFromAny(b)
	if ea != eb {
		t.Fatalf("expected stable ETag for nested typed maps, got %s vs %s", ea, eb)
	}
}
