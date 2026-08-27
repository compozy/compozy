package terminal

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"slices"
	"strings"
)

// CommandVerdict describes whether parsed argv may bypass interactive approval.
type CommandVerdict string

const (
	CommandVerdictPrompt      CommandVerdict = "prompt"
	CommandVerdictAllowlisted CommandVerdict = "allowlisted"
	CommandVerdictDenied      CommandVerdict = "denied"
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
	if isIrreversibleArgv(argv) {
		return CommandClassification{Verdict: CommandVerdictDenied, Reason: "irreversible", Digest: digest}
	}
	if isUnclassifiableArgv(argv) {
		return CommandClassification{Verdict: CommandVerdictPrompt, Reason: "unclassifiable", Digest: digest}
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
	return CommandClassification{Verdict: CommandVerdictPrompt, Reason: errorCodeApprovalRequired, Digest: digest}
}

func argvDigest(argv []string) string {
	digest := sha256.Sum256([]byte(strings.Join(argv, "\x00")))
	return hex.EncodeToString(digest[:])
}

func isUnclassifiableArgv(argv []string) bool {
	executable := strings.ToLower(filepath.Base(argv[0]))
	if executable == "eval" || strings.ContainsAny(executable, "'\"") {
		return true
	}
	if slices.Contains(
		[]string{"sh", "bash", "dash", "zsh", "fish", "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh"},
		executable,
	) {
		for _, arg := range argv[1:] {
			if slices.Contains([]string{"-c", "-lc", "-Command", "/c", "/C"}, arg) {
				return true
			}
		}
	}
	for _, arg := range argv {
		if strings.ContainsAny(arg, "\n\r`|") || strings.Contains(arg, "$(") ||
			strings.Contains(arg, "${") || strings.Contains(arg, "&&") || strings.Contains(arg, "||") ||
			strings.Contains(arg, ";") || strings.Contains(arg, "\\x") || strings.Contains(arg, "\\u") {
			return true
		}
	}
	return false
}

func isIrreversibleArgv(argv []string) bool {
	for _, arg := range argv {
		compact := strings.ReplaceAll(strings.TrimSpace(arg), " ", "")
		if strings.Contains(compact, ":(){:|:&};:") {
			return true
		}
	}
	executable := strings.ToLower(filepath.Base(argv[0]))
	switch executable {
	case "mkfs", "mkfs.ext2", "mkfs.ext3", "mkfs.ext4", "mkfs.xfs", "fdisk", "sfdisk":
		return true
	case "dd":
		for _, arg := range argv[1:] {
			if strings.HasPrefix(arg, "of=/dev/") {
				return true
			}
		}
	case "diskutil":
		return len(argv) > 1 && slices.Contains([]string{"eraseDisk", "zeroDisk", "secureErase"}, argv[1])
	case "rm":
		return destructiveRemove(argv[1:])
	}
	return false
}

func destructiveRemove(args []string) bool {
	recursive, forced := false, false
	targets := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			recursive = recursive || strings.Contains(arg, "r") || strings.Contains(arg, "R")
			forced = forced || strings.Contains(arg, "f")
			continue
		}
		targets = append(targets, filepath.Clean(arg))
	}
	if !recursive || !forced {
		return false
	}
	for _, target := range targets {
		if target == string(filepath.Separator) || target == "." || target == ".." || target == "$HOME" ||
			target == "~" {
			return true
		}
	}
	return false
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
