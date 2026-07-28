package contract

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ErrNotImplemented lets optional provider methods request the local fallback.
var ErrNotImplemented = errors.New("memory provider: not implemented")

// Header contains validated metadata parsed from a memory file frontmatter.
type Header struct {
	Filename    string      `json:"filename"               yaml:"-"`
	FilePath    string      `json:"-"                      yaml:"-"`
	ModTime     time.Time   `json:"mod_time"               yaml:"-"`
	Name        string      `json:"name"                   yaml:"name"`
	Description string      `json:"description,omitempty"  yaml:"description,omitempty"`
	Type        Type        `json:"type"                   yaml:"type"`
	Scope       Scope       `json:"scope,omitempty"        yaml:"scope,omitempty"`
	WorkspaceID string      `json:"workspace_id,omitempty" yaml:"-"`
	AgentName   string      `json:"agent_name,omitempty"   yaml:"agent,omitempty"`
	AgentTier   AgentTier   `json:"agent_tier,omitempty"   yaml:"agent_tier,omitempty"`
	Provenance  *Provenance `json:"provenance,omitempty"   yaml:"provenance,omitempty"`
}

// Normalize trims and normalizes the parsed memory header metadata in place.
func (h *Header) Normalize() {
	h.Name = strings.TrimSpace(h.Name)
	h.Description = strings.TrimSpace(h.Description)
	h.Type = h.Type.Normalize()
	h.Scope = h.Scope.Normalize()
	h.WorkspaceID = strings.TrimSpace(h.WorkspaceID)
	h.AgentName = strings.TrimSpace(h.AgentName)
	h.AgentTier = h.AgentTier.Normalize()
	if h.Provenance != nil {
		h.Provenance.Normalize()
	}
}

// Validate reports whether the parsed memory header is complete and valid.
func (h *Header) Validate() error {
	h.Normalize()
	if h.Name == "" {
		return fmt.Errorf("memory name is required")
	}
	if err := h.Type.Validate(); err != nil {
		return err
	}
	if h.Scope != "" {
		if err := h.Scope.Validate(); err != nil {
			return err
		}
	}
	if h.Scope == ScopeAgent {
		if h.AgentName == "" {
			return fmt.Errorf("agent name is required for agent-scoped memory")
		}
		if h.AgentTier == "" {
			return fmt.Errorf("agent tier is required for agent-scoped memory")
		}
		if err := h.AgentTier.Validate(); err != nil {
			return err
		}
	} else {
		if h.AgentName != "" {
			return fmt.Errorf("agent name requires agent scope")
		}
		if h.AgentTier != "" {
			return fmt.Errorf("agent tier requires agent scope")
		}
	}
	if h.Provenance != nil {
		if err := h.Provenance.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Provenance records how a curated memory entry was created or superseded.
type Provenance struct {
	SourceSessionIDs []string  `json:"source_session_ids,omitempty" yaml:"source_sessions,omitempty"`
	SourceActor      Origin    `json:"source_actor"                 yaml:"source_actor"`
	Confidence       string    `json:"confidence,omitempty"         yaml:"confidence,omitempty"`
	SupersededBy     string    `json:"superseded_by,omitempty"      yaml:"superseded_by,omitempty"`
	CreatedAt        time.Time `json:"created_at"                   yaml:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"                   yaml:"updated_at"`
}

// Normalize trims and normalizes provenance metadata in place.
func (p *Provenance) Normalize() {
	p.SourceActor = p.SourceActor.Normalize()
	p.Confidence = strings.TrimSpace(p.Confidence)
	p.SupersededBy = strings.TrimSpace(p.SupersededBy)
	for i, id := range p.SourceSessionIDs {
		p.SourceSessionIDs[i] = strings.TrimSpace(id)
	}
}

// Validate reports whether provenance metadata is complete and valid.
func (p *Provenance) Validate() error {
	if p == nil {
		return nil
	}
	p.Normalize()
	if err := p.SourceActor.Validate(); err != nil {
		return fmt.Errorf("provenance source actor: %w", err)
	}
	return nil
}

// SearchOptions controls catalog-backed or fallback memory search behavior.
type SearchOptions struct {
	Scope     Scope
	Workspace string
	Limit     int
}

// SearchResult is one ranked memory search hit.
type SearchResult struct {
	Filename    string    `json:"filename"`
	Scope       Scope     `json:"scope"`
	Workspace   string    `json:"workspace,omitempty"`
	Type        Type      `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Score       float64   `json:"score"`
	Snippet     string    `json:"snippet,omitempty"`
	ModTime     time.Time `json:"mod_time"`
}

// ReindexOptions controls which scopes are rebuilt into the derived catalog.
type ReindexOptions struct {
	Scope     Scope
	Workspace string
}

// ReindexResult reports the outcome of a catalog rebuild.
type ReindexResult struct {
	IndexedFiles int       `json:"indexed_files"`
	Scope        Scope     `json:"scope,omitempty"`
	Workspace    string    `json:"workspace,omitempty"`
	CompletedAt  time.Time `json:"completed_at"`
}

// OperationHistoryQuery filters durable memory operation history.
type OperationHistoryQuery struct {
	Scope     Scope
	Workspace string
	Operation Operation
	Since     time.Time
	Limit     int
}

// OperationRecord is one redacted durable memory operation history row.
type OperationRecord struct {
	ID        string    `json:"id"`
	Operation Operation `json:"operation"`
	Scope     Scope     `json:"scope,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	Filename  string    `json:"filename,omitempty"`
	AgentName string    `json:"agent_name,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthStats summarizes derived-catalog state for operator surfaces.
type HealthStats struct {
	IndexedFiles    int        `json:"indexed_files"`
	OrphanedFiles   int        `json:"orphaned_files"`
	LastReindex     *time.Time `json:"last_reindex"`
	OperationCount  int        `json:"operation_count"`
	LastOperationAt *time.Time `json:"last_operation_at"`
}

// Backend captures the memory backend surface used by daemon, API, and CLI layers.
type Backend interface {
	List(scope Scope) ([]Header, error)
	Read(scope Scope, filename string) ([]byte, error)
	Write(scope Scope, filename string, content []byte) error
	Delete(scope Scope, filename string) error
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
	Reindex(ctx context.Context, opts ReindexOptions) (ReindexResult, error)
	History(ctx context.Context, query OperationHistoryQuery) ([]OperationRecord, error)
	LoadPromptIndex(scope Scope) (content string, truncated bool, err error)
}

// RuleHit records one deterministic rule that contributed to a write decision.
type RuleHit struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason,omitempty"`
	Target  string `json:"target,omitempty"`
	Details string `json:"details,omitempty"`
}

// LLMCall records bounded metadata for an LLM tiebreaker call.
type LLMCall struct {
	Model         string        `json:"model"`
	PromptVersion string        `json:"prompt_version"`
	Latency       time.Duration `json:"latency"`
	RawResponse   string        `json:"raw_response,omitempty"`
	Error         string        `json:"error,omitempty"`
}

// Candidate carries one fact proposed for the curated layer.
type Candidate struct {
	WorkspaceID       string            `json:"workspace_id,omitempty"`
	Scope             Scope             `json:"scope"`
	AgentName         string            `json:"agent_name,omitempty"`
	AgentTier         AgentTier         `json:"agent_tier,omitempty"`
	Origin            Origin            `json:"origin"`
	Content           string            `json:"content"`
	Frontmatter       Header            `json:"frontmatter"`
	TrustedRawContent string            `json:"-"`
	Entity            string            `json:"entity,omitempty"`
	Attribute         string            `json:"attribute,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	SubmittedAt       time.Time         `json:"submitted_at"`
}

// Decision carries enough material to deterministically replay a file mutation.
type Decision struct {
	ID              string         `json:"id"`
	CandidateHash   string         `json:"candidate_hash"`
	IdempotencyKey  string         `json:"idempotency_key"`
	Op              Op             `json:"op"`
	Targets         []string       `json:"targets,omitempty"`
	TargetFilename  string         `json:"target_filename"`
	Frontmatter     Header         `json:"frontmatter"`
	PostContent     string         `json:"post_content,omitempty"`
	PostContentHash string         `json:"post_content_hash,omitempty"`
	PriorContent    string         `json:"prior_content,omitempty"`
	Confidence      float32        `json:"confidence"`
	Source          DecisionSource `json:"source"`
	RuleTrace       []RuleHit      `json:"rule_trace,omitempty"`
	LLMTrace        *LLMCall       `json:"llm_trace,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	PromptVersion   string         `json:"prompt_version,omitempty"`
	DecidedAt       time.Time      `json:"decided_at"`
}

// Controller decides how candidates mutate the curated memory layer.
type Controller interface {
	Decide(ctx context.Context, candidate Candidate) (Decision, error)
}

// Query describes one recall query.
type Query struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
	QueryText   string `json:"query_text"`
	ContextHint string `json:"context_hint,omitempty"`
}

// RecallOptions controls deterministic recall packaging.
type RecallOptions struct {
	TopK                   int      `json:"top_k,omitempty"`
	RawCandidates          int      `json:"raw_candidates,omitempty"`
	IncludeAlreadySurfaced bool     `json:"include_already_surfaced,omitempty"`
	IncludeSystem          bool     `json:"include_system,omitempty"`
	AlreadySurfaced        []string `json:"already_surfaced,omitempty"`
	AllowTrivialQuery      bool     `json:"allow_trivial_query,omitempty"`
}

// CacheStableHeader identifies the prompt-cache-stable header for a recall package.
type CacheStableHeader struct {
	Text        string `json:"text"`
	ContentHash string `json:"content_hash"`
}

// Packaged is the prompt-ready output of a recall query.
type Packaged struct {
	Blocks []Block           `json:"blocks"`
	Header CacheStableHeader `json:"header"`
}

// Block groups recalled entries by scope.
type Block struct {
	Scope     Scope           `json:"scope"`
	AgentTier AgentTier       `json:"agent_tier,omitempty"`
	Entries   []PackagedEntry `json:"entries"`
}

// PackagedEntry is one prompt-ready recalled memory entry.
type PackagedEntry struct {
	ID              string    `json:"id"`
	Filename        string    `json:"filename,omitempty"`
	Title           string    `json:"title"`
	Type            Type      `json:"type,omitempty"`
	WorkspaceID     string    `json:"workspace_id,omitempty"`
	Body            string    `json:"body"`
	ModTime         time.Time `json:"mod_time"`
	AgeDays         int       `json:"age_days"`
	StalenessBanner string    `json:"staleness_banner,omitempty"`
	WhyRecalled     []string  `json:"why_recalled,omitempty"`
}

// Recaller retrieves prompt-ready memory for a query.
type Recaller interface {
	Recall(ctx context.Context, query Query, opts RecallOptions) (Packaged, error)
}

// TranscriptMessage is one compact message in an extractor snapshot.
type TranscriptMessage struct {
	Sequence int64     `json:"sequence"`
	Role     string    `json:"role"`
	Content  string    `json:"content"`
	At       time.Time `json:"at"`
}

// TranscriptSnapshot contains bounded transcript material for extraction.
type TranscriptSnapshot struct {
	Messages []TranscriptMessage `json:"messages"`
}

// TurnRecord describes the message range inspected by the extractor.
type TurnRecord struct {
	SessionID       string             `json:"session_id"`
	RootSessionID   string             `json:"root_session_id"`
	ParentSessionID string             `json:"parent_session_id,omitempty"`
	AgentID         string             `json:"agent_id"`
	ActorKind       string             `json:"actor_kind"`
	WorkspaceID     string             `json:"workspace_id,omitempty"`
	SinceMessageSeq int64              `json:"since_message_seq"`
	UntilMessageSeq int64              `json:"until_message_seq"`
	Snapshot        TranscriptSnapshot `json:"snapshot"`
	Trigger         Trigger            `json:"trigger"`
}

// Extractor produces memory candidates from transcript turns.
type Extractor interface {
	Extract(ctx context.Context, turn TurnRecord) ([]Candidate, error)
	Drain(ctx context.Context) error
}

// ProviderInit configures a memory provider for one workspace.
type ProviderInit struct {
	WorkspaceID   string         `json:"workspace_id,omitempty"`
	WorkspaceRoot string         `json:"workspace_root,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	Logger        *slog.Logger   `json:"-"`
}

// SnapshotRequest asks a provider for a frozen prompt snapshot.
type SnapshotRequest struct {
	Scope         Scope     `json:"scope"`
	AgentName     string    `json:"agent_name,omitempty"`
	AgentTier     AgentTier `json:"agent_tier,omitempty"`
	WorkspaceID   string    `json:"workspace_id,omitempty"`
	WorkspaceRoot string    `json:"workspace_root,omitempty"`
}

// SnapshotResult is provider-supplied markdown for prompt injection.
type SnapshotResult struct {
	Markdown string `json:"markdown"`
	AgeMs    int64  `json:"age_ms"`
}

// RecallRequest asks a provider to recall memory.
type RecallRequest struct {
	Query
	Options RecallOptions `json:"options"`
}

// RecallResult wraps provider recall output.
type RecallResult struct {
	Packaged
}

// PrefetchRequest lets providers warm data before a turn.
type PrefetchRequest struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id"`
	AgentName   string `json:"agent_name,omitempty"`
	QueryText   string `json:"query_text,omitempty"`
}

// PreCompressRequest lets providers prepare before transcript compaction.
type PreCompressRequest struct {
	WorkspaceID  string             `json:"workspace_id,omitempty"`
	SessionID    string             `json:"session_id"`
	AgentName    string             `json:"agent_name,omitempty"`
	FromSequence int64              `json:"from_sequence"`
	ToSequence   int64              `json:"to_sequence"`
	Snapshot     TranscriptSnapshot `json:"snapshot"`
}

// PreCompressHint lets providers return memory guidance before compaction.
type PreCompressHint struct {
	Markdown string   `json:"markdown,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

// SessionEndRecord describes a completed session for provider synchronization.
type SessionEndRecord struct {
	WorkspaceID string             `json:"workspace_id,omitempty"`
	SessionID   string             `json:"session_id"`
	AgentName   string             `json:"agent_name,omitempty"`
	EndedAt     time.Time          `json:"ended_at"`
	Snapshot    TranscriptSnapshot `json:"snapshot"`
}

// SessionSwitchRecord describes a session lineage handoff.
type SessionSwitchRecord struct {
	WorkspaceID string    `json:"workspace_id,omitempty"`
	FromSession string    `json:"from_session"`
	ToSession   string    `json:"to_session"`
	SwitchedAt  time.Time `json:"switched_at"`
}

// WriteRecord is emitted to providers after a controller decision.
type WriteRecord struct {
	Decision  Decision  `json:"decision"`
	Candidate Candidate `json:"candidate"`
}

// MemoryProvider is the lifecycle interface for pluggable memory backends.
type MemoryProvider interface {
	Initialize(ctx context.Context, init ProviderInit) error
	SystemPromptBlock(ctx context.Context, req SnapshotRequest) (SnapshotResult, error)
	Recall(ctx context.Context, req RecallRequest) (RecallResult, error)
	Prefetch(ctx context.Context, req PrefetchRequest) error
	SyncTurn(ctx context.Context, rec TurnRecord) error
	OnSessionEnd(ctx context.Context, rec SessionEndRecord) error
	OnSessionSwitch(ctx context.Context, rec SessionSwitchRecord) error
	OnPreCompress(ctx context.Context, req PreCompressRequest) (PreCompressHint, error)
	OnMemoryWrite(ctx context.Context, rec WriteRecord) error
	Shutdown(ctx context.Context) error
}
