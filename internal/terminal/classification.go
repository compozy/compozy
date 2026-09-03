package terminal

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// CommandVerdict describes whether parsed argv may bypass interactive approval.
type CommandVerdict string

const (
	CommandVerdictPrompt          CommandVerdict = "prompt"
	CommandVerdictAllowlisted     CommandVerdict = "allowlisted"
	CommandVerdictDenied          CommandVerdict = "denied"
	commandReasonApprovalRequired                = "approval_required"
	commandReasonIrreversible                    = "irreversible"
)

// ArgvPattern is one administrator-approved command shape. A "*" argument
// matches one argv element; the executable is always matched by base name.
type ArgvPattern []string

// CommandClassification is the deterministic result of classifying parsed argv.
type CommandClassification struct {
	Verdict CommandVerdict
	Reason  string
	Digest  string
}

// ClassifyArgv classifies parsed argv without performing an approval or reading state.
func ClassifyArgv(argv []string, allowlist []ArgvPattern, denyLists ...[]ArgvPattern) CommandClassification {
	digest := argvDigest(argv)
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return CommandClassification{Verdict: CommandVerdictPrompt, Reason: "empty_argv", Digest: digest}
	}
	if isBlockedIrreversibleArgv(argv) {
		return CommandClassification{Verdict: CommandVerdictDenied, Reason: commandReasonIrreversible, Digest: digest}
	}
	if isUnclassifiableArgv(argv) {
		return CommandClassification{Verdict: CommandVerdictPrompt, Reason: "unclassifiable", Digest: digest}
	}
	effectiveArgv, ok := unwrapCommandArgv(argv)
	if !ok || isUnclassifiableArgv(effectiveArgv) {
		return CommandClassification{Verdict: CommandVerdictPrompt, Reason: "unclassifiable", Digest: digest}
	}
	if isBlockedIrreversibleArgv(effectiveArgv) {
		return CommandClassification{Verdict: CommandVerdictDenied, Reason: commandReasonIrreversible, Digest: digest}
	}
	if isIrreversibleArgv(effectiveArgv) {
		return CommandClassification{Verdict: CommandVerdictPrompt, Reason: commandReasonIrreversible, Digest: digest}
	}
	for _, patterns := range denyLists {
		for _, pattern := range patterns {
			if matchArgvPattern(argv, pattern) {
				return CommandClassification{Verdict: CommandVerdictPrompt, Reason: "denylist", Digest: digest}
			}
		}
	}
	for _, pattern := range allowlist {
		if matchArgvPattern(argv, pattern) {
			return CommandClassification{Verdict: CommandVerdictAllowlisted, Reason: "allowlist", Digest: digest}
		}
	}
	return CommandClassification{
		Verdict: CommandVerdictPrompt, Reason: commandReasonApprovalRequired, Digest: digest,
	}
}

func argvDigest(argv []string) string {
	digest := sha256.Sum256([]byte(strings.Join(argv, "\x00")))
	return hex.EncodeToString(digest[:])
}

func normalizedExecutable(value string) string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(value)), ".exe")
}

func matchArgvPattern(argv []string, pattern ArgvPattern) bool {
	if len(argv) != len(pattern) || len(pattern) == 0 {
		return false
	}
	for index, expected := range pattern {
		actual := argv[index]
		if index == 0 {
			actual = filepath.Base(actual)
			expected = filepath.Base(expected)
		}
		if expected != "*" && actual != expected {
			return false
		}
	}
	return true
}
