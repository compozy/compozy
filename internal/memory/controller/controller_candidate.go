package controller

import (
	"encoding/json"
	"errors"
	"fmt"

	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/frontmatter"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	yaml "github.com/goccy/go-yaml"
)

func normalizeCandidate(candidate memcontract.Candidate, now time.Time) (memcontract.Candidate, error) {
	candidate.WorkspaceID = strings.TrimSpace(candidate.WorkspaceID)
	candidate.Scope = candidate.Scope.Normalize()
	candidate.AgentName = strings.TrimSpace(firstNonEmpty(candidate.AgentName, candidate.Frontmatter.AgentName))
	candidate.AgentTier = firstNonEmptyAgentTier(candidate.AgentTier, candidate.Frontmatter.AgentTier).Normalize()
	candidate.Origin = candidate.Origin.Normalize()
	candidate.Content = strings.TrimSpace(candidate.Content)
	candidate.Entity = normalizeSlot(candidate.Entity)
	candidate.Attribute = normalizeSlot(candidate.Attribute)
	metadata := make(map[string]string, len(candidate.Metadata))
	for key, value := range candidate.Metadata {
		if key == metadataRawContentKey {
			return memcontract.Candidate{}, errRawContentMetadata
		}
		metadata[key] = value
	}
	candidate.Metadata = metadata
	if candidate.SubmittedAt.IsZero() {
		candidate.SubmittedAt = now.UTC()
	}
	if candidate.Origin == "" {
		candidate.Origin = memcontract.OriginFile
	}
	if err := candidate.Origin.Validate(); err != nil {
		return memcontract.Candidate{}, fmt.Errorf("memory controller: candidate origin: %w", err)
	}
	if candidate.Scope == "" {
		candidate.Scope = candidate.Frontmatter.Scope.Normalize()
	}
	if candidate.Scope == "" && candidate.Frontmatter.Type.Normalize() != "" {
		scope, err := memcontract.DefaultScopeForType(candidate.Frontmatter.Type)
		if err != nil {
			return memcontract.Candidate{}, fmt.Errorf("memory controller: infer candidate scope: %w", err)
		}
		candidate.Scope = scope
	}
	if err := candidate.Scope.Validate(); err != nil {
		return memcontract.Candidate{}, fmt.Errorf("memory controller: candidate scope: %w", err)
	}
	if opFromCandidate(candidate) != memcontract.OpDelete {
		candidate.Frontmatter.Scope = candidate.Scope.Normalize()
		if candidate.AgentName != "" {
			candidate.Frontmatter.AgentName = candidate.AgentName
		}
		if candidate.AgentTier != "" {
			candidate.Frontmatter.AgentTier = candidate.AgentTier
		}
		if err := candidate.Frontmatter.Validate(); err != nil {
			return memcontract.Candidate{}, fmt.Errorf("memory controller: candidate frontmatter: %w", err)
		}
		if candidate.Content == "" {
			return memcontract.Candidate{}, errors.New("memory controller: candidate content is required")
		}
	} else if targetFilename(candidate) == "" {
		return memcontract.Candidate{}, errors.New("memory controller: delete candidate target filename is required")
	}
	return candidate, nil
}

// CandidateHash returns the stable hash used for decision audit rows.
func CandidateHash(candidate memcontract.Candidate) string {
	payload := map[string]any{
		"workspace_id": candidate.WorkspaceID,
		"scope":        candidate.Scope.Normalize(),
		"agent_name":   strings.TrimSpace(candidate.AgentName),
		"agent_tier":   candidate.AgentTier.Normalize(),
		"origin":       candidate.Origin.Normalize(),
		"content":      strings.TrimSpace(candidate.Content),
		"frontmatter":  candidate.Frontmatter,
		"entity":       normalizeSlot(candidate.Entity),
		"attribute":    normalizeSlot(candidate.Attribute),
		"metadata":     sortedMetadata(candidate.Metadata),
	}
	if strings.TrimSpace(candidate.TrustedRawContent) != "" {
		payload["trusted_raw_hash"] = hashString(candidate.TrustedRawContent)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return hashString(fmt.Sprintf("%#v", payload))
	}
	return hashString(string(encoded))
}

// FrontmatterHash returns a stable hash for a decision's frontmatter material.
func FrontmatterHash(header memcontract.Header) string {
	encoded, err := json.Marshal(header)
	if err != nil {
		return hashString(fmt.Sprintf("%#v", header))
	}
	return hashString(string(encoded))
}

// IdempotencyKey returns the write-ahead-log uniqueness key for a decision.
func IdempotencyKey(decision memcontract.Decision) string {
	parts := []string{
		decision.CandidateHash,
		decision.Op.String(),
		strings.Join(decision.Targets, ","),
		decision.TargetFilename,
		decision.PostContentHash,
		FrontmatterHash(decision.Frontmatter),
		decision.PromptVersion,
	}
	return hashString(strings.Join(parts, "\x00"))
}

func postContentForCandidate(candidate memcontract.Candidate) (string, error) {
	if strings.TrimSpace(candidate.TrustedRawContent) != "" {
		if err := validateTrustedRawContent(candidate); err != nil {
			return "", err
		}
		return candidate.TrustedRawContent, nil
	}
	metadata, err := yaml.Marshal(candidate.Frontmatter)
	if err != nil {
		return "", fmt.Errorf("memory controller: render frontmatter: %w", err)
	}
	return "---\n" + string(metadata) + "---\n\n" + strings.TrimSpace(candidate.Content) + "\n", nil
}

func validateTrustedRawContent(candidate memcontract.Candidate) error {
	var rawHeader memcontract.Header
	body, err := frontmatter.Decode([]byte(candidate.TrustedRawContent), func(data []byte) error {
		if err := yaml.UnmarshalWithOptions(data, &rawHeader, yaml.Strict()); err != nil {
			return fmt.Errorf("decode YAML: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("memory controller: parse trusted raw content: %w", err)
	}
	if canonicalBody(body) != canonicalBody(candidate.Content) {
		return errors.New("memory controller: trusted raw content body differs from candidate content")
	}
	if !equivalentTrustedRawHeader(rawHeader, candidate.Frontmatter) {
		return errors.New("memory controller: trusted raw content frontmatter differs from candidate frontmatter")
	}
	return nil
}

func equivalentTrustedRawHeader(rawHeader memcontract.Header, expected memcontract.Header) bool {
	rawHeader.Normalize()
	expected.Normalize()
	rawHeader.Filename = expected.Filename
	rawHeader.FilePath = expected.FilePath
	rawHeader.ModTime = expected.ModTime
	if rawHeader.Scope == "" {
		rawHeader.Scope = expected.Scope
	}
	if rawHeader.AgentName == "" {
		rawHeader.AgentName = expected.AgentName
	}
	if rawHeader.AgentTier == "" {
		rawHeader.AgentTier = expected.AgentTier
	}
	return rawHeader.Name == expected.Name &&
		rawHeader.Description == expected.Description &&
		rawHeader.Type.Normalize() == expected.Type.Normalize() &&
		rawHeader.Scope.Normalize() == expected.Scope.Normalize() &&
		strings.TrimSpace(rawHeader.AgentName) == strings.TrimSpace(expected.AgentName) &&
		rawHeader.AgentTier.Normalize() == expected.AgentTier.Normalize() &&
		equivalentProvenance(rawHeader.Provenance, expected.Provenance)
}

func equivalentProvenance(left *memcontract.Provenance, right *memcontract.Provenance) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return slices.Equal(left.SourceSessionIDs, right.SourceSessionIDs) &&
		left.SourceActor.Normalize() == right.SourceActor.Normalize() &&
		strings.TrimSpace(left.Confidence) == strings.TrimSpace(right.Confidence) &&
		strings.TrimSpace(left.SupersededBy) == strings.TrimSpace(right.SupersededBy) &&
		left.CreatedAt.Equal(right.CreatedAt) &&
		left.UpdatedAt.Equal(right.UpdatedAt)
}
