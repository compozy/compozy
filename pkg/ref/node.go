package ref

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Node - Flexible reference node that handles both string and object forms
// -----------------------------------------------------------------------------

// Node represents a flexible reference that can be marshaled/unmarshaled as either
// a string (shorthand form) or an object (verbose form).
type Node struct {
	ref *Ref
}

// NewNode creates a new Node from a Ref.
func NewNode(ref *Ref) *Node {
	return &Node{ref: ref}
}

// NewNodeFromString creates a new Node from a string reference.
func NewNodeFromString(refStr string) (*Node, error) {
	ref, err := parseStringRef(refStr)
	if err != nil {
		return nil, err
	}
	return &Node{ref: ref}, nil
}

// InnerRef returns the underlying InnerRef.
func (n *Node) InnerRef() *Ref {
	if n == nil {
		return nil
	}
	return n.ref
}

// IsEmpty returns true if the node contains no reference.
func (n *Node) IsEmpty() bool {
	return n == nil || n.ref == nil
}

// String returns the string representation of the reference.
func (n *Node) String() string {
	if n.IsEmpty() {
		return ""
	}
	return n.ref.String()
}

// Resolve resolves the reference using the provided context and metadata.
func (n *Node) Resolve(ctx context.Context, currentDoc any, currentFilePath, projectRoot string) (any, error) {
	if n.IsEmpty() {
		return nil, errors.New("cannot resolve empty reference node")
	}
	return n.ref.Resolve(ctx, currentDoc, currentFilePath, projectRoot)
}

// ApplyMergeMode applies the merge mode to combine reference and inline values.
func (n *Node) ApplyMergeMode(refValue, inlineValue any) (any, error) {
	if n.IsEmpty() {
		return inlineValue, nil
	}
	return n.ref.ApplyMergeMode(refValue, inlineValue)
}

// -----------------------------------------------------------------------------
// JSON Marshaling/Unmarshaling
// -----------------------------------------------------------------------------

// MarshalJSON implements json.Marshaler.
// It serializes as a string if it's a simple reference, otherwise as an object.
func (n Node) MarshalJSON() ([]byte, error) {
	if n.ref == nil {
		return json.Marshal(nil)
	}
	// Always try to marshal as string first since the spec supports string form for all cases
	return json.Marshal(n.ref.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *Node) UnmarshalJSON(data []byte) error {
	// Handle null values
	if string(data) == "null" {
		n.ref = nil
		return nil
	}
	// Try to unmarshal as string first
	var refStr string
	if err := json.Unmarshal(data, &refStr); err == nil {
		ref, err := parseStringRef(refStr)
		if err != nil {
			return errors.Wrap(err, "failed to parse string reference")
		}
		n.ref = ref
		return nil
	}
	// Try to unmarshal as object
	var refObj Ref
	if err := json.Unmarshal(data, &refObj); err != nil {
		return errors.Wrap(err, "failed to parse reference object")
	}
	n.ref = &refObj
	return nil
}

// -----------------------------------------------------------------------------
// YAML Marshaling/Unmarshaling
// -----------------------------------------------------------------------------

// MarshalYAML implements yaml.Marshaler.
func (n Node) MarshalYAML() (any, error) {
	if n.ref == nil {
		return nil, nil
	}
	// Always try to marshal as string first since the spec supports string form for all cases
	return n.ref.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (n *Node) UnmarshalYAML(node *yaml.Node) error {
	ref, err := ParseRef(node)
	if err != nil {
		return err
	}
	n.ref = ref
	return nil
}
