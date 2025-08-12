package core

import (
	"reflect"
	"testing"
)

// helper to check that two maps are deeply equal
func mustDeepEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("not deep equal.\n got: %#v\nwant: %#v", got, want)
	}
}

// helper to check that two references are to different underlying data when mutated
func mutateNestedStructures(m map[string]any) {
	// mutate known nested structures if present
	if v, ok := m["nums"].([]int); ok && len(v) > 0 {
		v[0] = 9999
	}
	if nested, ok := m["nested"].(map[string]any); ok {
		nested["k2"] = "changed"
	}
	if s, ok := m["strs"].([]string); ok && len(s) > 0 {
		s[0] = "ZZZ"
	}
}

func TestDeepCopy_Input_HappyPathDeepSemantics(t *testing.T) {
	orig := Input{
		"a":    1,
		"nums": []int{1, 2, 3},
		"nested": map[string]any{
			"k1": "v1",
		},
		"strs": []string{"x", "y"},
	}

	cpy, err := DeepCopy[Input](orig)
	if err != nil {
		t.Fatalf("DeepCopy(Input) returned error: %v", err)
	}

	// Same content
	mustDeepEqual(t, cpy, orig)

	// Mutate copy and ensure original not affected (deep copy)
	mutateNestedStructures(map[string]any(cpy))

	// After mutations, content must differ from original for mutated fields
	if reflect.DeepEqual(cpy, orig) {
		t.Fatalf("expected deep copy to diverge after mutation, but maps are still equal")
	}

	// Validate original stayed intact
	wantOrig := Input{
		"a":    1,
		"nums": []int{1, 2, 3},
		"nested": map[string]any{
			"k1": "v1",
		},
		"strs": []string{"x", "y"},
	}
	mustDeepEqual(t, orig, wantOrig)

	// Validate type integrity: must be Input, not devolved to plain map
	if _, ok := any(cpy).(Input); !ok {
		t.Fatalf("DeepCopy(Input) did not preserve Input type")
	}
}

func TestDeepCopy_Input_Nil(t *testing.T) {
	var orig Input
	cpy, err := DeepCopy[Input](orig)
	if err != nil {
		t.Fatalf("DeepCopy(nil Input) error: %v", err)
	}
	if cpy != nil {
		t.Fatalf("expected nil result for nil Input, got: %#v", cpy)
	}
}

func TestDeepCopy_Output_HappyPathDeepSemantics(t *testing.T) {
	orig := Output{
		"x":    "y",
		"nums": []int{10, 20},
		"nested": map[string]any{
			"n": 1,
		},
	}

	cpy, err := DeepCopy[Output](orig)
	if err != nil {
		t.Fatalf("DeepCopy(Output) returned error: %v", err)
	}

	mustDeepEqual(t, cpy, orig)

	mutateNestedStructures(map[string]any(cpy))
	if reflect.DeepEqual(cpy, orig) {
		t.Fatalf("expected deep copy to diverge after mutation, but maps are still equal")
	}

	// original remains intact
	wantOrig := Output{
		"x":    "y",
		"nums": []int{10, 20},
		"nested": map[string]any{
			"n": 1,
		},
	}
	mustDeepEqual(t, orig, wantOrig)

	// Type integrity
	if _, ok := any(cpy).(Output); !ok {
		t.Fatalf("DeepCopy(Output) did not preserve Output type")
	}
}

func TestDeepCopy_Output_Nil(t *testing.T) {
	var orig Output
	cpy, err := DeepCopy[Output](orig)
	if err != nil {
		t.Fatalf("DeepCopy(nil Output) error: %v", err)
	}
	if cpy != nil {
		t.Fatalf("expected nil result for nil Output, got: %#v", cpy)
	}
}

func TestDeepCopy_InputPtr_HappyPathDeepSemantics(t *testing.T) {
	v := Input{
		"k":      "v",
		"nums":   []int{1, 2},
		"nested": map[string]any{"a": "b"},
	}
	orig := &v

	cpyPtr, err := DeepCopy[*Input](orig)
	if err != nil {
		t.Fatalf("DeepCopy(*Input) returned error: %v", err)
	}
	if cpyPtr == nil {
		t.Fatalf("DeepCopy(*Input) returned nil pointer")
	}

	// Content equal initially
	mustDeepEqual(t, *cpyPtr, *orig)

	// Mutate the copy
	mutateNestedStructures(map[string]any(*cpyPtr))
	if reflect.DeepEqual(*cpyPtr, *orig) {
		t.Fatalf("expected deep copy to diverge after mutation, but values are still equal")
	}

	// original remains same
	want := Input{
		"k":      "v",
		"nums":   []int{1, 2},
		"nested": map[string]any{"a": "b"},
	}
	mustDeepEqual(t, *orig, want)
}

func TestDeepCopy_InputPtr_SrcNilPointer(t *testing.T) {
	var orig *Input
	cpy, err := DeepCopy[*Input](orig)
	if err != nil {
		t.Fatalf("DeepCopy(nil *Input) error: %v", err)
	}
	if cpy != nil {
		t.Fatalf("expected nil for nil *Input, got: %#v", cpy)
	}
}

func TestDeepCopy_InputPtr_PointedNilMap(t *testing.T) {
	tmp := Input(nil)
	orig := &tmp
	cpy, err := DeepCopy[*Input](orig)
	if err != nil {
		t.Fatalf("DeepCopy(*Input with nil map) error: %v", err)
	}
	if cpy != nil {
		t.Fatalf("expected nil for *Input pointing to nil map, got: %#v", cpy)
	}
}

func TestDeepCopy_OutputPtr_HappyPathDeepSemantics(t *testing.T) {
	v := Output{
		"ok":     true,
		"nested": map[string]any{"k": "v"},
		"nums":   []int{4, 5, 6},
	}
	orig := &v

	cpyPtr, err := DeepCopy[*Output](orig)
	if err != nil {
		t.Fatalf("DeepCopy(*Output) returned error: %v", err)
	}
	if cpyPtr == nil {
		t.Fatalf("DeepCopy(*Output) returned nil pointer")
	}

	mustDeepEqual(t, *cpyPtr, *orig)

	mutateNestedStructures(map[string]any(*cpyPtr))
	if reflect.DeepEqual(*cpyPtr, *orig) {
		t.Fatalf("expected deep copy to diverge after mutation, but values are still equal")
	}

	want := Output{
		"ok":     true,
		"nested": map[string]any{"k": "v"},
		"nums":   []int{4, 5, 6},
	}
	mustDeepEqual(t, *orig, want)
}

func TestDeepCopy_OutputPtr_SrcNilPointer(t *testing.T) {
	var orig *Output
	cpy, err := DeepCopy[*Output](orig)
	if err != nil {
		t.Fatalf("DeepCopy(nil *Output) error: %v", err)
	}
	if cpy != nil {
		t.Fatalf("expected nil for nil *Output, got: %#v", cpy)
	}
}

func TestDeepCopy_OutputPtr_PointedNilMap(t *testing.T) {
	tmp := Output(nil)
	orig := &tmp
	cpy, err := DeepCopy[*Output](orig)
	if err != nil {
		t.Fatalf("DeepCopy(*Output with nil map) error: %v", err)
	}
	if cpy != nil {
		t.Fatalf("expected nil for *Output pointing to nil map, got: %#v", cpy)
	}
}

func TestDeepCopy_Generic_Primitives(t *testing.T) {
	i := 42
	iCopy, err := DeepCopy[int](i)
	if err != nil {
		t.Fatalf("DeepCopy(int) error: %v", err)
	}
	if iCopy != i {
		t.Fatalf("DeepCopy(int) mismatch: got %d want %d", iCopy, i)
	}

	s := "hello"
	sCopy, err := DeepCopy[string](s)
	if err != nil {
		t.Fatalf("DeepCopy(string) error: %v", err)
	}
	if sCopy != s {
		t.Fatalf("DeepCopy(string) mismatch: got %q want %q", sCopy, s)
	}
}

type genericStruct struct {
	N   int
	S   string
	Arr []int
	Nst *nestedStruct
}
type nestedStruct struct {
	K string
	V map[string]int
}

func TestDeepCopy_Generic_StructDeepSemantics(t *testing.T) {
	orig := genericStruct{
		N:   7,
		S:   "abc",
		Arr: []int{1, 2, 3},
		Nst: &nestedStruct{
			K: "k",
			V: map[string]int{"x": 1},
		},
	}

	cpy, err := DeepCopy[genericStruct](orig)
	if err != nil {
		t.Fatalf("DeepCopy(genericStruct) error: %v", err)
	}

	// Equal content initially
	if !reflect.DeepEqual(cpy, orig) {
		t.Fatalf("DeepCopy struct mismatch.\n got: %#v\nwant: %#v", cpy, orig)
	}

	// Mutate the copy deeply
	cpy.N = 8
	cpy.Arr[0] = 999
	cpy.Nst.K = "k2"
	cpy.Nst.V["x"] = 77

	// Ensure original did not change (deep copy)
	if reflect.DeepEqual(cpy, orig) {
		t.Fatalf("expected deep copy to diverge after mutation")
	}
	want := genericStruct{
		N:   7,
		S:   "abc",
		Arr: []int{1, 2, 3},
		Nst: &nestedStruct{
			K: "k",
			V: map[string]int{"x": 1},
		},
	}
	if !reflect.DeepEqual(orig, want) {
		t.Fatalf("original mutated unexpectedly.\n got: %#v\nwant: %#v", orig, want)
	}
}

func TestDeepCopy_Generic_MapAny(t *testing.T) {
	orig := map[string]any{
		"a": 1,
		"b": []string{"a", "b"},
		"c": map[string]any{"z": 1},
	}
	cpy, err := DeepCopy[map[string]any](orig)
	if err != nil {
		t.Fatalf("DeepCopy(map[string]any) error: %v", err)
	}
	mustDeepEqual(t, cpy, orig)

	// Mutate copy
	cpy["a"] = 2
	cpy["b"].([]string)[0] = "changed"
	cpy["c"].(map[string]any)["z"] = 9

	// Ensure original unchanged
	want := map[string]any{
		"a": 1,
		"b": []string{"a", "b"},
		"c": map[string]any{"z": 1},
	}
	mustDeepEqual(t, orig, want)
}

// While deepCopyMap is an unexported helper, since tests reside in the same package,
// we can validate its behavior indirectly through DeepCopy(Input/Output) tests above.
// For completeness, we add a direct test to ensure the error path is not triggered
// under normal usage with a map[string]any input.
func Test_deepCopyMap_SucceedsOnMapAny(t *testing.T) {
	orig := map[string]any{"k": []int{1, 2}, "m": map[string]any{"x": 1}}
	cpy, err := deepCopyMap(orig)
	if err != nil {
		t.Fatalf("deepCopyMap returned error: %v", err)
	}
	mustDeepEqual(t, cpy, orig)
	// mutate copy to ensure independence
	// Mutate the slice in the copy
	if slice, ok := cpy["k"].([]int); ok && len(slice) > 0 {
		slice[0] = 999
	}
	// Mutate the nested map in the copy
	if nestedMap, ok := cpy["m"].(map[string]any); ok {
		nestedMap["x"] = 999
	}
	if reflect.DeepEqual(cpy, orig) {
		t.Fatalf("expected deep copy to diverge after mutation")
	}
}
