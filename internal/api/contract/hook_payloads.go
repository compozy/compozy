package contract

import (
	"encoding/json"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

// HookCatalogQuery captures the shared resolved-hook catalog filters.
type HookCatalogQuery struct {
	Workspace string
	Agent     string
	Event     string
	Source    string
	Mode      string
}

// HookRunsQuery captures the shared hook execution history filters.
type HookRunsQuery struct {
	Session string
	Event   string
	Outcome string
	Since   string
	Last    int
}

// HookEventsQuery captures the shared hook taxonomy filters.
type HookEventsQuery struct {
	Family   string
	SyncOnly bool
}

// HookCatalogPayload is the shared resolved-hook catalog response payload.
type HookCatalogPayload struct {
	Order        int                  `json:"order"`
	Name         string               `json:"name"`
	Event        string               `json:"event"`
	Source       string               `json:"source"`
	SkillSource  string               `json:"skill_source,omitempty"`
	Mode         string               `json:"mode"`
	Required     bool                 `json:"required"`
	Priority     int                  `json:"priority"`
	TimeoutMS    int64                `json:"timeout_ms,omitempty"`
	ExecutorKind string               `json:"executor_kind,omitempty"`
	Matcher      hookspkg.HookMatcher `json:"matcher"`
	Metadata     map[string]string    `json:"metadata,omitempty"`
}

// HookRunPayload is the shared hook execution history response payload.
type HookRunPayload struct {
	HookName      string          `json:"hook_name"`
	Event         string          `json:"event"`
	Source        string          `json:"source"`
	Mode          string          `json:"mode"`
	DurationMS    int64           `json:"duration_ms"`
	Outcome       string          `json:"outcome"`
	DispatchDepth int             `json:"dispatch_depth"`
	PatchApplied  json.RawMessage `json:"patch_applied,omitempty"`
	Error         string          `json:"error,omitempty"`
	Required      bool            `json:"required,omitempty"`
	RecordedAt    time.Time       `json:"recorded_at"`
}

// HookEventPayload is the shared hook taxonomy response payload.
type HookEventPayload struct {
	Event         string `json:"event"`
	Family        string `json:"family"`
	SyncEligible  bool   `json:"sync_eligible"`
	PayloadSchema string `json:"payload_schema"`
	PatchSchema   string `json:"patch_schema,omitempty"`
}
