package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// Source identifies the origin of a usage entry.
type Source string

const (
	// SourceTask indicates the entry was recorded for a task execution.
	SourceTask Source = "task"
	// SourceWorkflow indicates the entry aggregates workflow-level totals.
	SourceWorkflow Source = "workflow"
)

// Entry groups token usage metrics for a specific provider/model pair.
type Entry struct {
	Provider           string     `json:"provider"`
	Model              string     `json:"model"`
	PromptTokens       int        `json:"prompt_tokens"`
	CompletionTokens   int        `json:"completion_tokens"`
	TotalTokens        int        `json:"total_tokens"`
	ReasoningTokens    *int       `json:"reasoning_tokens,omitempty"`
	CachedPromptTokens *int       `json:"cached_prompt_tokens,omitempty"`
	InputAudioTokens   *int       `json:"input_audio_tokens,omitempty"`
	OutputAudioTokens  *int       `json:"output_audio_tokens,omitempty"`
	AgentIDs           []string   `json:"agent_ids,omitempty"`
	CapturedAt         *time.Time `json:"captured_at,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
	Source             string     `json:"source,omitempty"`
}

// Summary aggregates usage entries for an execution.
type Summary struct {
	Entries []Entry `json:"entries"`
}

// MarshalJSON encodes the summary as a JSON array for compact storage.
func (s *Summary) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	return json.Marshal(s.Entries)
}

// UnmarshalJSON decodes either a raw array or a wrapped object into the summary.
func (s *Summary) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		s.Entries = nil
		return nil
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err == nil {
		s.Entries = entries
		return nil
	}
	var wrapper struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	s.Entries = wrapper.Entries
	return nil
}

// Clone returns a deep copy of the summary.
func (s *Summary) Clone() *Summary {
	if s == nil {
		return nil
	}
	entries := make([]Entry, len(s.Entries))
	for i := range s.Entries {
		entry := s.Entries[i]
		if len(entry.AgentIDs) > 0 {
			ids := make([]string, len(entry.AgentIDs))
			copy(ids, entry.AgentIDs)
			entry.AgentIDs = ids
		}
		entries[i] = entry
	}
	return &Summary{Entries: entries}
}

// CloneWithSource returns a deep copy tagged with the provided source value.
func (s *Summary) CloneWithSource(source Source) *Summary {
	clone := s.Clone()
	if clone == nil {
		return nil
	}
	for i := range clone.Entries {
		clone.Entries[i].Source = string(source)
	}
	return clone
}

// MergeEntry merges the given entry into the summary using provider+model as the key.
func (s *Summary) MergeEntry(entry *Entry) {
	if entry == nil {
		return
	}
	entry.normalize()
	for i := range s.Entries {
		if s.Entries[i].Provider == entry.Provider && s.Entries[i].Model == entry.Model &&
			sameSource(s.Entries[i].Source, entry.Source) {
			mergeEntries(&s.Entries[i], entry)
			return
		}
	}
	s.Entries = append(s.Entries, *entry)
}

// MergeAll merges all entries from the other summary.
func (s *Summary) MergeAll(other *Summary) {
	if other == nil {
		return
	}
	for i := range other.Entries {
		s.MergeEntry(&other.Entries[i])
	}
}

// Sort canonicalises entry ordering for deterministic output.
func (s *Summary) Sort() {
	if s == nil {
		return
	}
	sort.Slice(s.Entries, func(i, j int) bool {
		left := s.Entries[i]
		right := s.Entries[j]
		if left.Provider == right.Provider {
			if left.Model == right.Model {
				return left.Source < right.Source
			}
			return left.Model < right.Model
		}
		return left.Provider < right.Provider
	})
}

// Validate ensures all summary entries contain the required fields and sane values.
func (s *Summary) Validate() error {
	if s == nil {
		return nil
	}
	for i := range s.Entries {
		if err := s.Entries[i].validate(); err != nil {
			return fmt.Errorf("usage entry %d: %w", i, err)
		}
	}
	return nil
}

func (e *Entry) normalize() {
	if e.TotalTokens == 0 {
		e.TotalTokens = e.PromptTokens + e.CompletionTokens
	}
	if len(e.AgentIDs) > 1 {
		ids := slices.Clone(e.AgentIDs)
		for i := range ids {
			ids[i] = strings.TrimSpace(ids[i])
		}
		slices.Sort(ids)
		ids = slices.Compact(ids)
		e.AgentIDs = ids
	}
}

func mergeEntries(base, delta *Entry) {
	base.PromptTokens += delta.PromptTokens
	base.CompletionTokens += delta.CompletionTokens
	if delta.TotalTokens > 0 {
		base.TotalTokens += delta.TotalTokens
	} else {
		base.TotalTokens = base.PromptTokens + base.CompletionTokens
	}
	base.ReasoningTokens = mergeOptionalInt(base.ReasoningTokens, delta.ReasoningTokens)
	base.CachedPromptTokens = mergeOptionalInt(base.CachedPromptTokens, delta.CachedPromptTokens)
	base.InputAudioTokens = mergeOptionalInt(base.InputAudioTokens, delta.InputAudioTokens)
	base.OutputAudioTokens = mergeOptionalInt(base.OutputAudioTokens, delta.OutputAudioTokens)
	base.AgentIDs = mergeAgentIDs(base.AgentIDs, delta.AgentIDs)
	base.CapturedAt = pickEarliest(base.CapturedAt, delta.CapturedAt)
	base.UpdatedAt = pickLatest(base.UpdatedAt, delta.UpdatedAt)
	if base.Source == "" {
		base.Source = delta.Source
	}
}

func mergeOptionalInt(a, b *int) *int {
	if a == nil && b == nil {
		return nil
	}
	sum := 0
	if a != nil {
		sum += *a
	}
	if b != nil {
		sum += *b
	}
	return &sum
}

func mergeAgentIDs(current, incoming []string) []string {
	if len(current) == 0 {
		return slices.Clone(incoming)
	}
	if len(incoming) == 0 {
		return current
	}
	merged := append(slices.Clone(current), incoming...)
	for i := range merged {
		merged[i] = strings.TrimSpace(merged[i])
	}
	slices.Sort(merged)
	merged = slices.Compact(merged)
	return merged
}

func (e *Entry) validate() error {
	if e == nil {
		return fmt.Errorf("entry is nil")
	}
	e.Provider = strings.TrimSpace(e.Provider)
	if e.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	e.Model = strings.TrimSpace(e.Model)
	if e.Model == "" {
		return fmt.Errorf("model is required")
	}
	if e.PromptTokens < 0 {
		return fmt.Errorf("prompt_tokens must be non-negative")
	}
	if e.CompletionTokens < 0 {
		return fmt.Errorf("completion_tokens must be non-negative")
	}
	if e.TotalTokens < 0 {
		return fmt.Errorf("total_tokens must be non-negative")
	}
	if err := validateOptionalInt(e.ReasoningTokens, "reasoning_tokens"); err != nil {
		return err
	}
	if err := validateOptionalInt(e.CachedPromptTokens, "cached_prompt_tokens"); err != nil {
		return err
	}
	if err := validateOptionalInt(e.InputAudioTokens, "input_audio_tokens"); err != nil {
		return err
	}
	if err := validateOptionalInt(e.OutputAudioTokens, "output_audio_tokens"); err != nil {
		return err
	}
	if e.Source != "" {
		e.Source = strings.TrimSpace(e.Source)
	}
	e.normalize()
	return nil
}

func validateOptionalInt(value *int, field string) error {
	if value == nil {
		return nil
	}
	if *value < 0 {
		return fmt.Errorf("%s must be non-negative", field)
	}
	return nil
}

func pickEarliest(a, b *time.Time) *time.Time {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if b.Before(*a) {
		return b
	}
	return a
}

func pickLatest(a, b *time.Time) *time.Time {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if b.After(*a) {
		return b
	}
	return a
}

func sameSource(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// Metrics captures observability hooks for usage collection so callers can emit counters
// without depending on concrete monitoring implementations.
type Metrics interface {
	RecordSuccess(
		ctx context.Context,
		component core.ComponentType,
		provider string,
		model string,
		promptTokens int,
		completionTokens int,
		latency time.Duration,
	)
	RecordFailure(
		ctx context.Context,
		component core.ComponentType,
		provider string,
		model string,
		latency time.Duration,
	)
}
