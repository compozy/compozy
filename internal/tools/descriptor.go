package tools

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/toolmeta"
)

// Tool is the cold desired-state resource spec for a registry tool.
type Tool struct {
	ID           ToolID     `json:"id"`
	Backend      BackendRef `json:"backend"`
	DisplayTitle string     `json:"display_title,omitempty"`
	*ToolPresentation
	Description         string          `json:"description"`
	InputSchema         json.RawMessage `json:"input_schema"`
	OutputSchema        json.RawMessage `json:"output_schema,omitempty"`
	InputSchemaDigest   string          `json:"input_schema_digest"`
	OutputSchemaDigest  string          `json:"output_schema_digest,omitempty"`
	Source              SourceRef       `json:"source"`
	Visibility          Visibility      `json:"visibility"`
	Risk                RiskClass       `json:"risk"`
	ReadOnly            bool            `json:"read_only"`
	Destructive         bool            `json:"destructive"`
	OpenWorld           bool            `json:"open_world"`
	RequiresInteraction bool            `json:"requires_interaction"`
	ConcurrencySafe     bool            `json:"concurrency_safe"`
	MaxResultBytes      int64           `json:"max_result_bytes,omitempty"`
	Toolsets            []ToolsetID     `json:"toolsets,omitempty"`
	Tags                []string        `json:"tags,omitempty"`
	SearchHints         []string        `json:"search_hints,omitempty"`
}

// Descriptor is the normalized runtime metadata used for indexing and dispatch.
type Descriptor struct {
	ID           ToolID     `json:"id"`
	Backend      BackendRef `json:"backend"`
	DisplayTitle string     `json:"display_title,omitempty"`
	*ToolPresentation
	Description         string          `json:"description"`
	InputSchema         json.RawMessage `json:"input_schema"`
	OutputSchema        json.RawMessage `json:"output_schema,omitempty"`
	InputSchemaDigest   string          `json:"input_schema_digest"`
	OutputSchemaDigest  string          `json:"output_schema_digest,omitempty"`
	Source              SourceRef       `json:"source"`
	Visibility          Visibility      `json:"visibility"`
	Risk                RiskClass       `json:"risk"`
	ReadOnly            bool            `json:"read_only"`
	Destructive         bool            `json:"destructive"`
	OpenWorld           bool            `json:"open_world"`
	RequiresInteraction bool            `json:"requires_interaction"`
	ConcurrencySafe     bool            `json:"concurrency_safe"`
	MaxResultBytes      int64           `json:"max_result_bytes,omitempty"`
	Toolsets            []ToolsetID     `json:"toolsets,omitempty"`
	Tags                []string        `json:"tags,omitempty"`
	SearchHints         []string        `json:"search_hints,omitempty"`
}

// Descriptor converts a cold resource into the runtime descriptor shape.
func (t Tool) Descriptor() Descriptor {
	return Descriptor{
		ID:                  t.ID,
		Backend:             t.Backend,
		DisplayTitle:        t.DisplayTitle,
		ToolPresentation:    CloneToolPresentation(t.ToolPresentation),
		Description:         t.Description,
		InputSchema:         cloneRawMessage(t.InputSchema),
		OutputSchema:        cloneRawMessage(t.OutputSchema),
		InputSchemaDigest:   t.InputSchemaDigest,
		OutputSchemaDigest:  t.OutputSchemaDigest,
		Source:              t.Source,
		Visibility:          t.Visibility,
		Risk:                t.Risk,
		ReadOnly:            t.ReadOnly,
		Destructive:         t.Destructive,
		OpenWorld:           t.OpenWorld,
		RequiresInteraction: t.RequiresInteraction,
		ConcurrencySafe:     t.ConcurrencySafe,
		MaxResultBytes:      t.MaxResultBytes,
		Toolsets:            cloneToolsets(t.Toolsets),
		Tags:                cloneStrings(t.Tags),
		SearchHints:         cloneStrings(t.SearchHints),
	}
}

// Tool returns the cold resource shape for a runtime descriptor.
func (d Descriptor) Tool() Tool {
	return Tool{
		ID:                  d.ID,
		Backend:             d.Backend,
		DisplayTitle:        d.DisplayTitle,
		ToolPresentation:    CloneToolPresentation(d.ToolPresentation),
		Description:         d.Description,
		InputSchema:         cloneRawMessage(d.InputSchema),
		OutputSchema:        cloneRawMessage(d.OutputSchema),
		InputSchemaDigest:   d.InputSchemaDigest,
		OutputSchemaDigest:  d.OutputSchemaDigest,
		Source:              d.Source,
		Visibility:          d.Visibility,
		Risk:                d.Risk,
		ReadOnly:            d.ReadOnly,
		Destructive:         d.Destructive,
		OpenWorld:           d.OpenWorld,
		RequiresInteraction: d.RequiresInteraction,
		ConcurrencySafe:     d.ConcurrencySafe,
		MaxResultBytes:      d.MaxResultBytes,
		Toolsets:            cloneToolsets(d.Toolsets),
		Tags:                cloneStrings(d.Tags),
		SearchHints:         cloneStrings(d.SearchHints),
	}
}

// Validate ensures the descriptor is dispatchable metadata.
func (d Descriptor) Validate() error {
	if err := d.ID.Validate(); err != nil {
		return err
	}
	if err := d.Backend.Validate("backend"); err != nil {
		return err
	}
	if err := d.Source.Validate("source"); err != nil {
		return err
	}
	if err := d.Visibility.Validate("visibility"); err != nil {
		return err
	}
	if err := d.Risk.Validate("risk"); err != nil {
		return err
	}
	presentation := d.Presentation()
	if err := toolmeta.ValidateDescriptorMetadata(presentation.FriendlyVerb, presentation.Preview); err != nil {
		return NewValidationError("presentation", ReasonSchemaInvalid, err.Error())
	}
	if err := ValidateJSONObject("input_schema", d.InputSchema, true); err != nil {
		return err
	}
	if err := ValidateJSONObject("output_schema", d.OutputSchema, false); err != nil {
		return err
	}
	if d.MaxResultBytes < 0 {
		return NewValidationError(
			"max_result_bytes",
			ReasonResultBudgetExceeded,
			"must be greater than or equal to zero",
		)
	}
	if d.ReadOnly && (d.Destructive || d.OpenWorld) {
		return NewValidationError(
			"read_only",
			ReasonPolicyDenied,
			"read-only tools cannot be destructive or open-world",
		)
	}
	for i, toolset := range d.Toolsets {
		if err := toolset.Validate(); err != nil {
			return wrapField(err, fmt.Sprintf("toolsets[%d]", i))
		}
	}
	return nil
}

// Validate ensures the cold resource can normalize into a descriptor.
func (t Tool) Validate() error {
	return t.Descriptor().Validate()
}
