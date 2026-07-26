// Package dsl defines the agh.loop/v1 authoring document.
package dsl

import (
	"fmt"

	"github.com/compozy/agh/internal/network/participation"
)

const (
	// APIVersion is the only loop DSL version accepted by this package.
	APIVersion = "agh.loop/v1"
	// KindLoop is the document kind for loop definitions.
	KindLoop = "Loop"
	// GateMaxRevisionsCeiling is the hard structural gate revision ceiling.
	GateMaxRevisionsCeiling = 64
)

// NodeID is the stable graph node identity.
type NodeID string

// Definition is the canonical agh.loop/v1 document.
type Definition struct {
	APIVersion                string            `json:"apiVersion"            yaml:"apiVersion"`
	Kind                      string            `json:"kind"                  yaml:"kind"`
	Meta                      Meta              `json:"meta"                  yaml:"meta"`
	Concurrency               ConcurrencyPolicy `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	Inputs                    map[string]Input  `json:"inputs,omitempty"      yaml:"inputs,omitempty"`
	Contract                  Contract          `json:"contract"              yaml:"contract"`
	Graph                     Graph             `json:"graph"                 yaml:"graph"`
	*DefinitionExtensionState `                                               yaml:",inline"`
	Extra                     map[string]any `json:"-"                     yaml:",inline"`
}

// DefinitionExtensionState keeps optional authoring extensions off the hot Definition value.
type DefinitionExtensionState struct {
	Start                []StartBinding         `json:"start,omitempty"                 yaml:"start,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty" yaml:"network_participation,omitempty"`
}

// Normalize applies document-level zero-value semantics without inventing authoring defaults.
func (d *Definition) Normalize() {
	if d.DefinitionExtensionState == nil {
		d.DefinitionExtensionState = &DefinitionExtensionState{}
	}
	if d.APIVersion == "" {
		d.APIVersion = APIVersion
	}
	if d.Kind == "" {
		d.Kind = KindLoop
	}
	if d.Concurrency == "" {
		d.Concurrency = ConcurrencyForbid
	}
	if d.Inputs == nil {
		d.Inputs = map[string]Input{}
	}
	if d.Start == nil {
		d.Start = []StartBinding{}
	}
	d.Contract.Normalize()
	d.Graph.Normalize()
}

// ValidateHeader checks the fixed document identity.
func (d Definition) ValidateHeader() error {
	if d.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion must be %q", APIVersion)
	}
	if d.Kind != KindLoop {
		return fmt.Errorf("kind must be %q", KindLoop)
	}
	return nil
}

// Meta describes a loop definition for catalog and authoring surfaces.
type Meta struct {
	Name        string         `json:"name"                  yaml:"name"`
	Version     int            `json:"version,omitempty"     yaml:"version,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Catalog     CatalogMeta    `json:"catalog"               yaml:"catalog"`
	Extra       map[string]any `json:"-"                     yaml:",inline"`
}

// CatalogMeta is the matchable catalog projection embedded in a definition.
type CatalogMeta struct {
	UseWhen  string         `json:"use_when,omitempty" yaml:"use_when,omitempty"`
	Keywords []string       `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Category string         `json:"category,omitempty" yaml:"category,omitempty"`
	Extra    map[string]any `json:"-"                  yaml:",inline"`
}

// ConcurrencyPolicy controls same-loop concurrent starts.
type ConcurrencyPolicy string

const (
	// ConcurrencyForbid rejects concurrent starts of the same loop.
	ConcurrencyForbid ConcurrencyPolicy = "forbid"
	// ConcurrencyAllow permits concurrent starts of the same loop.
	ConcurrencyAllow ConcurrencyPolicy = "allow"
	// ConcurrencyQueue creates a queued loop run for later promotion.
	ConcurrencyQueue ConcurrencyPolicy = "queue"
)

// InputType is the closed declared input type vocabulary.
type InputType string

const (
	// InputTypeString declares a string input.
	InputTypeString InputType = "string"
	// InputTypeNumber declares a numeric input.
	InputTypeNumber InputType = "number"
	// InputTypeBoolean declares a boolean input.
	InputTypeBoolean InputType = "boolean"
	// InputTypeFile declares a file input.
	InputTypeFile InputType = "file"
	// InputTypeAgent declares an agent reference input.
	InputTypeAgent InputType = "agent"
	// InputTypeRef declares a typed resource reference input.
	InputTypeRef InputType = "ref"
)

// Input declares one named loop input.
type Input struct {
	Type        InputType      `json:"type"                  yaml:"type"`
	Required    bool           `json:"required,omitempty"    yaml:"required,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Ref         *InputRef      `json:"ref,omitempty"         yaml:"ref,omitempty"`
	Default     any            `json:"default,omitempty"     yaml:"default,omitempty"`
	Extra       map[string]any `json:"-"                     yaml:",inline"`
}

// InputRef narrows an input of type ref to one resource kind.
type InputRef struct {
	Kind  string         `json:"kind" yaml:"kind"`
	Extra map[string]any `json:"-"    yaml:",inline"`
}
