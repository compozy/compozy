package daemon

import (
	"bufio"
	"bytes"
	"context"

	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/memory/prompts"
)

func renderMemoryExtractorPrompt(turn memcontract.TurnRecord) (string, error) {
	tmpl, err := prompts.ParseTemplate(prompts.NameExtract, prompts.VersionV1)
	if err != nil {
		return "", err
	}
	policy, err := prompts.Load(prompts.NameWhatNotToSave, prompts.VersionV1)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	data := map[string]any{
		"WhatNotToSave": policy.Content,
		"Turn":          turn,
		"Transcript":    renderMemoryTranscript(turn.Snapshot),
	}
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("daemon: render memory extractor prompt: %w", err)
	}
	return rendered.String(), nil
}

func renderMemoryTranscript(snapshot memcontract.TranscriptSnapshot) string {
	var buf strings.Builder
	for _, message := range snapshot.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = daemonUnknownValue
		}
		if _, err := fmt.Fprintf(
			&buf,
			"- sequence=%d role=%s at=%s\n%s\n",
			message.Sequence,
			role,
			message.At.UTC().Format(time.RFC3339Nano),
			strings.TrimSpace(message.Content),
		); err != nil {
			return buf.String()
		}
	}
	return buf.String()
}

func memoryExtractorOverlay() string {
	return strings.TrimSpace(`
You are an AGH internal Memory v2 extractor child session.
Return only JSONL candidates that match the requested schema.
Do not modify files, run commands, or include commentary outside JSONL.
`)
}

func collectMemoryExtractorOutput(ctx context.Context, events <-chan acp.AgentEvent) (string, error) {
	var output strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("daemon: collect memory extractor output: %w", ctx.Err())
		case event, ok := <-events:
			if !ok {
				return output.String(), nil
			}
			switch event.Type {
			case acp.EventTypeAgentMessage:
				// Agent messages are stream chunks; inserting separators can corrupt JSONL.
				output.WriteString(event.Text)
			case acp.EventTypeError:
				return "", fmt.Errorf("daemon: memory extractor agent error: %s", strings.TrimSpace(event.Error))
			}
		}
	}
}

type extractedMemoryLine struct {
	Type      string `json:"type"`
	Scope     string `json:"scope"`
	AgentTier string `json:"agent_tier"`
	Content   string `json:"content"`
	Evidence  string `json:"evidence"`
	Entity    string `json:"entity"`
	Attribute string `json:"attribute"`
}

func parseMemoryExtractorCandidates(
	output string,
	turn memcontract.TurnRecord,
	workspaceRoot string,
	submittedAt time.Time,
) ([]memcontract.Candidate, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	candidates := make([]memcontract.Candidate, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := normalizeExtractorJSONLLine(scanner.Text())
		if raw == "" {
			continue
		}
		var line extractedMemoryLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			return nil, fmt.Errorf("daemon: decode memory extractor line %d: %w", lineNumber, err)
		}
		candidate, err := candidateFromExtractedLine(line, turn, workspaceRoot, submittedAt)
		if err != nil {
			return nil, fmt.Errorf("daemon: normalize memory extractor line %d: %w", lineNumber, err)
		}
		candidates = append(candidates, candidate)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("daemon: scan memory extractor output: %w", err)
	}
	return candidates, nil
}

func normalizeExtractorJSONLLine(raw string) string {
	line := strings.TrimSpace(raw)
	switch line {
	case "", "```", "```json", "```jsonl":
		return ""
	default:
		return line
	}
}

func candidateFromExtractedLine(
	line extractedMemoryLine,
	turn memcontract.TurnRecord,
	workspaceRoot string,
	submittedAt time.Time,
) (memcontract.Candidate, error) {
	memoryType := memcontract.Type(strings.TrimSpace(line.Type)).Normalize()
	if err := memoryType.Validate(); err != nil {
		return memcontract.Candidate{}, err
	}
	scope := memcontract.Scope(strings.TrimSpace(line.Scope)).Normalize()
	if scope == "" {
		defaultScope, err := memcontract.DefaultScopeForType(memoryType)
		if err != nil {
			return memcontract.Candidate{}, err
		}
		scope = defaultScope
	}
	if err := scope.Validate(); err != nil {
		return memcontract.Candidate{}, err
	}
	content := strings.TrimSpace(line.Content)
	if content == "" {
		return memcontract.Candidate{}, errors.New("content is required")
	}
	agentTier := memcontract.AgentTier(strings.TrimSpace(line.AgentTier)).Normalize()
	if scope == memcontract.ScopeAgent {
		if agentTier == "" {
			agentTier = memcontract.AgentTierWorkspace
		}
	} else {
		agentTier = ""
	}
	agentName := ""
	if scope == memcontract.ScopeAgent {
		agentName = strings.TrimSpace(turn.AgentID)
	}
	metadata := map[string]string{
		"evidence": line.Evidence,
	}
	if workspaceRoot != "" {
		metadata["workspace_root"] = workspaceRoot
	}
	return memcontract.Candidate{
		WorkspaceID: strings.TrimSpace(turn.WorkspaceID),
		Scope:       scope,
		AgentName:   agentName,
		AgentTier:   agentTier,
		Origin:      memcontract.OriginExtractor,
		Content:     content,
		Frontmatter: memcontract.Header{
			Name:        memoryCandidateName(line, content),
			Description: strings.TrimSpace(line.Evidence),
			Type:        memoryType,
			Scope:       scope,
			AgentName:   agentName,
			AgentTier:   agentTier,
			Provenance: &memcontract.Provenance{
				SourceSessionIDs: []string{turn.SessionID},
				SourceActor:      memcontract.OriginExtractor,
				Confidence:       "candidate",
				CreatedAt:        submittedAt.UTC(),
				UpdatedAt:        submittedAt.UTC(),
			},
		},
		Entity:      strings.TrimSpace(line.Entity),
		Attribute:   strings.TrimSpace(line.Attribute),
		Metadata:    metadata,
		SubmittedAt: submittedAt.UTC(),
	}, nil
}

func memoryCandidateName(line extractedMemoryLine, content string) string {
	entity := strings.TrimSpace(line.Entity)
	attribute := strings.TrimSpace(line.Attribute)
	switch {
	case entity != "" && attribute != "":
		return entity + " " + attribute
	case entity != "":
		return entity
	default:
		words := strings.Fields(content)
		if len(words) > 8 {
			words = words[:8]
		}
		return strings.Join(words, " ")
	}
}
