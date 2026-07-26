package controller

import (
	"crypto/sha256"
	"encoding/hex"

	"slices"
	"strings"

	"unicode"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func opFromCandidate(candidate memcontract.Candidate) memcontract.Op {
	raw := firstNonEmpty(
		metadataValue(candidate.Metadata, metadataOperationKey),
		metadataValue(candidate.Metadata, metadataOpKey),
	)
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case memcontract.OpDelete.String(), "forget", "remove":
		return memcontract.OpDelete
	default:
		return memcontract.OpAdd
	}
}

func passedRule(name string, reason string, target string) memcontract.RuleHit {
	return memcontract.RuleHit{Name: "controller." + name, Passed: true, Reason: reason, Target: target}
}

func failedRule(name string, reason string, targets []string) memcontract.RuleHit {
	return memcontract.RuleHit{
		Name:    "controller." + name,
		Passed:  false,
		Reason:  reason,
		Details: strings.Join(targets, ","),
	}
}

func targetIDs(targets []Target) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if id := strings.TrimSpace(target.ID); id != "" {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func targetSortKey(target Target) string {
	return strings.Join([]string{
		string(target.Scope.Normalize()),
		strings.TrimSpace(target.WorkspaceID),
		strings.TrimSpace(target.AgentName),
		string(target.AgentTier.Normalize()),
		strings.TrimSpace(target.TargetFilename),
	}, "\x00")
}

func equalMemoryBody(left string, right string) bool {
	return canonicalBody(left) == canonicalBody(right)
}

func canonicalBody(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func tokenSet(value string) map[string]struct{} {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if len(trimmed) < 3 {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}

func sortedMetadata(metadata map[string]string) [][2]string {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make([][2]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, [2]string{key, metadata[key]})
	}
	return out
}

func metadataValue(metadata map[string]string, key string) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(metadata[key])
}

func normalizeSlot(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func slugify(value string) string {
	trimmed := strings.TrimSpace(value)
	normalized := filenameUnsafePattern.ReplaceAllString(strings.ToLower(trimmed), "_")
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		if strings.EqualFold(trimmed, "memory") {
			return "memory"
		}
		return "memory_" + hashString(trimmed)[:12]
	}
	return normalized
}

func firstWords(value string, limit int) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	if len(fields) > limit {
		fields = fields[:limit]
	}
	return strings.Join(fields, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyAgentTier(values ...memcontract.AgentTier) memcontract.AgentTier {
	for _, value := range values {
		if normalized := value.Normalize(); normalized != "" {
			return normalized
		}
	}
	return ""
}

func confidenceForOp(op memcontract.Op) float32 {
	switch op {
	case memcontract.OpReject:
		return 1.0
	case memcontract.OpNoop:
		return 0.95
	default:
		return 0.9
	}
}

func boundRuleTrace(trace []memcontract.RuleHit) []memcontract.RuleHit {
	if len(trace) == 0 {
		return []memcontract.RuleHit{passedRule("default", "rule path completed", "")}
	}
	return trace
}

func boundString(value string, maxBytes int) string {
	trimmed := strings.TrimSpace(value)
	if maxBytes <= 0 || len(trimmed) <= maxBytes {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxBytes])
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
