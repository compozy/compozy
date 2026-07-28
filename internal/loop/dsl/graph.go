package dsl

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Graph is the acyclic loop body.
type Graph struct {
	Nodes []Node         `json:"nodes" yaml:"nodes"`
	Edges []Edge         `json:"edges" yaml:"edges"`
	Extra map[string]any `json:"-"     yaml:",inline"`
}

// Normalize initializes graph slices for stable callers.
func (g *Graph) Normalize() {
	if g.Nodes == nil {
		g.Nodes = []Node{}
	}
	if g.Edges == nil {
		g.Edges = []Edge{}
	}
}

// Node is the single envelope for every graph node.
type Node struct {
	ID            NodeID              `json:"id"                       yaml:"id"`
	Class         NodeClass           `json:"class"                    yaml:"class"`
	Kind          string              `json:"kind"                     yaml:"kind"`
	Session       *SessionSpec        `json:"session,omitempty"        yaml:"session,omitempty"`
	Timeout       string              `json:"timeout,omitempty"        yaml:"timeout,omitempty"`
	Retry         *RetrySpec          `json:"retry,omitempty"          yaml:"retry,omitempty"`
	Harvest       *HarvestSpec        `json:"harvest,omitempty"        yaml:"harvest,omitempty"`
	Produces      Schema              `json:"produces,omitempty"       yaml:"produces,omitempty"`
	Params        NodeParams          `json:"params,omitempty"         yaml:"params,omitempty"`
	Collection    string              `json:"collection,omitempty"     yaml:"collection,omitempty"`
	Filter        string              `json:"filter,omitempty"         yaml:"filter,omitempty"`
	BatchSize     int                 `json:"batch_size,omitempty"     yaml:"batch_size,omitempty"`
	MaxParallel   int                 `json:"max_parallel,omitempty"   yaml:"max_parallel,omitempty"`
	MaxFanOut     int                 `json:"max_fan_out,omitempty"    yaml:"max_fan_out,omitempty"`
	Condition     string              `json:"condition,omitempty"      yaml:"condition,omitempty"`
	Criteria      []GateCriterion     `json:"criteria,omitempty"       yaml:"criteria,omitempty"`
	VerdictPolicy VerdictPolicy       `json:"verdict_policy,omitempty" yaml:"verdict_policy,omitempty"`
	OnResult      map[string]any      `json:"on_result,omitempty"      yaml:"on_result,omitempty"`
	MaxRevisions  int                 `json:"max_revisions,omitempty"  yaml:"max_revisions,omitempty"`
	Body          *Graph              `json:"body,omitempty"           yaml:"body,omitempty"`
	Contract      *Contract           `json:"contract,omitempty"       yaml:"contract,omitempty"`
	InputRef      string              `json:"input_ref,omitempty"      yaml:"input_ref,omitempty"`
	Pattern       string              `json:"pattern,omitempty"        yaml:"pattern,omitempty"`
	Parse         FileParseKind       `json:"parse,omitempty"          yaml:"parse,omitempty"`
	WatchSpec     map[string]any      `json:"watch,omitempty"          yaml:"watch,omitempty"`
	Events        []EventSubscription `json:"events,omitempty"         yaml:"events,omitempty"`
	Extra         map[string]any      `json:"-"                        yaml:",inline"`
}

// Schema is a JSON-schema-compatible object or the TechSpec shorthand map.
type Schema map[string]any

// NodeParams holds raw action params and can decode into per-kind schema types.
type NodeParams map[string]any

// Decode unmarshals params into a concrete per-kind schema type.
func (p NodeParams) Decode(dest any) error {
	data, err := yaml.Marshal(map[string]any(p))
	if err != nil {
		return fmt.Errorf("marshal node params: %w", err)
	}
	if err := yaml.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("decode node params: %w", err)
	}
	return nil
}

// Edge declares one blocks dependency.
type Edge struct {
	From  NodeID         `json:"from" yaml:"from"`
	To    NodeID         `json:"to"   yaml:"to"`
	Extra map[string]any `json:"-"    yaml:",inline"`
}
