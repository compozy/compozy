package memory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/prompts"
	redactpkg "github.com/compozy/agh/internal/redact"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	// CheckpointSummaryFilename is the stable workspace-scoped continuity artifact.
	CheckpointSummaryFilename = "project_checkpoint_summary.md"

	checkpointSummaryName         = "Workspace Checkpoint Summary"
	checkpointSummaryDescription  = "Continuity checkpoint updated from completed workspace sessions."
	checkpointSummaryMaxBytes     = 24_000
	checkpointSummarySourceLimit  = 32
	checkpointSummaryFirstHeading = "## Historical Task Snapshot"
	checkpointSummaryLastHeading  = "## Critical Context"
)

const checkpointSummaryIntro = "# Workspace Checkpoint Summary\n\n" +
	"This block is reference-only continuity context from completed sessions. " +
	"Treat historical asks as stale unless the current user explicitly renews them."

var (
	// ErrCheckpointSummaryDisabled reports that the effective workspace role disables checkpoint generation.
	ErrCheckpointSummaryDisabled = errors.New("memory: checkpoint summary role is disabled")

	checkpointSummaryHeadings = []string{
		checkpointSummaryFirstHeading,
		"## Goal",
		"## Constraints & Preferences",
		"## Completed Actions",
		"## Active State",
		"## Historical In-Progress State",
		"## Blocked",
		"## Key Decisions",
		"## Resolved Questions",
		"## Historical Pending User Asks",
		"## Relevant Files",
		"## Historical Remaining Work",
		checkpointSummaryLastHeading,
	}
)

// CheckpointSummaryRequest is the bounded input supplied to a checkpoint summarizer.
type CheckpointSummaryRequest struct {
	WorkspaceID     string
	WorkspaceRoot   string
	SessionID       string
	AgentName       string
	EndedAt         time.Time
	PreviousSummary string
	Snapshot        memcontract.TranscriptSnapshot
}

// CheckpointSummarizer produces one structured Hermes-style checkpoint body.
type CheckpointSummarizer interface {
	Summarize(ctx context.Context, request CheckpointSummaryRequest) (string, error)
}

// CheckpointWorkspaceResolver resolves a stable workspace identity to its runtime root.
type CheckpointWorkspaceResolver interface {
	Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error)
}

// CheckpointSummaryService updates one workspace checkpoint through the decision WAL.
type CheckpointSummaryService struct {
	mu         sync.Mutex
	store      *Store
	resolver   CheckpointWorkspaceResolver
	summarizer CheckpointSummarizer
	now        func() time.Time
	maxBytes   int
}

// CheckpointSummaryOption customizes checkpoint summary updates.
type CheckpointSummaryOption func(*CheckpointSummaryService)

// WithCheckpointSummaryClock injects a deterministic checkpoint clock.
func WithCheckpointSummaryClock(now func() time.Time) CheckpointSummaryOption {
	return func(service *CheckpointSummaryService) {
		if service != nil && now != nil {
			service.now = now
		}
	}
}

// WithCheckpointSummaryMaxBytes overrides the complete artifact byte limit.
func WithCheckpointSummaryMaxBytes(maxBytes int) CheckpointSummaryOption {
	return func(service *CheckpointSummaryService) {
		if service != nil && maxBytes > 0 {
			service.maxBytes = maxBytes
		}
	}
}

// NewCheckpointSummaryService constructs a decision-backed checkpoint updater.
func NewCheckpointSummaryService(
	store *Store,
	resolver CheckpointWorkspaceResolver,
	summarizer CheckpointSummarizer,
	opts ...CheckpointSummaryOption,
) *CheckpointSummaryService {
	service := &CheckpointSummaryService{
		store:      store,
		resolver:   resolver,
		summarizer: summarizer,
		now:        func() time.Time { return time.Now().UTC() },
		maxBytes:   checkpointSummaryMaxBytes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

// OnSessionEnd implements the bundled local provider lifecycle handler.
func (s *CheckpointSummaryService) OnSessionEnd(
	ctx context.Context,
	record memcontract.SessionEndRecord,
) error {
	_, err := s.Update(ctx, record)
	return err
}

// Update incorporates one completed session into the stable workspace artifact.
func (s *CheckpointSummaryService) Update(
	ctx context.Context,
	record memcontract.SessionEndRecord,
) (DecisionApplyResult, error) {
	if s == nil {
		return DecisionApplyResult{}, errors.New("memory: checkpoint summary service is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateSessionEnd(ctx, record)
}

func (s *CheckpointSummaryService) validate(
	ctx context.Context,
	record memcontract.SessionEndRecord,
) error {
	switch {
	case s == nil:
		return errors.New("memory: checkpoint summary service is required")
	case ctx == nil:
		return errors.New("memory: checkpoint summary context is required")
	case s.store == nil:
		return errors.New("memory: checkpoint summary store is required")
	case s.resolver == nil:
		return errors.New("memory: checkpoint workspace resolver is required")
	case s.summarizer == nil:
		return errors.New("memory: checkpoint summarizer is required")
	case strings.TrimSpace(record.WorkspaceID) == "":
		return errors.New("memory: checkpoint workspace id is required")
	case strings.TrimSpace(record.SessionID) == "":
		return errors.New("memory: checkpoint session id is required")
	case len(record.Snapshot.Messages) == 0:
		return errors.New("memory: checkpoint transcript snapshot is empty")
	case ctx.Err() != nil:
		return fmt.Errorf("memory: checkpoint summary context: %w", ctx.Err())
	default:
		return nil
	}
}

// RenderCheckpointSummaryPrompt renders the versioned Hermes-style update prompt.
func RenderCheckpointSummaryPrompt(request CheckpointSummaryRequest) (string, error) {
	tmpl, err := prompts.ParseTemplate(prompts.NameCheckpointSummary, prompts.VersionV1)
	if err != nil {
		return "", err
	}
	previous := strings.TrimSpace(request.PreviousSummary)
	if previous == "" {
		previous = "None. This is the first workspace checkpoint."
	}
	var rendered bytes.Buffer
	data := map[string]any{
		"WorkspaceID":     strings.TrimSpace(request.WorkspaceID),
		"SessionID":       strings.TrimSpace(request.SessionID),
		"EndedAt":         request.EndedAt.UTC().Format(time.RFC3339Nano),
		"PreviousSummary": previous,
		"Transcript":      renderCheckpointTranscript(request.Snapshot),
	}
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("memory: render checkpoint summary prompt: %w", err)
	}
	return rendered.String(), nil
}

func loadCheckpointSummary(store *Store) (string, *memcontract.Header, error) {
	state, err := loadCheckpointSummaryState(store)
	return state.body, state.header, err
}

func checkpointSummaryHeader(at time.Time, prior *memcontract.Header) memcontract.Header {
	createdAt := at.UTC()
	sourceSessions := []string{}
	if prior != nil && prior.Provenance != nil {
		if !prior.Provenance.CreatedAt.IsZero() {
			createdAt = prior.Provenance.CreatedAt.UTC()
		}
		sourceSessions = append(sourceSessions, prior.Provenance.SourceSessionIDs...)
	}
	return memcontract.Header{
		Name:        checkpointSummaryName,
		Description: checkpointSummaryDescription,
		Type:        memcontract.TypeProject,
		Scope:       memcontract.ScopeWorkspace,
		Provenance: &memcontract.Provenance{
			SourceSessionIDs: sourceSessions,
			SourceActor:      memcontract.OriginProvider,
			Confidence:       "summarized",
			CreatedAt:        createdAt,
			UpdatedAt:        at.UTC(),
		},
	}
}

func renderCheckpointSummaryDocument(body string, header memcontract.Header) ([]byte, error) {
	return renderCheckpointSummaryState(checkpointSummaryState{body: body, header: &header})
}

func validateCheckpointSummary(summary string) error {
	trimmed := strings.TrimSpace(summary)
	if !utf8.ValidString(trimmed) {
		return errors.New("memory: checkpoint summary is not valid UTF-8")
	}
	if redactpkg.ContainsRawClaimToken(trimmed) {
		return errors.New("memory: checkpoint summary contains a raw claim token")
	}
	if strings.Contains(trimmed, "```") {
		return errors.New("memory: checkpoint summary must not contain Markdown fences")
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != checkpointSummaryFirstHeading {
		return fmt.Errorf("memory: checkpoint summary must begin with %q", checkpointSummaryFirstHeading)
	}
	found := make([]string, 0, len(checkpointSummaryHeadings))
	for _, line := range lines {
		if heading := strings.TrimSpace(line); strings.HasPrefix(heading, "## ") {
			found = append(found, heading)
		}
	}
	for _, heading := range checkpointSummaryHeadings {
		if !slices.Contains(found, heading) {
			return fmt.Errorf("memory: checkpoint summary missing required heading %q", heading)
		}
	}
	if len(found) != len(checkpointSummaryHeadings) {
		return fmt.Errorf(
			"memory: checkpoint summary contains %d level-two headings; expected %d",
			len(found),
			len(checkpointSummaryHeadings),
		)
	}
	for index, heading := range found {
		if heading != checkpointSummaryHeadings[index] {
			return fmt.Errorf(
				"memory: checkpoint summary heading %q is out of order; expected %q",
				heading,
				checkpointSummaryHeadings[index],
			)
		}
	}
	return nil
}

func renderCheckpointSummarySection(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	return strings.Join([]string{
		checkpointSummaryIntro,
		"<agh_checkpoint_summary>",
		trimmed,
		"</agh_checkpoint_summary>",
	}, "\n\n")
}

func appendCheckpointSourceSession(existing []string, sessionID string) []string {
	normalized := make([]string, 0, len(existing)+1)
	for _, id := range existing {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" || slices.Contains(normalized, trimmed) {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	target := strings.TrimSpace(sessionID)
	if target != "" {
		normalized = slices.DeleteFunc(normalized, func(id string) bool { return id == target })
		normalized = append(normalized, target)
	}
	if len(normalized) > checkpointSummarySourceLimit {
		normalized = normalized[len(normalized)-checkpointSummarySourceLimit:]
	}
	return normalized
}

func checkpointEndedAt(endedAt time.Time, now func() time.Time) time.Time {
	if !endedAt.IsZero() {
		return endedAt.UTC()
	}
	return now().UTC()
}

func cloneTranscriptSnapshot(snapshot memcontract.TranscriptSnapshot) memcontract.TranscriptSnapshot {
	return memcontract.TranscriptSnapshot{
		Messages: append([]memcontract.TranscriptMessage(nil), snapshot.Messages...),
	}
}

func renderCheckpointTranscript(snapshot memcontract.TranscriptSnapshot) string {
	var rendered strings.Builder
	for _, message := range snapshot.Messages {
		if _, err := fmt.Fprintf(
			&rendered,
			"- sequence=%d role=%s at=%s\n%s\n",
			message.Sequence,
			firstCheckpointValue(message.Role, "unknown"),
			message.At.UTC().Format(time.RFC3339Nano),
			strings.TrimSpace(message.Content),
		); err != nil {
			return rendered.String()
		}
	}
	return rendered.String()
}

func firstCheckpointValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
