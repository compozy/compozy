package terminal

import (
	"slices"
	"strings"
)

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
