package controller

import (
	"fmt"
	"path/filepath"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/memory/scan"
)

func exactContentTarget(candidate memcontract.Candidate, targets []Target) *Target {
	candidateBody := canonicalBody(candidate.Content)
	for idx := range targets {
		if canonicalBody(targets[idx].Content) == candidateBody {
			return &targets[idx]
		}
	}
	return nil
}

func exactFilenameTarget(candidate memcontract.Candidate, targets []Target) *Target {
	filename := targetFilename(candidate)
	for idx := range targets {
		if targets[idx].TargetFilename == filename {
			return &targets[idx]
		}
	}
	return nil
}

func entitySlotTargets(candidate memcontract.Candidate, targets []Target) []Target {
	entity := normalizeSlot(firstNonEmpty(candidate.Entity, metadataValue(candidate.Metadata, metadataTargetEntityKey)))
	attribute := normalizeSlot(
		firstNonEmpty(candidate.Attribute, metadataValue(candidate.Metadata, metadataTargetAttributeKey)),
	)
	if entity == "" || attribute == "" {
		return nil
	}
	matches := make([]Target, 0)
	for _, target := range targets {
		if normalizeSlot(target.Entity) == entity && normalizeSlot(target.Attribute) == attribute {
			matches = append(matches, target)
		}
	}
	return matches
}

func surfaceTargets(candidate memcontract.Candidate, targets []Target) []Target {
	candidateTokens := tokenSet(candidate.Content)
	matches := make([]Target, 0)
	for _, target := range targets {
		overlap := 0
		for token := range tokenSet(target.Content) {
			if _, exists := candidateTokens[token]; exists {
				overlap++
			}
		}
		if overlap >= minSurfaceOverlap {
			matches = append(matches, target)
		}
	}
	return matches
}

func targetFilename(candidate memcontract.Candidate) string {
	for _, key := range []string{metadataTargetFilenameKey, metadataFilenameKey} {
		if filename := cleanFilename(metadataValue(candidate.Metadata, key)); filename != "" {
			return filename
		}
	}
	if filename := cleanFilename(candidate.Frontmatter.Filename); filename != "" {
		return filename
	}
	base := firstNonEmpty(candidate.Entity, candidate.Frontmatter.Name, firstWords(candidate.Content, 5), "memory")
	prefix := string(candidate.Frontmatter.Type.Normalize())
	if prefix == "" {
		prefix = "memory"
	}
	return prefix + "_" + slugify(base) + ".md"
}

func isAutonomousWriteOrigin(origin memcontract.Origin) bool {
	switch origin.Normalize() {
	case memcontract.OriginDreaming, memcontract.OriginExtractor, memcontract.OriginProvider:
		return true
	default:
		return false
	}
}

func hasExplicitTargetFilename(candidate memcontract.Candidate) bool {
	for _, key := range []string{metadataTargetFilenameKey, metadataFilenameKey} {
		if cleanFilename(metadataValue(candidate.Metadata, key)) != "" {
			return true
		}
	}
	return cleanFilename(candidate.Frontmatter.Filename) != ""
}

func directTargetIdentifiesNewMemory(candidate memcontract.Candidate, targets []Target) bool {
	hasFilename := hasExplicitTargetFilename(candidate)
	candidateName := normalizeSlot(candidate.Frontmatter.Name)
	if !hasFilename && candidateName == "" {
		return false
	}
	if candidateName == "" {
		return true
	}
	for _, target := range targets {
		if normalizeSlot(target.Frontmatter.Name) == candidateName {
			return false
		}
	}
	return true
}

func cleanFilename(filename string) string {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return ""
	}
	if strings.ContainsAny(trimmed, `/\`) {
		return ""
	}
	if filepath.Ext(trimmed) == "" {
		trimmed += ".md"
	}
	return trimmed
}

func scanRuleHits(result scan.Result, candidate memcontract.Candidate) []memcontract.RuleHit {
	hits := result.RuleHits()
	if len(hits) == 0 {
		return nil
	}
	sampleBytes := len([]byte(candidate.Content))
	for idx := range hits {
		hits[idx].Details = strings.TrimSpace(hits[idx].Details + fmt.Sprintf(" sample_bytes=%d", sampleBytes))
	}
	return hits
}

func scanReason(result scan.Result, candidate memcontract.Candidate) string {
	return fmt.Sprintf("%s sample_bytes=%d", result.Reason(), len([]byte(candidate.Content)))
}
