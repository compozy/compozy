package session

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

const (
	maxFileMutationTargetsPerCall = 32
	maxFileMutationEvidencePaths  = 32
)

var fileMutationPatchTargetPattern = regexp.MustCompile(
	`(?m)^\*\*\* (?:Update|Add|Delete) File:\s*(.+?)\s*$`,
)

type fileMutationVerifier struct {
	calls              map[string]fileMutationCall
	unresolved         map[string]string
	assistantPersisted bool
}

type fileMutationCall struct {
	kind    string
	targets []string
}

// Observe reduces one event only after its durable record succeeds.
func (v *fileMutationVerifier) Observe(event acp.AgentEvent) {
	if v == nil {
		return
	}
	if event.Type == acp.EventTypeAgentMessage && strings.TrimSpace(event.Text) != "" {
		v.assistantPersisted = true
	}
	callID := strings.TrimSpace(event.ToolCallID)
	switch event.Type {
	case acp.EventTypeToolCall:
		if callID == "" {
			return
		}
		if v.calls == nil {
			v.calls = make(map[string]fileMutationCall)
		}
		call := v.calls[callID]
		if kind := strings.TrimSpace(fileMutationToolKind(event)); kind != "" {
			call.kind = kind
		}
		if targets := fileMutationTargets(event.ToolInput()); len(targets) > 0 {
			call.targets = targets
		}
		v.calls[callID] = call
	case acp.EventTypeToolResult:
		call := v.calls[callID]
		delete(v.calls, callID)
		kind := firstNonEmpty(fileMutationToolKind(event), call.kind)
		if !strings.EqualFold(strings.TrimSpace(kind), "edit") {
			return
		}
		targets := call.targets
		if len(targets) == 0 {
			targets = fileMutationTargets(event.ToolInput())
		}
		if len(targets) == 0 {
			return
		}
		if event.ToolError() {
			if v.unresolved == nil {
				v.unresolved = make(map[string]string)
			}
			detail := firstNonEmpty(event.ToolErrorDetail(), event.Error, event.Text, "file mutation failed")
			for _, target := range targets {
				if _, exists := v.unresolved[target]; !exists {
					v.unresolved[target] = detail
				}
			}
			return
		}
		for _, target := range targets {
			delete(v.unresolved, target)
		}
	}
}

func (v *fileMutationVerifier) Marker() (string, map[string]any, bool) {
	if v == nil || !v.assistantPersisted || len(v.unresolved) == 0 {
		return "", nil, false
	}
	targets := make([]string, 0, len(v.unresolved))
	for target := range v.unresolved {
		targets = append(targets, target)
	}
	slices.Sort(targets)
	count := len(targets)
	evidence := map[string]any{"failure_count": count}
	if len(targets) > maxFileMutationEvidencePaths {
		targets = targets[:maxFileMutationEvidencePaths]
		evidence["paths_truncated"] = true
	}
	evidence["paths"] = targets
	const verificationGuidance = " Verify the affected files before trusting completion claims."
	summary := fmt.Sprintf("%d file mutations failed and were not recovered in this turn.", count) +
		verificationGuidance
	if count == 1 {
		summary = "1 file mutation failed and was not recovered in this turn. " +
			"Verify the affected file before trusting completion claims."
	}
	return summary, evidence, true
}

func fileMutationToolKind(event acp.AgentEvent) string {
	if kind := strings.TrimSpace(event.ToolKind()); kind != "" {
		return kind
	}
	var raw struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(event.Raw, &raw); err != nil {
		return ""
	}
	return strings.TrimSpace(raw.Kind)
}

func fileMutationTargets(input json.RawMessage) []string {
	if len(input) == 0 {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return nil
	}
	targets := make([]string, 0, 4)
	for _, key := range []string{"path", "file_path"} {
		var value string
		if err := json.Unmarshal(fields[key], &value); err == nil {
			targets = appendFileMutationTarget(targets, value)
		}
	}
	for _, key := range []string{"patch", "patch_text"} {
		var patch string
		if err := json.Unmarshal(fields[key], &patch); err != nil {
			continue
		}
		for _, match := range fileMutationPatchTargetPattern.FindAllStringSubmatch(
			patch,
			maxFileMutationTargetsPerCall,
		) {
			if len(match) > 1 {
				targets = appendFileMutationTarget(targets, match[1])
			}
		}
	}
	return targets
}

func appendFileMutationTarget(targets []string, candidate string) []string {
	if len(targets) >= maxFileMutationTargetsPerCall {
		return targets
	}
	target := strings.TrimSpace(candidate)
	if before, after, ok := strings.Cut(target, " -> "); ok {
		targets = appendFileMutationTarget(targets, before)
		target = after
	}
	if target == "" {
		return targets
	}
	target = filepath.Clean(target)
	if slices.Contains(targets, target) {
		return targets
	}
	return append(targets, target)
}
