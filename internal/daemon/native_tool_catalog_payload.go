package daemon

import (
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	defaultNativeToolListLimit = 100
	maxNativeToolListLimit     = 100
)

type nativeToolListDescriptor struct {
	ID                  toolspkg.ToolID      `json:"id"`
	InputSchemaDigest   string               `json:"input_schema_digest"`
	OutputSchemaDigest  string               `json:"output_schema_digest,omitempty"`
	Risk                toolspkg.RiskClass   `json:"risk"`
	ReadOnly            bool                 `json:"read_only"`
	Destructive         bool                 `json:"destructive"`
	OpenWorld           bool                 `json:"open_world"`
	RequiresInteraction bool                 `json:"requires_interaction"`
	ConcurrencySafe     bool                 `json:"concurrency_safe"`
	MaxResultBytes      int64                `json:"max_result_bytes,omitempty"`
	Toolsets            []toolspkg.ToolsetID `json:"toolsets,omitempty"`
}

type nativeToolListView struct {
	Descriptor   nativeToolListDescriptor       `json:"descriptor"`
	Availability toolspkg.Availability          `json:"availability"`
	Decision     toolspkg.EffectiveToolDecision `json:"decision"`
}

type nativeToolListPage struct {
	Tools      []nativeToolListView `json:"tools"`
	Offset     int                  `json:"offset"`
	Count      int                  `json:"count"`
	Total      int                  `json:"total"`
	NextOffset int                  `json:"next_offset,omitempty"`
	HasMore    bool                 `json:"has_more"`
}

func normalizeNativeToolListInput(
	id toolspkg.ToolID,
	input toolListInput,
) (int, int, error) {
	if input.Offset < 0 {
		return 0, 0, nativeToolListValidationError(
			id,
			"offset",
			"offset must be greater than or equal to zero",
		)
	}
	limit := input.Limit
	if limit == 0 {
		limit = defaultNativeToolListLimit
	}
	if limit < 1 || limit > maxNativeToolListLimit {
		return 0, 0, nativeToolListValidationError(
			id,
			"limit",
			fmt.Sprintf("limit must be between 1 and %d", maxNativeToolListLimit),
		)
	}
	return input.Offset, limit, nil
}

func nativeToolListValidationError(id toolspkg.ToolID, field string, detail string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		detail,
		fmt.Errorf(
			"%w: %w",
			toolspkg.ErrToolInvalidInput,
			toolspkg.NewValidationError(field, toolspkg.ReasonSchemaInvalid, detail),
		),
		toolspkg.ReasonSchemaInvalid,
	)
}

func nativeToolListPayloads(views []toolspkg.ToolView) []nativeToolListView {
	payloads := make([]nativeToolListView, 0, len(views))
	for index := range views {
		view := &views[index]
		payloads = append(payloads, nativeToolListView{
			Descriptor: nativeToolListDescriptor{
				ID:                  view.Descriptor.ID,
				InputSchemaDigest:   view.Descriptor.InputSchemaDigest,
				OutputSchemaDigest:  view.Descriptor.OutputSchemaDigest,
				Risk:                view.Descriptor.Risk,
				ReadOnly:            view.Descriptor.ReadOnly,
				Destructive:         view.Descriptor.Destructive,
				OpenWorld:           view.Descriptor.OpenWorld,
				RequiresInteraction: view.Descriptor.RequiresInteraction,
				ConcurrencySafe:     view.Descriptor.ConcurrencySafe,
				MaxResultBytes:      view.Descriptor.MaxResultBytes,
				Toolsets:            append([]toolspkg.ToolsetID(nil), view.Descriptor.Toolsets...),
			},
			Availability: view.Availability,
			Decision:     view.Decision,
		})
	}
	return payloads
}

func nativeToolListPageFromViews(views []toolspkg.ToolView, offset int, limit int) nativeToolListPage {
	start := min(offset, len(views))
	end := min(start+limit, len(views))
	hasMore := end < len(views)
	nextOffset := 0
	if hasMore {
		nextOffset = end
	}
	return nativeToolListPage{
		Tools:      nativeToolListPayloads(views[start:end]),
		Offset:     offset,
		Count:      end - start,
		Total:      len(views),
		NextOffset: nextOffset,
		HasMore:    hasMore,
	}
}
