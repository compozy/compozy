package daemon

import (
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

type compactToolListView struct {
	Descriptor   compactToolListDescriptor `json:"descriptor"`
	Availability toolspkg.Availability     `json:"availability"`
	Decision     compactToolDecision       `json:"decision"`
}
type compactToolListPage struct {
	Tools      []compactToolListView `json:"tools"`
	Offset     int                   `json:"offset"`
	Count      int                   `json:"count"`
	Total      int                   `json:"total"`
	NextOffset int                   `json:"next_offset,omitzero"`
	HasMore    bool                  `json:"has_more"`
}

var toolListViews = resultViews[nativeToolListPage]{
	Compact: func(p nativeToolListPage) any {
		var rows []compactToolListView
		if p.Tools != nil {
			rows = make([]compactToolListView, len(p.Tools))
			for i, v := range p.Tools {
				rows[i] = compactToolListView{
					projectCompactToolListDescriptor(v.Descriptor),
					v.Availability,
					projectCompactToolDecision(v.Decision),
				}
			}
		}
		return compactToolListPage{rows, p.Offset, p.Count, p.Total, p.NextOffset, p.HasMore}
	},
	Preview: func(p nativeToolListPage) string { return fmt.Sprintf("%d of %d tools", p.Count, p.Total) },
}

type compactToolSearchView struct {
	Descriptor   compactToolSearchDescriptor `json:"descriptor"`
	Availability toolspkg.Availability       `json:"availability"`
	Decision     compactToolDecision         `json:"decision"`
}

var toolSearchViews = resultViews[[]toolspkg.ToolView]{
	Compact: func(p []toolspkg.ToolView) any {
		var rows []compactToolSearchView
		if p != nil {
			rows = make([]compactToolSearchView, len(p))
			for i, v := range p {
				rows[i] = compactToolSearchView{
					projectCompactToolSearchDescriptor(v.Descriptor),
					v.Availability,
					projectCompactToolDecision(v.Decision),
				}
			}
		}
		return map[string]any{"tools": rows}
	},
	Full:    func(p []toolspkg.ToolView) any { return map[string]any{"tools": p} },
	Preview: func(p []toolspkg.ToolView) string { return fmt.Sprintf("%d tools", len(p)) },
}
var approvalSetViews = resultViews[nativeToolApprovalSetResponse]{
	Compact: func(p nativeToolApprovalSetResponse) any {
		return map[string]any{"grant": projectCompactApprovalGrant(p.Grant)}
	},
}
var approvalListViews = resultViews[nativeToolApprovalListResponse]{
	Compact: func(p nativeToolApprovalListResponse) any {
		var rows []compactApprovalGrant
		if p.Grants != nil {
			rows = make([]compactApprovalGrant, len(p.Grants))
			for i, v := range p.Grants {
				rows[i] = projectCompactApprovalGrant(v)
			}
		}
		return map[string]any{"grants": rows, "total": p.Total}
	},
	Preview: func(p nativeToolApprovalListResponse) string {
		return fmt.Sprintf("%d remembered tool approvals", p.Total)
	},
}
