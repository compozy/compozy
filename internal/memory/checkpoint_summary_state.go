package memory

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/goccy/go-yaml"
)

const (
	checkpointCoverageVersion    = 1
	checkpointCoverageRangeLimit = checkpointSummarySourceLimit
	checkpointCoveragePrefix     = "<!-- agh:checkpoint-compaction:v1 "
	checkpointCoverageSuffix     = " -->"
)

type checkpointSummaryState struct {
	body     string
	header   *memcontract.Header
	coverage checkpointCoverage
}

type checkpointCoverage struct {
	Version       int                       `json:"version"`
	Ranges        []checkpointCoverageRange `json:"ranges,omitempty"`
	ResumeSummary string                    `json:"resume_summary,omitempty"`
}

type checkpointCoverageRange struct {
	WorkspaceID  string `json:"workspace_id"`
	SessionID    string `json:"session_id"`
	FromSequence int64  `json:"from_sequence"`
	ToSequence   int64  `json:"to_sequence"`
}

func loadCheckpointSummaryState(store *Store) (checkpointSummaryState, error) {
	content, err := store.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return checkpointSummaryState{}, nil
		}
		return checkpointSummaryState{}, fmt.Errorf("memory: read prior checkpoint summary: %w", err)
	}
	state, err := parseCheckpointSummaryState(content)
	if err != nil {
		return checkpointSummaryState{}, fmt.Errorf("memory: parse prior checkpoint summary: %w", err)
	}
	return state, nil
}

func parseCheckpointSummaryState(content []byte) (checkpointSummaryState, error) {
	var header memcontract.Header
	body, err := parseFrontmatter(content, &header)
	if err != nil {
		return checkpointSummaryState{}, err
	}
	if err := header.Validate(); err != nil {
		return checkpointSummaryState{}, fmt.Errorf("validate checkpoint frontmatter: %w", err)
	}
	visible, coverage, err := splitCheckpointCoverage(body)
	if err != nil {
		return checkpointSummaryState{}, err
	}
	if visible != "" {
		if err := validateCheckpointSummary(visible); err != nil {
			return checkpointSummaryState{}, fmt.Errorf("validate checkpoint body: %w", err)
		}
	} else if len(coverage.Ranges) == 0 {
		return checkpointSummaryState{}, errors.New("checkpoint body is empty")
	}
	return checkpointSummaryState{body: visible, header: &header, coverage: coverage}, nil
}

func renderCheckpointSummaryState(state checkpointSummaryState) ([]byte, error) {
	if state.header == nil {
		return nil, errors.New("memory: checkpoint summary header is required")
	}
	metadata, err := yaml.Marshal(*state.header)
	if err != nil {
		return nil, fmt.Errorf("memory: render checkpoint frontmatter: %w", err)
	}
	sections := make([]string, 0, 2)
	if body := strings.TrimSpace(state.body); body != "" {
		sections = append(sections, body)
	}
	marker, err := renderCheckpointCoverage(state.coverage)
	if err != nil {
		return nil, err
	}
	if marker != "" {
		sections = append(sections, marker)
	}
	return []byte("---\n" + string(metadata) + "---\n\n" + strings.Join(sections, "\n\n") + "\n"), nil
}

func splitCheckpointCoverage(body string) (string, checkpointCoverage, error) {
	trimmed := strings.TrimSpace(body)
	index := strings.LastIndex(trimmed, checkpointCoveragePrefix)
	if index < 0 {
		return trimmed, checkpointCoverage{}, nil
	}
	marker := strings.TrimSpace(trimmed[index:])
	if !strings.HasSuffix(marker, checkpointCoverageSuffix) {
		return "", checkpointCoverage{}, errors.New("checkpoint compaction coverage marker is malformed")
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(marker, checkpointCoveragePrefix), checkpointCoverageSuffix)
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", checkpointCoverage{}, fmt.Errorf("decode checkpoint compaction coverage: %w", err)
	}
	var coverage checkpointCoverage
	if err := json.Unmarshal(data, &coverage); err != nil {
		return "", checkpointCoverage{}, fmt.Errorf("parse checkpoint compaction coverage: %w", err)
	}
	if err := coverage.validate(); err != nil {
		return "", checkpointCoverage{}, err
	}
	return strings.TrimSpace(trimmed[:index]), coverage, nil
}

func renderCheckpointCoverage(coverage checkpointCoverage) (string, error) {
	if len(coverage.Ranges) == 0 {
		return "", nil
	}
	coverage.Version = checkpointCoverageVersion
	if err := coverage.validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(coverage)
	if err != nil {
		return "", fmt.Errorf("memory: render checkpoint compaction coverage: %w", err)
	}
	return checkpointCoveragePrefix + base64.RawURLEncoding.EncodeToString(data) + checkpointCoverageSuffix, nil
}

func (c checkpointCoverage) validate() error {
	if c.Version != checkpointCoverageVersion {
		return fmt.Errorf("memory: unsupported checkpoint compaction coverage version %d", c.Version)
	}
	if len(c.Ranges) == 0 {
		return errors.New("memory: checkpoint compaction coverage has no ranges")
	}
	if err := validateCheckpointSummary(c.ResumeSummary); err != nil {
		return fmt.Errorf("memory: validate checkpoint compaction resume summary: %w", err)
	}
	for _, item := range c.Ranges {
		if strings.TrimSpace(item.WorkspaceID) == "" || strings.TrimSpace(item.SessionID) == "" {
			return errors.New("memory: checkpoint compaction coverage identity is required")
		}
		if item.FromSequence <= 0 || item.ToSequence < item.FromSequence {
			return fmt.Errorf(
				"memory: invalid checkpoint compaction coverage range %d..%d",
				item.FromSequence,
				item.ToSequence,
			)
		}
	}
	return nil
}

func (c checkpointCoverage) covers(workspaceID string, sessionID string, fromSequence int64, toSequence int64) bool {
	for _, item := range c.Ranges {
		if item.WorkspaceID == strings.TrimSpace(workspaceID) &&
			item.SessionID == strings.TrimSpace(sessionID) &&
			item.FromSequence <= fromSequence && item.ToSequence >= toSequence {
			return true
		}
	}
	return false
}

func (c checkpointCoverage) hasSession(sessionID string) bool {
	target := strings.TrimSpace(sessionID)
	for _, item := range c.Ranges {
		if item.SessionID == target {
			return true
		}
	}
	return false
}

func (c checkpointCoverage) withRange(item checkpointCoverageRange, summary string) checkpointCoverage {
	updated := checkpointCoverage{
		Version:       checkpointCoverageVersion,
		Ranges:        append([]checkpointCoverageRange(nil), c.Ranges...),
		ResumeSummary: strings.TrimSpace(summary),
	}
	updated.Ranges = append(updated.Ranges, item)
	sort.Slice(updated.Ranges, func(i int, j int) bool {
		left, right := updated.Ranges[i], updated.Ranges[j]
		if left.WorkspaceID != right.WorkspaceID {
			return left.WorkspaceID < right.WorkspaceID
		}
		if left.SessionID != right.SessionID {
			return left.SessionID < right.SessionID
		}
		return left.FromSequence < right.FromSequence
	})
	merged := make([]checkpointCoverageRange, 0, len(updated.Ranges))
	for _, candidate := range updated.Ranges {
		if len(merged) == 0 {
			merged = append(merged, candidate)
			continue
		}
		last := &merged[len(merged)-1]
		if last.WorkspaceID == candidate.WorkspaceID && last.SessionID == candidate.SessionID &&
			candidate.FromSequence <= last.ToSequence+1 {
			last.ToSequence = max(last.ToSequence, candidate.ToSequence)
			continue
		}
		merged = append(merged, candidate)
	}
	updated.Ranges = boundCheckpointCoverageRanges(merged, item)
	return updated
}

func boundCheckpointCoverageRanges(
	ranges []checkpointCoverageRange,
	latest checkpointCoverageRange,
) []checkpointCoverageRange {
	if len(ranges) <= checkpointCoverageRangeLimit {
		return ranges
	}
	latestIndex := -1
	for index, candidate := range ranges {
		if candidate.WorkspaceID == latest.WorkspaceID && candidate.SessionID == latest.SessionID &&
			candidate.FromSequence <= latest.FromSequence && candidate.ToSequence >= latest.ToSequence {
			latestIndex = index
			break
		}
	}
	drop := len(ranges) - checkpointCoverageRangeLimit
	bounded := make([]checkpointCoverageRange, 0, checkpointCoverageRangeLimit)
	for index, candidate := range ranges {
		if drop > 0 && index != latestIndex {
			drop--
			continue
		}
		bounded = append(bounded, candidate)
	}
	return bounded
}

func renderCheckpointSummaryResumeSection(state checkpointSummaryState, sessionID string) string {
	body := strings.TrimSpace(state.body)
	fallback := strings.TrimSpace(state.coverage.ResumeSummary)
	if state.coverage.hasSession(sessionID) && fallback != "" && fallback != body {
		if body == "" {
			return renderCheckpointSummarySection(fallback)
		}
		return joinAssemblerSections(
			renderCheckpointSummarySection(body),
			"# Archived Session Continuity\n\n"+
				"This checkpoint fallback covers persisted events excluded from replay.\n\n"+
				"<agh_checkpoint_archive_coverage>\n\n"+fallback+"\n\n</agh_checkpoint_archive_coverage>",
		)
	}
	return renderCheckpointSummarySection(body)
}
