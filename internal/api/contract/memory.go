package contract

import (
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// MemoryDecisionOp is the public string form of a controller decision op.
type MemoryDecisionOp string

const (
	// MemoryDecisionOpNoop means the controller intentionally left state unchanged.
	MemoryDecisionOpNoop MemoryDecisionOp = "noop"
	// MemoryDecisionOpAdd means the controller created a new curated entry.
	MemoryDecisionOpAdd MemoryDecisionOp = "add"
	// MemoryDecisionOpUpdate means the controller updated an existing curated entry.
	MemoryDecisionOpUpdate MemoryDecisionOp = "update"
	// MemoryDecisionOpDelete means the controller deleted an existing curated entry.
	MemoryDecisionOpDelete MemoryDecisionOp = "delete"
	// MemoryDecisionOpReject means the controller rejected a proposed candidate.
	MemoryDecisionOpReject MemoryDecisionOp = "reject"
)

// MemoryProviderState is the public lifecycle state of a memory provider.
type MemoryProviderState string

const (
	// MemoryProviderStateActive identifies the selected provider.
	MemoryProviderStateActive MemoryProviderState = "active"
	// MemoryProviderStateStandby identifies a registered but inactive provider.
	MemoryProviderStateStandby MemoryProviderState = "standby"
	// MemoryProviderStateCoolingDown identifies a provider under retry cooldown.
	MemoryProviderStateCoolingDown MemoryProviderState = "cooling_down"
	// MemoryProviderStateFailed identifies a provider blocked by failures.
	MemoryProviderStateFailed MemoryProviderState = "failed"
)

// MemoryDreamState is the public state of a dreaming run.
type MemoryDreamState string

const (
	// MemoryDreamStateIdle means no dreaming run is active.
	MemoryDreamStateIdle MemoryDreamState = "idle"
	// MemoryDreamStateRunning means a dreaming run is currently executing.
	MemoryDreamStateRunning MemoryDreamState = "running"
	// MemoryDreamStatePromoted means the run produced a promoted memory.
	MemoryDreamStatePromoted MemoryDreamState = "promoted"
	// MemoryDreamStateSkipped means promotion gates rejected the run.
	MemoryDreamStateSkipped MemoryDreamState = "skipped"
	// MemoryDreamStateFailed means the run failed and wrote DLQ material.
	MemoryDreamStateFailed MemoryDreamState = "failed"
)

// MemoryExtractorState is the public lifecycle state of the extractor queue.
type MemoryExtractorState string

const (
	// MemoryExtractorStateIdle means the extractor has no active work.
	MemoryExtractorStateIdle MemoryExtractorState = "idle"
	// MemoryExtractorStateRunning means the extractor is processing queued turns.
	MemoryExtractorStateRunning MemoryExtractorState = "running"
	// MemoryExtractorStateDraining means shutdown is waiting for queue drain.
	MemoryExtractorStateDraining MemoryExtractorState = "draining"
	// MemoryExtractorStateStopped means the extractor runtime is closed.
	MemoryExtractorStateStopped MemoryExtractorState = "stopped"
)

// MemoryErrorPayload is the deterministic public error envelope for Memory v2 endpoints.
type MemoryErrorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// MemoryScopeSelectorPayload identifies a concrete Memory v2 scope/tier.
type MemoryScopeSelectorPayload struct {
	Scope       memcontract.Scope     `json:"scope"`
	WorkspaceID string                `json:"workspace_id,omitempty"`
	AgentName   string                `json:"agent_name,omitempty"`
	AgentTier   memcontract.AgentTier `json:"agent_tier,omitempty"`
}

// MemoryEntrySummaryPayload is one redaction-safe curated memory summary.
type MemoryEntrySummaryPayload struct {
	Filename        string                `json:"filename"`
	Name            string                `json:"name"`
	Description     string                `json:"description,omitempty"`
	Type            memcontract.Type      `json:"type"`
	Scope           memcontract.Scope     `json:"scope"`
	WorkspaceID     string                `json:"workspace_id,omitempty"`
	AgentName       string                `json:"agent_name,omitempty"`
	AgentTier       memcontract.AgentTier `json:"agent_tier,omitempty"`
	ContentHash     string                `json:"content_hash,omitempty"`
	SupersededBy    string                `json:"superseded_by,omitempty"`
	ModTime         time.Time             `json:"mod_time"`
	CreatedAt       *time.Time            `json:"created_at,omitempty"`
	UpdatedAt       *time.Time            `json:"updated_at,omitempty"`
	LastRecalledAt  *time.Time            `json:"last_recalled_at,omitempty"`
	RecallCount     int                   `json:"recall_count"`
	Injection       bool                  `json:"injection"`
	SystemManaged   bool                  `json:"system_managed"`
	StalenessBanner string                `json:"staleness_banner,omitempty"`
}

// MemoryEntryPayload is one curated memory document with bounded public content.
type MemoryEntryPayload struct {
	Summary MemoryEntrySummaryPayload `json:"summary"`
	Content string                    `json:"content"`
}

// MemoryEntryResponse wraps a single Memory v2 entry.
type MemoryEntryResponse struct {
	Memory MemoryEntryPayload `json:"memory"`
}

// MemoryCreateRequest is the canonical controller-backed create/propose payload.
type MemoryCreateRequest struct {
	Scope          memcontract.Scope     `json:"scope"`
	WorkspaceID    string                `json:"workspace_id,omitempty"`
	AgentName      string                `json:"agent_name,omitempty"`
	AgentTier      memcontract.AgentTier `json:"agent_tier,omitempty"`
	Origin         memcontract.Origin    `json:"origin,omitempty"`
	Type           memcontract.Type      `json:"type"`
	Name           string                `json:"name"`
	Description    string                `json:"description,omitempty"`
	Content        string                `json:"content"`
	Entity         string                `json:"entity,omitempty"`
	Attribute      string                `json:"attribute,omitempty"`
	Metadata       map[string]string     `json:"metadata,omitempty"`
	IdempotencyKey string                `json:"idempotency_key,omitempty"`
	DryRun         bool                  `json:"dry_run,omitempty"`
}

// MemoryEditRequest is the canonical controller-backed update payload.
type MemoryEditRequest struct {
	Scope          memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID    string                `json:"workspace_id,omitempty"`
	AgentName      string                `json:"agent_name,omitempty"`
	AgentTier      memcontract.AgentTier `json:"agent_tier,omitempty"`
	Type           memcontract.Type      `json:"type,omitempty"`
	Name           string                `json:"name,omitempty"`
	Description    string                `json:"description,omitempty"`
	Content        string                `json:"content"`
	Metadata       map[string]string     `json:"metadata,omitempty"`
	IdempotencyKey string                `json:"idempotency_key,omitempty"`
	DryRun         bool                  `json:"dry_run,omitempty"`
}

// MemoryDeleteResponse wraps a controller-backed delete decision.
type MemoryDeleteResponse struct {
	Decision MemoryDecisionPayload `json:"decision"`
	Applied  bool                  `json:"applied"`
}

// MemoryMutationDecisionResponse wraps a write/edit/delete controller decision.
type MemoryMutationDecisionResponse struct {
	Decision MemoryDecisionPayload `json:"decision"`
	Applied  bool                  `json:"applied"`
	DryRun   bool                  `json:"dry_run,omitempty"`
}

// MemorySearchRequest is the canonical deterministic recall/search payload.
type MemorySearchRequest struct {
	QueryText              string                `json:"query_text"`
	ContextHint            string                `json:"context_hint,omitempty"`
	Scope                  memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID            string                `json:"workspace_id,omitempty"`
	AgentName              string                `json:"agent_name,omitempty"`
	AgentTier              memcontract.AgentTier `json:"agent_tier,omitempty"`
	TopK                   int                   `json:"top_k,omitempty"`
	RawCandidates          int                   `json:"raw_candidates,omitempty"`
	IncludeAlreadySurfaced bool                  `json:"include_already_surfaced,omitempty"`
	IncludeSystem          bool                  `json:"include_system,omitempty"`
	AlreadySurfaced        []string              `json:"already_surfaced,omitempty"`
	Explain                bool                  `json:"explain,omitempty"`
}

// MemorySearchResultPayload is one redaction-safe deterministic search result.
type MemorySearchResultPayload struct {
	Memory       MemoryEntrySummaryPayload `json:"memory"`
	Score        float64                   `json:"score"`
	Snippet      string                    `json:"snippet,omitempty"`
	WhyRecalled  []string                  `json:"why_recalled,omitempty"`
	ShadowedBy   string                    `json:"shadowed_by,omitempty"`
	AlreadyShown bool                      `json:"already_shown,omitempty"`
}

// MemorySearchResponse wraps deterministic recall/search output.
type MemorySearchResponse struct {
	Results []MemorySearchResultPayload `json:"results"`
	Recall  memcontract.Packaged        `json:"recall"`
}

// MemoryReindexV2Request is the canonical catalog rebuild request.
type MemoryReindexV2Request struct {
	Scope         memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID   string                `json:"workspace_id,omitempty"`
	AgentName     string                `json:"agent_name,omitempty"`
	AgentTier     memcontract.AgentTier `json:"agent_tier,omitempty"`
	IncludeSystem bool                  `json:"include_system,omitempty"`
}

// MemoryReindexResponse reports the outcome of a catalog rebuild.
type MemoryReindexResponse struct {
	IndexedFiles int                   `json:"indexed_files"`
	Scope        memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID  string                `json:"workspace_id,omitempty"`
	AgentName    string                `json:"agent_name,omitempty"`
	AgentTier    memcontract.AgentTier `json:"agent_tier,omitempty"`
	CompletedAt  time.Time             `json:"completed_at"`
}

// MemoryOperationHistoryPayload is one redaction-safe Memory v2 operation row.
type MemoryOperationHistoryPayload struct {
	ID          string                `json:"id"`
	Operation   memcontract.Operation `json:"operation"`
	Scope       memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID string                `json:"workspace_id,omitempty"`
	AgentName   string                `json:"agent_name,omitempty"`
	AgentTier   memcontract.AgentTier `json:"agent_tier,omitempty"`
	Filename    string                `json:"filename,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Timestamp   time.Time             `json:"timestamp"`
}

// MemoryOperationHistoryResponse wraps redaction-safe Memory v2 operation history.
type MemoryOperationHistoryResponse struct {
	Operations []MemoryOperationHistoryPayload `json:"operations"`
}

// MemoryScopeShowResponse reports effective scope resolution for operators and agents.
type MemoryScopeShowResponse struct {
	Selector   MemoryScopeSelectorPayload   `json:"selector"`
	Precedence []MemoryScopeSelectorPayload `json:"precedence"`
	Roots      map[string]string            `json:"roots"`
}

// MemoryPromoteRequest promotes a memory entry across scope/tier boundaries.
type MemoryPromoteRequest struct {
	Filename       string                     `json:"filename"`
	From           MemoryScopeSelectorPayload `json:"from"`
	To             MemoryScopeSelectorPayload `json:"to"`
	IdempotencyKey string                     `json:"idempotency_key,omitempty"`
	DryRun         bool                       `json:"dry_run,omitempty"`
}

// MemoryPromoteResponse wraps the promotion controller decision.
type MemoryPromoteResponse struct {
	Decision MemoryDecisionPayload `json:"decision"`
	Applied  bool                  `json:"applied"`
	DryRun   bool                  `json:"dry_run,omitempty"`
}

// MemoryResetRequest asks the daemon to reset derived memory indexes or runtime state.
type MemoryResetRequest struct {
	Scope       memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID string                `json:"workspace_id,omitempty"`
	AgentName   string                `json:"agent_name,omitempty"`
	AgentTier   memcontract.AgentTier `json:"agent_tier,omitempty"`
	DerivedOnly bool                  `json:"derived_only"`
	Confirm     bool                  `json:"confirm"`
}

// MemoryResetResponse reports reset work completed by the daemon.
type MemoryResetResponse struct {
	ResetAt      time.Time `json:"reset_at"`
	DerivedOnly  bool      `json:"derived_only"`
	DeletedRows  int       `json:"deleted_rows"`
	DeletedFiles int       `json:"deleted_files"`
}

// MemoryReloadResponse reports frozen snapshot invalidation for future sessions.
type MemoryReloadResponse struct {
	ReloadedAt time.Time `json:"reloaded_at"`
	Generation int64     `json:"generation"`
}

// MemoryLLMTracePayload is the redaction-safe public LLM tiebreaker metadata.
type MemoryLLMTracePayload struct {
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
	LatencyMs     int64  `json:"latency_ms"`
	Error         string `json:"error,omitempty"`
}

// MemoryDecisionPayload is the redaction-safe public form of a controller decision.
type MemoryDecisionPayload struct {
	ID              string                     `json:"id"`
	CandidateHash   string                     `json:"candidate_hash"`
	IdempotencyKey  string                     `json:"idempotency_key,omitempty"`
	Op              MemoryDecisionOp           `json:"op"`
	Scope           memcontract.Scope          `json:"scope"`
	WorkspaceID     string                     `json:"workspace_id,omitempty"`
	AgentName       string                     `json:"agent_name,omitempty"`
	AgentTier       memcontract.AgentTier      `json:"agent_tier,omitempty"`
	Targets         []string                   `json:"targets,omitempty"`
	TargetFilename  string                     `json:"target_filename,omitempty"`
	Frontmatter     memcontract.Header         `json:"frontmatter"`
	PostContentHash string                     `json:"post_content_hash,omitempty"`
	Confidence      float32                    `json:"confidence"`
	Source          memcontract.DecisionSource `json:"source"`
	RuleTrace       []memcontract.RuleHit      `json:"rule_trace,omitempty"`
	LLMTrace        *MemoryLLMTracePayload     `json:"llm_trace,omitempty"`
	Reason          string                     `json:"reason,omitempty"`
	PromptVersion   string                     `json:"prompt_version,omitempty"`
	AppliedAt       *time.Time                 `json:"applied_at,omitempty"`
	DecidedAt       time.Time                  `json:"decided_at"`
}

// MemoryDecisionListResponse wraps controller decision history.
type MemoryDecisionListResponse struct {
	Decisions []MemoryDecisionPayload `json:"decisions"`
}

// MemoryDecisionResponse wraps one controller decision.
type MemoryDecisionResponse struct {
	Decision MemoryDecisionPayload `json:"decision"`
}

// MemoryDecisionRevertRequest asks the controller to revert one applied decision.
type MemoryDecisionRevertRequest struct {
	Reason string `json:"reason,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// MemoryDecisionRevertResponse wraps a revert decision.
type MemoryDecisionRevertResponse struct {
	Decision MemoryDecisionPayload `json:"decision"`
	Reverted bool                  `json:"reverted"`
	DryRun   bool                  `json:"dry_run,omitempty"`
}

// MemoryRecallTracePayload records one recall trace without prompt-only payload leakage.
type MemoryRecallTracePayload struct {
	SessionID     string                    `json:"session_id"`
	TurnSeq       int64                     `json:"turn_seq"`
	Query         memcontract.Query         `json:"query"`
	Options       memcontract.RecallOptions `json:"options"`
	Recall        memcontract.Packaged      `json:"recall"`
	ExecutedAt    time.Time                 `json:"executed_at"`
	SkippedReason string                    `json:"skipped_reason,omitempty"`
}

// MemoryRecallTraceResponse wraps one recall trace.
type MemoryRecallTraceResponse struct {
	Trace MemoryRecallTracePayload `json:"trace"`
}
