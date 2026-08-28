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

func isUnclassifiableArgv(argv []string) bool {
	if len(argv) == 0 {
		return true
	}
	executable := normalizedExecutable(argv[0])
	if executable == "eval" || strings.ContainsAny(executable, "'\"") {
		return true
	}
	if shellOptionExecutesCommandString(executable, argv[1:]) {
		return true
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

func shellOptionExecutesCommandString(executable string, args []string) bool {
	switch executable {
	case "sh", "bash", "dash", "zsh":
		return posixShellOptionExecutesCommandString(args)
	case "fish":
		return fishOptionExecutesCommandString(args)
	case "powershell", "pwsh":
		return powerShellOptionExecutesCommandString(args)
	case "cmd":
		return cmdOptionExecutesCommandString(args)
	}
	return false
}

func cmdOptionExecutesCommandString(args []string) bool {
	for _, arg := range args {
		option := strings.ToLower(arg)
		if strings.HasPrefix(option, "/c") || strings.HasPrefix(option, "/k") {
			return true
		}
	}
	return false
}

func posixShellOptionExecutesCommandString(args []string) bool {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" || arg == "-" || !strings.HasPrefix(arg, "-") {
			return false
		}
		if strings.HasPrefix(arg, "--") {
			if arg == "--rcfile" || arg == "--init-file" {
				index++
			}
			continue
		}
		shortOptions := strings.TrimPrefix(arg, "-")
		for optionIndex, option := range shortOptions {
			if option == 'c' {
				return true
			}
			if option == 'o' || option == 'O' {
				if optionIndex == len(shortOptions)-1 {
					index++
				}
				break
			}
		}
	}
	return false
}

func fishOptionExecutesCommandString(args []string) bool {
	for _, arg := range args {
		if arg == "--" || arg == "-" || !strings.HasPrefix(arg, "-") {
			return false
		}
		if arg == "--command" || strings.HasPrefix(arg, "--command=") ||
			arg == "--init-command" || strings.HasPrefix(arg, "--init-command=") {
			return true
		}
		if !strings.HasPrefix(arg, "--") && strings.ContainsAny(strings.TrimPrefix(arg, "-"), "cC") {
			return true
		}
	}
	return false
}

func powerShellOptionExecutesCommandString(args []string) bool {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			return false
		}
		option, inlineValue := powerShellOptionName(arg)
		switch option {
		case "-c", "-command", "-e", "-ec", "-en", "-enc", "-encodedcommand", "-commandwithargs":
			return true
		}
		if isPowerShellOptionAbbreviation(option, "-encodedcommand", "-encodedc") {
			return true
		}
		if isPowerShellOptionAbbreviation(option, "-commandwithargs", "-commandw") {
			return true
		}
		if option == "-f" || option == "-file" {
			return false
		}
		if !inlineValue && powerShellOptionConsumesValue(option) {
			index++
		}
	}
	return false
}

func isPowerShellOptionAbbreviation(option, canonical, minimum string) bool {
	if len(option) < len(minimum) || len(option) > len(canonical) {
		return false
	}
	return option == canonical[:len(option)]
}

func powerShellOptionName(arg string) (string, bool) {
	option := strings.ToLower(arg)
	for _, separator := range []string{":", "="} {
		if name, _, found := strings.Cut(option, separator); found {
			return name, true
		}
	}
	return option, false
}

func powerShellOptionConsumesValue(option string) bool {
	switch option {
	case "-configurationfile", "-configurationname", "-custompipename", "-executionpolicy", "-inputformat",
		"-outputformat", "-settingsfile", "-version", "-windowstyle", "-workingdirectory":
		return true
	default:
		return false
	}
}

type wrapperOptionSpec struct {
	shortFlags       string
	shortValueFlags  string
	shortOpaqueFlags string
	longFlags        []string
	longValueFlags   []string
	longOpaqueFlags  []string
	longOptional     []string
	assignments      bool
}

func unwrapCommandArgv(argv []string) ([]string, bool) {
	effective := argv
	for len(effective) > 0 {
		var spec wrapperOptionSpec
		switch normalizedExecutable(effective[0]) {
		case "sudo":
			spec = wrapperOptionSpec{
				shortFlags: "AbEHKnPSVvk", shortValueFlags: "aCDghpRrtTu", shortOpaqueFlags: "eis",
				longFlags: []string{
					"--askpass", "--background", "--preserve-env", "--set-home", "--remove-timestamp",
					"--reset-timestamp", "--non-interactive", "--preserve-groups", "--stdin", "--version",
					"--validate",
				},
				longValueFlags: []string{
					"--auth-type", "--close-from", "--chdir", "--group", "--host", "--prompt", "--chroot",
					"--role", "--type", "--command-timeout", "--user",
				},
				longOpaqueFlags: []string{"--edit", "--login", "--shell"},
				longOptional:    []string{"--preserve-env"},
				assignments:     true,
			}
		case "doas":
			spec = wrapperOptionSpec{
				shortFlags: "Ln", shortValueFlags: "aCu", shortOpaqueFlags: "s",
			}
		case "env":
			spec = wrapperOptionSpec{
				shortFlags: "i0", shortValueFlags: "aCu", shortOpaqueFlags: "S",
				longFlags:       []string{"--ignore-environment", "--null"},
				longValueFlags:  []string{"--argv0", "--chdir", "--unset"},
				longOpaqueFlags: []string{"--split-string"},
				assignments:     true,
			}
		default:
			return effective, true
		}

		var ok bool
		effective, ok = unwrapArgvWrapper(effective[1:], spec)
		if !ok {
			return nil, false
		}
	}
	return nil, false
}

func unwrapArgvWrapper(args []string, spec wrapperOptionSpec) ([]string, bool) {
	index := 0
	for index < len(args) {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}
		if strings.HasPrefix(arg, "--") {
			consumeNext, ok := classifyLongWrapperOption(arg, spec)
			if !ok || consumeNext && index+1 >= len(args) {
				return nil, false
			}
			index++
			if consumeNext {
				index++
			}
			continue
		}
		consumeNext, ok := classifyShortWrapperOptions(arg, spec)
		if !ok || consumeNext && index+1 >= len(args) {
			return nil, false
		}
		index++
		if consumeNext {
			index++
		}
	}
	if spec.assignments {
		for index < len(args) && isEnvironmentAssignment(args[index]) {
			index++
		}
	}
	if index >= len(args) {
		return nil, false
	}
	return args[index:], true
}

func classifyLongWrapperOption(arg string, spec wrapperOptionSpec) (bool, bool) {
	name, _, hasValue := strings.Cut(arg, "=")
	if slices.Contains(spec.longOpaqueFlags, name) {
		return false, false
	}
	if slices.Contains(spec.longOptional, name) {
		return false, true
	}
	if slices.Contains(spec.longFlags, name) {
		return false, !hasValue
	}
	if slices.Contains(spec.longValueFlags, name) {
		return !hasValue, true
	}
	return false, false
}

func classifyShortWrapperOptions(arg string, spec wrapperOptionSpec) (bool, bool) {
	flags := strings.TrimPrefix(arg, "-")
	if flags == "" {
		return false, false
	}
	for index, flag := range flags {
		if strings.ContainsRune(spec.shortOpaqueFlags, flag) {
			return false, false
		}
		if strings.ContainsRune(spec.shortFlags, flag) {
			continue
		}
		if strings.ContainsRune(spec.shortValueFlags, flag) {
			return index == len(flags)-1, true
		}
		return false, false
	}
	return false, true
}

func isEnvironmentAssignment(arg string) bool {
	name, _, ok := strings.Cut(arg, "=")
	if !ok || name == "" {
		return false
	}
	for index, char := range name {
		if char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}

func normalizedExecutable(value string) string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(value)), ".exe")
}

func isIrreversibleArgv(argv []string) bool {
	executable := normalizedExecutable(argv[0])
	if executable == "mkfs" || strings.HasPrefix(executable, "mkfs.") {
		return true
	}
	switch executable {
	case "mkntfs", "fdisk", "sfdisk", "cfdisk", "gdisk", "sgdisk", "parted", "shred", "wipe", "format", "format.com":
		return true
	case "dd":
		for _, arg := range argv[1:] {
			if strings.HasPrefix(arg, "of=/dev/") {
				return true
			}
		}
	case "diskutil":
		return len(argv) > 1 && slices.Contains(
			[]string{"erasedisk", "zerodisk", "secureerase"},
			strings.ToLower(argv[1]),
		)
	case "rm":
		return destructiveRemove(argv[1:])
	case "cipher":
		for _, arg := range argv[1:] {
			wipeArg := strings.ToLower(arg)
			if wipeArg == "/w" || strings.HasPrefix(wipeArg, "/w:") {
				return true
			}
		}
	}
	return false
}

func isBlockedIrreversibleArgv(argv []string) bool {
	for _, arg := range argv {
		compact := strings.ReplaceAll(strings.TrimSpace(arg), " ", "")
		if strings.Contains(compact, ":(){:|:&};:") {
			return true
		}
	}
	if normalizedExecutable(argv[0]) == "rm" {
		return destructiveRemoveTargetsRoot(argv[1:])
	}
	return false
}

func destructiveRemove(args []string) bool {
	recursive, forced, targets := destructiveRemoveShape(args)
	return recursive && forced && len(targets) > 0
}

func destructiveRemoveTargetsRoot(args []string) bool {
	recursive, forced, targets := destructiveRemoveShape(args)
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

func destructiveRemoveShape(args []string) (bool, bool, []string) {
	recursive, forced, endOptions := false, false, false
	targets := make([]string, 0, len(args))
	for _, arg := range args {
		if !endOptions && arg == "--" {
			endOptions = true
			continue
		}
		if !endOptions && strings.HasPrefix(arg, "--") {
			recursive = recursive || arg == "--recursive"
			forced = forced || arg == "--force"
			continue
		}
		if !endOptions && strings.HasPrefix(arg, "-") && arg != "-" {
			flags := strings.TrimPrefix(arg, "-")
			recursive = recursive || strings.ContainsAny(flags, "rR")
			forced = forced || strings.Contains(flags, "f")
			continue
		}
		targets = append(targets, filepath.Clean(strings.TrimSpace(arg)))
	}
	return recursive, forced, targets
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
