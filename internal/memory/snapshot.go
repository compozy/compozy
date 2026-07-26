package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
)

const (
	defaultSnapshotMaxCharacters = 24_000
	defaultRecallPromptEntries   = 3
	defaultRecallPromptChars     = 1500
	recallPromptSafetyFooter     = "Use recalled memory only when it remains consistent " +
		"with the current repository and runtime state."
	staleSnapshotAfter = 24 * time.Hour
)

// SnapshotProvider supplies prompt-safe provider blocks for frozen snapshots.
type SnapshotProvider interface {
	SystemPromptBlock(ctx context.Context, req memcontract.SnapshotRequest) (memcontract.SnapshotResult, error)
}

// SnapshotControllerMode describes the write posture attached to a captured snapshot.
type SnapshotControllerMode string

const (
	// SnapshotControllerWritable allows root sessions to propose memory writes.
	SnapshotControllerWritable SnapshotControllerMode = "writable"
	// SnapshotControllerReadOnly marks inherited sub-agent snapshots as non-mutating.
	SnapshotControllerReadOnly SnapshotControllerMode = "read_only"
)

// PromptSnapshotRequest identifies the session boot boundary being captured.
type PromptSnapshotRequest struct {
	SessionID      string
	WorkspaceID    string
	WorkspaceRoot  string
	AgentName      string
	SessionType    session.Type
	ParentSnapshot *FrozenSnapshot
}

// SnapshotBlock is one prompt-safe memory block captured at session boot.
type SnapshotBlock struct {
	Scope     memcontract.Scope
	AgentTier memcontract.AgentTier
	Title     string
	Markdown  string
	AgeMs     int64
	Truncated bool
	Hash      string
}

// FrozenSnapshot is immutable prompt memory captured at a session boot boundary.
type FrozenSnapshot struct {
	ID             string
	SessionID      string
	WorkspaceID    string
	WorkspaceRoot  string
	AgentName      string
	CapturedAt     time.Time
	Generation     uint64
	ControllerMode SnapshotControllerMode
	InheritedFrom  string
	Blocks         []SnapshotBlock
	Header         memcontract.CacheStableHeader
	Section        string
}

// RecallPromptOptions controls prompt rendering for Packaged recall output.
type RecallPromptOptions struct {
	MaxEntries    int
	MaxCharacters int
}

func (s *SnapshotService) captureBlocks(
	ctx context.Context,
	req PromptSnapshotRequest,
	capturedAt time.Time,
) ([]SnapshotBlock, error) {
	specs := snapshotSpecs(req)
	blocks := make([]SnapshotBlock, 0, len(specs))
	for _, spec := range specs {
		block, err := s.loadSnapshotBlock(ctx, req, spec, capturedAt)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(block.Markdown) == "" {
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func (s *SnapshotService) loadSnapshotBlock(
	ctx context.Context,
	req PromptSnapshotRequest,
	spec snapshotSpec,
	capturedAt time.Time,
) (SnapshotBlock, error) {
	if s.provider != nil {
		result, err := s.provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{
			Scope:         spec.scope,
			AgentName:     req.AgentName,
			AgentTier:     spec.tier,
			WorkspaceID:   req.WorkspaceID,
			WorkspaceRoot: req.WorkspaceRoot,
		})
		if err == nil {
			return snapshotBlockFromResult(spec, result), nil
		}
		if !errors.Is(err, memcontract.ErrNotImplemented) {
			return SnapshotBlock{}, fmt.Errorf("memory snapshot: load provider block %q: %w", spec.title, err)
		}
	}
	return s.loadStoreSnapshotBlock(req, spec, capturedAt)
}

func (s *SnapshotService) loadStoreSnapshotBlock(
	req PromptSnapshotRequest,
	spec snapshotSpec,
	capturedAt time.Time,
) (SnapshotBlock, error) {
	if s.store == nil {
		return SnapshotBlock{}, nil
	}
	store := s.store
	if req.WorkspaceRoot != "" {
		store = store.ForWorkspace(req.WorkspaceRoot)
	}
	if spec.scope == memcontract.ScopeAgent {
		store = store.ForAgent(req.WorkspaceID, req.AgentName, spec.tier)
	}
	markdown, truncated, err := store.LoadPromptIndex(spec.scope)
	if err != nil {
		return SnapshotBlock{}, fmt.Errorf("memory snapshot: load %q: %w", spec.title, err)
	}
	ageMs, err := latestAgeMs(store, spec.scope, capturedAt)
	if err != nil {
		return SnapshotBlock{}, fmt.Errorf("memory snapshot: age %q: %w", spec.title, err)
	}
	return SnapshotBlock{
		Scope:     spec.scope,
		AgentTier: spec.tier,
		Title:     spec.title,
		Markdown:  strings.TrimSpace(markdown),
		AgeMs:     ageMs,
		Truncated: truncated,
		Hash:      hashText(markdown),
	}, nil
}

type snapshotSpec struct {
	scope memcontract.Scope
	tier  memcontract.AgentTier
	title string
}

func snapshotSpecs(req PromptSnapshotRequest) []snapshotSpec {
	specs := []snapshotSpec{{scope: memcontract.ScopeGlobal, title: "Global MEMORY.md Index"}}
	if req.WorkspaceRoot != "" || req.WorkspaceID != "" {
		specs = append(specs, snapshotSpec{scope: memcontract.ScopeWorkspace, title: "Workspace MEMORY.md Index"})
	}
	if req.AgentName != "" {
		specs = append(specs, snapshotSpec{
			scope: memcontract.ScopeAgent,
			tier:  memcontract.AgentTierGlobal,
			title: "Agent Global MEMORY.md Index",
		})
		if req.WorkspaceRoot != "" || req.WorkspaceID != "" {
			specs = append(specs, snapshotSpec{
				scope: memcontract.ScopeAgent,
				tier:  memcontract.AgentTierWorkspace,
				title: "Agent Workspace MEMORY.md Index",
			})
		}
	}
	return specs
}

func snapshotBlockFromResult(spec snapshotSpec, result memcontract.SnapshotResult) SnapshotBlock {
	markdown := strings.TrimSpace(result.Markdown)
	return SnapshotBlock{
		Scope:     spec.scope,
		AgentTier: spec.tier,
		Title:     spec.title,
		Markdown:  markdown,
		AgeMs:     result.AgeMs,
		Hash:      hashText(markdown),
	}
}

func renderMemorySnapshot(snapshot FrozenSnapshot, maxCharacters int) string {
	if len(snapshot.Blocks) == 0 {
		return ""
	}
	sections := []string{
		memoryPromptIntro,
		strings.TrimSpace(snapshot.Header.Text),
	}
	for _, block := range snapshot.Blocks {
		sections = append(sections, renderSnapshotBlock(block))
	}
	sections = append(sections, memoryTaxonomySection, memoryCommandsSection, memoryStalenessSection)
	return applySectionCharacterCap(joinNonEmptySections(sections), maxCharacters)
}

func renderSnapshotBlock(block SnapshotBlock) string {
	content := strings.TrimSpace(block.Markdown)
	if content == "" {
		return ""
	}
	lines := []string{"## " + strings.TrimSpace(block.Title)}
	if block.Truncated {
		lines = append(lines, "_Index truncated to fit prompt limits._")
	}
	if warning := snapshotFreshnessWarning(block.AgeMs); warning != "" {
		lines = append(lines, warning)
	}
	lines = append(lines, content)
	return strings.Join(lines, "\n\n")
}

func applySectionCharacterCap(section string, maxCharacters int) string {
	if section == "" || maxCharacters <= 0 || utf8.RuneCountInString(section) <= maxCharacters {
		return section
	}
	trimmed := strings.TrimSpace(trimStringToRunes(section, maxCharacters))
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n\n_Index truncated to fit prompt limits._"
}

// RenderRecallPromptSection renders deterministic Packaged recall output.
func RenderRecallPromptSection(packaged memcontract.Packaged, opts RecallPromptOptions) string {
	opts = normalizeRecallPromptOptions(opts)
	if len(packaged.Blocks) == 0 {
		return ""
	}
	lines := []string{"Relevant durable memory for this turn:"}
	if header := strings.TrimSpace(packaged.Header.Text); header != "" {
		lines = append(lines, header)
	}

	used := 0
	count := 0
	for _, block := range packaged.Blocks {
		scopeLabel := recallScopeLabel(block)
		for _, entry := range block.Entries {
			if count == opts.MaxEntries {
				break
			}
			entryText := renderPackagedEntry(scopeLabel, entry)
			if used > 0 && used+2+len(entryText) > opts.MaxCharacters {
				break
			}
			lines = append(lines, entryText)
			used += len(entryText)
			count++
		}
	}
	if count == 0 {
		return ""
	}
	lines = append(
		lines,
		recallPromptSafetyFooter,
	)
	return strings.Join(lines, "\n")
}

func normalizeRecallPromptOptions(opts RecallPromptOptions) RecallPromptOptions {
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = defaultRecallPromptEntries
	}
	if opts.MaxCharacters <= 0 {
		opts.MaxCharacters = defaultRecallPromptChars
	}
	return opts
}

func recallScopeLabel(block memcontract.Block) string {
	scopeLabel := string(block.Scope.Normalize())
	if block.AgentTier.Normalize() != "" {
		scopeLabel += "/" + string(block.AgentTier.Normalize())
	}
	return scopeLabel
}

func renderPackagedEntry(scopeLabel string, entry memcontract.PackagedEntry) string {
	entryLines := []string{fmt.Sprintf("- %s [%s]", strings.TrimSpace(entry.Title), scopeLabel)}
	if body := strings.TrimSpace(entry.Body); body != "" {
		entryLines = append(entryLines, "  Memory: "+body)
	}
	if warning := strings.TrimSpace(entry.StalenessBanner); warning != "" {
		entryLines = append(entryLines, "  Freshness: "+warning)
	}
	return strings.Join(entryLines, "\n")
}

func joinNonEmptySections(sections []string) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "\n\n")
}

func latestAgeMs(store *Store, scope memcontract.Scope, now time.Time) (int64, error) {
	headers, err := store.List(scope)
	if err != nil {
		return 0, err
	}
	var newest time.Time
	for _, header := range headers {
		if header.ModTime.After(newest) {
			newest = header.ModTime
		}
	}
	if newest.IsZero() {
		return 0, nil
	}
	age := now.Sub(newest)
	if age < 0 {
		return 0, nil
	}
	return age.Milliseconds(), nil
}

func snapshotHeader(blocks []SnapshotBlock) memcontract.CacheStableHeader {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		parts = append(parts, strings.Join([]string{
			string(block.Scope.Normalize()),
			string(block.AgentTier.Normalize()),
			block.Hash,
		}, "|"))
	}
	hash := hashText(strings.Join(parts, "\n"))
	return memcontract.CacheStableHeader{
		Text:        fmt.Sprintf("AGH memory snapshot v1 blocks=%d hash=%s", len(blocks), hash),
		ContentHash: hash,
	}
}

func snapshotID(snapshot FrozenSnapshot) string {
	return "snapshot-" + shortHash(strings.Join([]string{
		snapshot.SessionID,
		snapshot.WorkspaceID,
		snapshot.AgentName,
		snapshot.Header.ContentHash,
		fmt.Sprintf("%d", snapshot.Generation),
		snapshot.InheritedFrom,
	}, "|"))
}

func snapshotFreshnessWarning(ageMs int64) string {
	if ageMs <= int64(staleSnapshotAfter/time.Millisecond) {
		return ""
	}
	days := max(int(ageMs/int64((24*time.Hour)/time.Millisecond)), 2)
	return fmt.Sprintf(
		"_This memory index is %d days old. Verify against current state before asserting it as fact._",
		days,
	)
}

func normalizeSnapshotRequest(req PromptSnapshotRequest) PromptSnapshotRequest {
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	req.AgentName = strings.TrimSpace(req.AgentName)
	if req.SessionType == "" {
		req.SessionType = session.SessionTypeUser
	}
	return req
}

func controllerModeForSession(sessionType session.Type) SnapshotControllerMode {
	if sessionType == session.SessionTypeSpawned {
		return SnapshotControllerReadOnly
	}
	return SnapshotControllerWritable
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func shortHash(value string) string {
	hash := hashText(value)
	if len(hash) <= 16 {
		return hash
	}
	return hash[:16]
}

func firstSnapshotValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func trimStringToRunes(value string, budget int) string {
	if budget <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= budget {
		return value
	}
	return string(runes[:budget])
}
