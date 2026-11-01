package shellwait

import (
	"strconv"
	"strings"
)

const powerShellSeparators = ";|&\n\r"

// quotedSpace stands in for a separator inside a quoted value, so a value stays
// one argument and a quoted command line never reads as a statement.
const quotedSpace = '\x00'

// DetectPowerShell reports the first wait in a PowerShell command line. It reads
// statements token by token because the POSIX parser rejects PowerShell loops.
func DetectPowerShell(command string) (Finding, bool) {
	scan := powerShellScan{source: command, frames: []*powerShellLoop{{plain: true, items: 1}}}
	var statement strings.Builder
	flush := func() (Finding, bool) {
		text := statement.String()
		statement.Reset()
		return scan.statement(strings.Fields(text))
	}
	for _, char := range flattenPowerShellQuotes(command) {
		structural := char == '{' || char == '}'
		if !structural && !strings.ContainsRune(powerShellSeparators+"()", char) {
			statement.WriteRune(char)
			continue
		}
		if finding, found := flush(); found {
			return finding, true
		}
		if structural && scan.brace(char) {
			return scan.pollFinding(), true
		}
	}
	if finding, found := flush(); found {
		return finding, true
	}
	if scan.frames[0].seconds >= MinDelaySeconds {
		return Finding{Kind: KindSleep, DelaySeconds: clampDelay(scan.frames[0].seconds)}, true
	}
	return Finding{}, false
}

// powerShellLoop totals the sleeps of one loop body; the first frame is the top
// level of the command.
type powerShellLoop struct {
	// plain is a brace block that does not repeat, such as an if body.
	plain   bool
	counted bool
	items   int
	seconds float64
	unknown bool
	found   bool
}

type powerShellScan struct {
	source     string
	frames     []*powerShellLoop
	pending    *powerShellLoop
	statements int
}

func (s *powerShellScan) statement(argv []string) (Finding, bool) {
	if len(argv) == 0 {
		return Finding{}, false
	}
	s.statements++
	if loop, ok := newPowerShellLoop(argv[0]); ok {
		s.pending = loop
		return Finding{}, false
	}
	if s.pending != nil && s.pending.counted {
		s.pending.items = powerShellItems(argv)
		return Finding{}, false
	}
	frame := s.frames[len(s.frames)-1]
	seconds, known, isSleep := powerShellSleepSeconds(argv)
	if isSleep {
		frame.seconds += seconds
		frame.unknown = frame.unknown || !known
		frame.found = true
		return Finding{}, false
	}
	finding, found := detectCall(argv, 0)
	if found && finding.Kind == KindWatch && finding.Source == "" {
		finding.Source, finding.Shell = s.source, shellPowerShell
		finding.Exact = s.isOnlyStatement() && !strings.Contains(strings.Join(argv, " "), "$")
	}
	return finding, found
}

// isOnlyStatement reads ahead: a watch is exact only when nothing else surrounds it.
func (s *powerShellScan) isOnlyStatement() bool {
	return s.statements == 1 && len(s.frames) == 1 &&
		len(strings.FieldsFunc(flattenPowerShellQuotes(s.source), func(char rune) bool {
			return strings.ContainsRune(powerShellSeparators+"(){}", char)
		})) == 1
}

// brace opens or closes a loop body and reports whether a closed loop polled.
func (s *powerShellScan) brace(char rune) bool {
	if char == '{' {
		loop := s.pending
		s.pending = nil
		if loop == nil {
			loop = &powerShellLoop{plain: true, items: 1}
		}
		s.frames = append(s.frames, loop)
		return false
	}
	if len(s.frames) == 1 {
		return false
	}
	loop := s.frames[len(s.frames)-1]
	s.frames = s.frames[:len(s.frames)-1]
	total := loop.seconds * float64(loop.items)
	if loop.found && loop.polls(total) {
		return true
	}
	parent := s.frames[len(s.frames)-1]
	parent.seconds += total
	parent.found = parent.found || loop.found
	parent.unknown = parent.unknown || loop.unknown
	return false
}

func (l *powerShellLoop) polls(total float64) bool {
	switch {
	case l.plain:
		return false
	case l.counted:
		return l.unknown || total >= MinDelaySeconds
	}
	return l.unknown || l.seconds > 0
}

func (s *powerShellScan) pollFinding() Finding {
	return Finding{Kind: KindPoll, Source: s.source, Shell: shellPowerShell}
}

func newPowerShellLoop(word string) (*powerShellLoop, bool) {
	switch strings.ToLower(word) {
	case "while", "until", "do":
		return &powerShellLoop{items: 1}, true
	case "for", "foreach":
		return &powerShellLoop{counted: true, items: unknownIterations}, true
	}
	return nil, false
}

// powerShellItems counts a foreach header's literal list, or reports it unknown.
func powerShellItems(header []string) int {
	if len(header) < 3 || !strings.EqualFold(header[1], "in") {
		return unknownIterations
	}
	list := strings.Join(header[2:], "")
	if strings.ContainsAny(list, "$.") {
		return unknownIterations
	}
	return strings.Count(list, ",") + 1
}

// flattenPowerShellQuotes drops quote marks and keeps each quoted value as one
// token, so "30" is an argument while "gh run watch 1" is not a command.
func flattenPowerShellQuotes(command string) string {
	var builder strings.Builder
	var quote rune
	for _, char := range command {
		switch {
		case quote != 0 && char == quote:
			quote = 0
		case quote != 0 && strings.ContainsRune(" \t(){}"+powerShellSeparators, char):
			builder.WriteRune(quotedSpace)
		case quote != 0:
			builder.WriteRune(char)
		case char == '\'' || char == '"':
			quote = char
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

// powerShellSleepSeconds returns the delay of a sleep statement. known is false
// when its argument is not a literal number; ok is false for other statements.
func powerShellSleepSeconds(argv []string) (seconds float64, known bool, ok bool) {
	switch executableName(argv[0]) {
	case "start-sleep", "sleep":
	default:
		return 0, false, false
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
			return 0, false, true
		}
		return value * scale, true, true
	}
	return 0, false, true
}
