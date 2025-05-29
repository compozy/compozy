package ref

import (
	"encoding/json"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Node
// -----------------------------------------------------------------------------

// Node represents a reference node that can be unmarshaled from either a string or object form.
type Node struct {
	ref *Ref
}

// NewNode creates a new Node from a Ref.
func NewNode(ref *Ref) *Node {
	return &Node{ref: ref}
}

// GetRef returns the underlying Ref.
func (rn *Node) GetRef() *Ref {
	return rn.ref
}

// IsEmpty returns true if the Node has no reference.
func (rn *Node) IsEmpty() bool {
	return rn.ref == nil
}

// UnmarshalYAML implements yaml.Unmarshaler to handle both string and object forms.
func (rn *Node) UnmarshalYAML(node *yaml.Node) error {
	ref, err := ParseRef(node)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal YAML")
	}
	rn.ref = ref
	return nil
}

// MarshalYAML implements yaml.Marshaler to output the appropriate form.
func (rn *Node) MarshalYAML() (any, error) {
	if rn.ref == nil {
		return nil, nil
	}
	if rn.shouldUseStringForm() {
		return rn.ref.String(), nil
	}
	return rn.ref, nil
}

// shouldUseStringForm determines if we should use string form for marshaling.
func (rn *Node) shouldUseStringForm() bool {
	return rn.ref != nil && (rn.ref.Mode == ModeMerge || rn.ref.Mode == "")
}

// UnmarshalJSON implements json.Unmarshaler for JSON support.
func (rn *Node) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		ref, err := parseRefValue(str)
		if err != nil {
			return errors.Wrap(err, "failed to parse JSON string ref")
		}
		rn.ref = ref
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return errors.New("$ref must be a string or object")
	}
	ref, err := parseRefValue(obj)
	if err != nil {
		return errors.Wrap(err, "failed to parse JSON object ref")
	}
	rn.ref = ref
	return nil
}

// MarshalJSON implements json.Marshaler for JSON support.
func (rn *Node) MarshalJSON() ([]byte, error) {
	if rn.ref == nil {
		return json.Marshal(nil)
	}
	if rn.shouldUseStringForm() {
		return json.Marshal(rn.ref.String())
	}
	return json.Marshal(rn.ref)
}

// ApplyMergeMode applies the merge mode from the Node.
func (rn *Node) ApplyMergeMode(refValue, inlineValue any) (any, error) {
	if rn.ref == nil {
		return inlineValue, nil
	}
	return rn.ref.ApplyMergeMode(refValue, inlineValue)
}
