package terminal

import (
	"path/filepath"
	"slices"
	"strings"
)

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
