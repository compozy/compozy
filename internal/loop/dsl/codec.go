package dsl

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Parse decodes one agh.loop/v1 YAML document.
func Parse(data []byte) (Definition, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Definition{}, fmt.Errorf("parse loop definition: document is empty")
	}
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return Definition{}, fmt.Errorf("parse loop definition: %w", err)
	}
	def.Normalize()
	if err := def.ValidateHeader(); err != nil {
		return Definition{}, fmt.Errorf("parse loop definition: %w", err)
	}
	return def, nil
}

// Serialize encodes one agh.loop/v1 YAML document.
func Serialize(def Definition) ([]byte, error) {
	def.Normalize()
	if err := def.ValidateHeader(); err != nil {
		return nil, fmt.Errorf("serialize loop definition: %w", err)
	}
	data, err := yaml.Marshal(def)
	if err != nil {
		return nil, fmt.Errorf("serialize loop definition: %w", err)
	}
	return data, nil
}

// GraphDocument is the editor-facing graph plus the original definition.
type GraphDocument struct {
	Original Definition
	Nodes    []Node
	Edges    []Edge
}

// DefinitionToGraph extracts the graph while retaining the original definition for structural merge.
func DefinitionToGraph(def Definition) *GraphDocument {
	def.Normalize()
	return &GraphDocument{
		Original: def,
		Nodes:    append([]Node(nil), def.Graph.Nodes...),
		Edges:    append([]Edge(nil), def.Graph.Edges...),
	}
}

// GraphToDefinition merges graph edits back into the original definition.
func GraphToDefinition(graph *GraphDocument) Definition {
	def := graph.Original
	def.Graph.Nodes = append([]Node(nil), graph.Nodes...)
	def.Graph.Edges = append([]Edge(nil), graph.Edges...)
	def.Normalize()
	return def
}
