package shellwait

import (
	"strconv"
	"strings"
)

// DetectPowerShell reports the first wait in a PowerShell command line. It reads
// statements token by token because the POSIX parser rejects PowerShell loops;
// quoted text is dropped first, so a mention inside a string is not a wait.
func DetectPowerShell(command string) (Finding, bool) {
	inLoop := false
	var loopSeconds float64
	for _, statement := range powerShellStatements(stripPowerShellStrings(command)) {
		argv := strings.Fields(statement)
		if len(argv) == 0 {
			continue
		}
		if isPowerShellLoopKeyword(argv[0]) {
			inLoop = true
			continue
		}
		seconds, isSleep := powerShellSleepSeconds(argv)
		switch {
		case isSleep:
			loopSeconds += seconds
		default:
			if finding, found := detectCall(argv, maxNestedShells); found {
				return finding, true
			}
		}
	}
	switch {
	case inLoop && loopSeconds >= 1:
		return Finding{Kind: KindPoll, Source: command, Shell: shellPowerShell}, true
	case loopSeconds >= MinDelaySeconds:
		return Finding{Kind: KindSleep, DelaySeconds: clampDelay(loopSeconds)}, true
	}
	return Finding{}, false
}

func stripPowerShellStrings(command string) string {
	var builder strings.Builder
	var quote rune
	for _, char := range command {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			quote = char
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func powerShellStatements(command string) []string {
	return strings.FieldsFunc(command, func(char rune) bool {
		return strings.ContainsRune(";|&\n\r{}()", char)
	})
}

func isPowerShellLoopKeyword(word string) bool {
	switch strings.ToLower(word) {
	case "while", "until", "do", "for", "foreach":
		return true
	}
	return false
}

func powerShellSleepSeconds(argv []string) (float64, bool) {
	switch executableName(argv[0]) {
	case "start-sleep", "sleep":
	default:
		return 0, false
	}
	scale := 1.0
	for index := 1; index < len(argv); index++ {
		option := strings.ToLower(argv[index])
		if strings.HasPrefix(option, "-m") {
			scale = 0.001
			continue
		}
		if strings.HasPrefix(option, "-") {
			continue
		}
		value, err := strconv.ParseFloat(option, 64)
		if err != nil {
			return 0, true
		}
		return value * scale, true
	}
	return 0, true
}
