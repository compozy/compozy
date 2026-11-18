package daemon

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

type compactSuggestionRecord struct {
	ID          string                         `json:"id"`
	ProfileID   string                         `json:"profile_id"`
	WorkspaceID string                         `json:"workspace_id"`
	Source      automationpkg.SuggestionSource `json:"source"`
	DedupKey    string                         `json:"dedup_key"`
	Status      automationpkg.SuggestionStatus `json:"status"`
	Payload     compactJobRecord               `json:"payload"`
	CreatedAt   time.Time                      `json:"created_at"`
	ResolvedAt  *time.Time                     `json:"resolved_at,omitempty"`
}

func compactSuggestion(p contract.AutomationSuggestionPayload) compactSuggestionRecord {
	return compactSuggestionRecord{
		ID:          p.ID,
		ProfileID:   p.ProfileID,
		WorkspaceID: p.WorkspaceID,
		Source:      p.Source,
		DedupKey:    p.DedupKey,
		Status:      p.Status,
		Payload:     compactJob(p.Payload),
		CreatedAt:   p.CreatedAt,
		ResolvedAt:  p.ResolvedAt,
	}
}

var automationSuggestionListViews = resultViews[contract.AutomationSuggestionsResponse]{
	Compact: func(p contract.AutomationSuggestionsResponse) any {
		var rows []compactSuggestionRecord
		if p.Suggestions != nil {
			rows = make([]compactSuggestionRecord, len(p.Suggestions))
			for i, value := range p.Suggestions {
				rows[i] = compactSuggestion(value)
			}
		}
		return map[string]any{"suggestions": rows}
	},
	Preview: func(p contract.AutomationSuggestionsResponse) string {
		return fmt.Sprintf("%d automation suggestions", len(p.Suggestions))
	},
}

var automationSuggestionAcceptViews = resultViews[contract.AutomationSuggestionAcceptanceResponse]{
	Compact: func(p contract.AutomationSuggestionAcceptanceResponse) any {
		return map[string]any{
			"suggestion":                compactSuggestion(p.Suggestion),
			nativeAutomationToolsJobKey: compactJob(p.Job),
		}
	},
	Preview: func(p contract.AutomationSuggestionAcceptanceResponse) string { return p.Job.ID },
}

var automationSuggestionViews = resultViews[contract.AutomationSuggestionResponse]{
	Compact: func(p contract.AutomationSuggestionResponse) any {
		return map[string]any{"suggestion": compactSuggestion(p.Suggestion)}
	},
	Preview: func(p contract.AutomationSuggestionResponse) string { return p.Suggestion.ID },
}
