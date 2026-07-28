package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"strings"
)

// CanonicalCapabilityDigest computes the runtime-owned digest for one
// capability document after normalization.
func CanonicalCapabilityDigest(capability CapabilityDef) (string, error) {
	return computeCapabilityDigest(normalizeCapabilityDef(cloneCapabilityDef(capability)))
}

func computeCapabilityDigest(capability CapabilityDef) (string, error) {
	payload, err := json.Marshal(canonicalCapabilityDigestPayload{
		ID:                capability.ID,
		Summary:           capability.Summary,
		Outcome:           capability.Outcome,
		Version:           capability.Version,
		ContextNeeded:     append([]string(nil), capability.ContextNeeded...),
		ArtifactsExpected: append([]string(nil), capability.ArtifactsExpected...),
		ExecutionOutline:  append([]string(nil), capability.ExecutionOutline...),
		Constraints:       append([]string(nil), capability.Constraints...),
		Examples:          append([]string(nil), capability.Examples...),
		Requirements:      canonicalizeCapabilityRequirements(capability.Requirements),
	})
	if err != nil {
		return "", fmt.Errorf("marshal canonical capability: %w", err)
	}

	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func cloneCapabilityDef(capability CapabilityDef) CapabilityDef {
	return CapabilityDef{
		ID:                capability.ID,
		Summary:           capability.Summary,
		Outcome:           capability.Outcome,
		Version:           capability.Version,
		ContextNeeded:     append([]string(nil), capability.ContextNeeded...),
		ArtifactsExpected: append([]string(nil), capability.ArtifactsExpected...),
		ExecutionOutline:  append([]string(nil), capability.ExecutionOutline...),
		Constraints:       append([]string(nil), capability.Constraints...),
		Examples:          append([]string(nil), capability.Examples...),
		Requirements:      append([]string(nil), capability.Requirements...),
		Digest:            capability.Digest,
	}
}

func joinQuotedPaths(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(paths))
	for _, path := range paths {
		quoted = append(quoted, fmt.Sprintf("%q", path))
	}
	return strings.Join(quoted, ", ")
}

func ensureJSONDocumentEOF(decoder *json.Decoder, source string, label string) error {
	if decoder == nil {
		return errors.New("config: JSON decoder is required")
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("config: decode %s %q: %w", label, source, err)
	}

	return fmt.Errorf("config: decode %s %q: unexpected trailing JSON value", label, source)
}
