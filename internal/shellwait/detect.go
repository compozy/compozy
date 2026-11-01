// Package shellwait recognizes provider shell commands that block on external
// work, so a session can be redirected to a daemon-owned monitor instead. It
// inspects parsed commands rather than text, so a quoted mention of a watch in a
// commit message or a search pattern is not a wait.
package shellwait

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/terminal"
	"mvdan.cc/sh/v3/syntax"
)

// Kind names the shape of a detected wait.
type Kind string

const (
	// KindWatch is a command that blocks until remote work finishes, such as gh pr checks --watch.
	KindWatch Kind = "watch"
	// KindSleep is a plain delay of at least MinDelaySeconds.
	KindSleep Kind = "sleep"
	// KindPoll is a loop that sleeps between checks.
	KindPoll Kind = "poll"
)

const (
	// MinDelaySeconds is the shortest wait that is redirected. It matches the
	// smallest timer monitor, so every denied delay has a monitor replacement.
	MinDelaySeconds = 5
	// MaxDelaySeconds is the longest timer monitor.
	MaxDelaySeconds = 86400

	// unknownIterations stands in for a counted loop whose length is not literal.
	unknownIterations = 5
	maxNestedShells   = 3

	shellPOSIX      = "sh"
	shellPowerShell = "powershell"
)

// Finding describes one detected wait and the data its redirect needs.
type Finding struct {
	Kind Kind
	// Argv is the unwrapped command for KindWatch.
	Argv []string
	// Source is the full command line for KindPoll.
	Source string
	// Shell runs Source: sh or powershell.
	Shell string
	// DelaySeconds is the delay for KindSleep.
	DelaySeconds int
}

// Detect reports the first wait in a POSIX shell command line. An unparsable
// command is allowed: the guard fails open.
func Detect(command string) (Finding, bool) {
	return detectDepth(command, 0)
}

func detectDepth(command string, depth int) (Finding, bool) {
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return Finding{}, false
	}
	var finding Finding
	found := false
	syntax.Walk(file, func(node syntax.Node) bool {
		if found {
			return false
		}
		switch typed := node.(type) {
		case *syntax.CallExpr:
			finding, found = detectCall(wordValues(typed.Args), depth)
		case *syntax.WhileClause:
			found = whileClausePolls(typed)
		case *syntax.ForClause:
			found = forClausePolls(typed)
		}
		if found && finding.Kind == "" {
			finding = Finding{Kind: KindPoll, Source: command, Shell: shellPOSIX}
		}
		return !found
	})
	return finding, found
}

func detectCall(argv []string, depth int) (Finding, bool) {
	argv = unwrapCommand(argv)
	if len(argv) == 0 {
		return Finding{}, false
	}
	if depth < maxNestedShells {
		if script, ok := nestedShellScript(argv); ok {
			return detectDepth(script, depth+1)
		}
	}
	if isWatchCommand(argv) {
		return Finding{Kind: KindWatch, Argv: argv}, true
	}
	if seconds, known, ok := sleepSeconds(argv); ok && known && seconds >= MinDelaySeconds {
		return Finding{Kind: KindSleep, DelaySeconds: clampDelay(seconds)}, true
	}
	return Finding{}, false
}

// unwrapCommand removes the launch wrappers that do not change what a command
// waits for, including the timeout wrappers agents add around a watch.
func unwrapCommand(argv []string) []string {
	for len(argv) > 0 {
		next, unwrapped := unwrapOnce(argv)
		if !unwrapped {
			return argv
		}
		argv = next
	}
	return nil
}

func unwrapOnce(argv []string) ([]string, bool) {
	switch executableName(argv[0]) {
	case "sudo", "doas", "env":
		effective, ok := terminal.UnwrapCommandArgv(argv)
		if !ok {
			return nil, true
		}
		return effective, true
	case "timeout", "gtimeout":
		return skipTimeoutOptions(argv[1:]), true
	case "nice":
		return skipOptions(argv[1:], "-n", "--adjustment"), true
	case "stdbuf":
		return skipOptions(argv[1:], "-i", "-o", "-e", "--input", "--output", "--error"), true
	case "command":
		if len(argv) > 1 && (argv[1] == "-v" || argv[1] == "-V") {
			return nil, true
		}
		return skipOptions(argv[1:]), true
	case "exec", "nohup", "time", "builtin":
		return skipOptions(argv[1:]), true
	}
	return argv, false
}

func skipTimeoutOptions(args []string) []string {
	rest := skipOptions(args, "-k", "-s", "--kill-after", "--signal")
	if len(rest) == 0 {
		return nil
	}
	return rest[1:]
}

func skipOptions(args []string, valueOptions ...string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-" {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		for _, valueOption := range valueOptions {
			if option == valueOption && len(args) > 0 {
				args = args[1:]
				break
			}
		}
	}
	return args
}

func nestedShellScript(argv []string) (string, bool) {
	switch executableName(argv[0]) {
	case "eval":
		return strings.Join(argv[1:], " "), len(argv) > 1
	case "sh", "bash", "dash", "zsh", "ksh", "fish":
		for index, arg := range argv[1:] {
			if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
				return "", false
			}
			if strings.Contains(arg, "c") {
				if index+2 < len(argv) {
					return argv[index+2], argv[index+2] != ""
				}
				return "", false
			}
		}
	}
	return "", false
}

func isWatchCommand(argv []string) bool {
	switch executableName(argv[0]) {
	case "gh":
		return isGitHubWatch(argv[1:])
	case "watch":
		return len(argv) > 1 && !hasHelpFlag(argv[1:])
	case "tail":
		return tailFollows(argv[1:])
	}
	return false
}

func isGitHubWatch(args []string) bool {
	if hasHelpFlag(args) {
		return false
	}
	positional := make([]string, 0, 2)
	for index := 0; index < len(args) && len(positional) < 2; index++ {
		arg := args[index]
		switch {
		case arg == "-R" || arg == "--repo":
			index++
		case !strings.HasPrefix(arg, "-"):
			positional = append(positional, arg)
		}
	}
	if len(positional) < 2 {
		return false
	}
	switch positional[0] + " " + positional[1] {
	case "run watch":
		return true
	case "pr checks":
		return hasEnabledWatchFlag(args)
	}
	return false
}

func hasEnabledWatchFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--watch" || arg == "--watch=true" {
			return true
		}
	}
	return false
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "--version" {
			return true
		}
	}
	return false
}

func tailFollows(args []string) bool {
	for _, arg := range args {
		switch {
		case arg == "--":
			return false
		case arg == "--follow" || strings.HasPrefix(arg, "--follow="):
			return true
		case strings.HasPrefix(arg, "--") || !strings.HasPrefix(arg, "-"):
			continue
		case strings.ContainsAny(arg, "fF"):
			return true
		}
	}
	return false
}

// sleepSeconds returns the delay of a sleep command. known is false when an
// argument is not a literal duration; ok is false for any other command.
func sleepSeconds(argv []string) (seconds float64, known bool, ok bool) {
	if executableName(argv[0]) != "sleep" || len(argv) < 2 {
		return 0, false, false
	}
	known = true
	for _, arg := range argv[1:] {
		value, parsed := parseDuration(arg)
		if !parsed {
			known = false
			continue
		}
		seconds += value
	}
	return seconds, known, true
}

func parseDuration(value string) (float64, bool) {
	if value == "infinity" {
		return math.Inf(1), true
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(value, "s"):
		value = strings.TrimSuffix(value, "s")
	case strings.HasSuffix(value, "m"):
		value, multiplier = strings.TrimSuffix(value, "m"), 60
	case strings.HasSuffix(value, "h"):
		value, multiplier = strings.TrimSuffix(value, "h"), 3600
	case strings.HasSuffix(value, "d"):
		value, multiplier = strings.TrimSuffix(value, "d"), 86400
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || math.IsNaN(parsed) {
		return 0, false
	}
	return parsed * multiplier, true
}

func clampDelay(seconds float64) int {
	if seconds >= MaxDelaySeconds {
		return MaxDelaySeconds
	}
	return max(int(math.Ceil(seconds)), MinDelaySeconds)
}

// whileClausePolls treats any sleep in a while or until loop as polling, except
// a loop that drains a stream with read: its pace comes from the input.
func whileClausePolls(clause *syntax.WhileClause) bool {
	if !clause.Until && conditionReads(clause.Cond) {
		return false
	}
	seconds, known, found := loopSleep(clause.Do)
	return found && (!known || seconds > 0)
}

func conditionReads(condition []*syntax.Stmt) bool {
	if len(condition) != 1 {
		return false
	}
	call, ok := condition[0].Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	return call.Args[0].Lit() == "read"
}

// forClausePolls estimates a counted loop's total delay, so a brief pause across
// a few items stays allowed while a retry loop does not.
func forClausePolls(clause *syntax.ForClause) bool {
	seconds, known, found := loopSleep(clause.Do)
	if !found {
		return false
	}
	return !known || seconds*float64(loopIterations(clause.Loop)) >= MinDelaySeconds
}

func loopSleep(body []*syntax.Stmt) (seconds float64, known bool, found bool) {
	known = true
	for _, stmt := range body {
		syntax.Walk(stmt, func(node syntax.Node) bool {
			call, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}
			argv := unwrapCommand(wordValues(call.Args))
			if len(argv) == 0 {
				return true
			}
			value, valueKnown, isSleep := sleepSeconds(argv)
			if isSleep {
				found = true
				seconds += value
				known = known && valueKnown
			}
			return true
		})
	}
	return seconds, known, found
}

func loopIterations(loop syntax.Loop) int {
	iter, ok := loop.(*syntax.WordIter)
	if !ok {
		return unknownIterations
	}
	total := 0
	for _, word := range iter.Items {
		total += wordIterations(word)
	}
	return max(total, 1)
}

func wordIterations(word *syntax.Word) int {
	if len(word.Parts) == 1 {
		if subst, ok := word.Parts[0].(*syntax.CmdSubst); ok {
			return commandIterations(subst)
		}
	}
	if count, ok := braceRangeLength(word.Lit()); ok {
		return count
	}
	return 1
}

func commandIterations(subst *syntax.CmdSubst) int {
	if len(subst.Stmts) != 1 {
		return unknownIterations
	}
	call, ok := subst.Stmts[0].Cmd.(*syntax.CallExpr)
	if !ok {
		return unknownIterations
	}
	argv := wordValues(call.Args)
	if len(argv) < 2 || executableName(argv[0]) != "seq" {
		return unknownIterations
	}
	first, last := "1", argv[len(argv)-1]
	if len(argv) == 3 {
		first = argv[1]
	}
	if len(argv) > 3 {
		return unknownIterations
	}
	return rangeLength(first, last)
}

func braceRangeLength(value string) (int, bool) {
	body, ok := strings.CutPrefix(value, "{")
	if !ok {
		return 0, false
	}
	body, ok = strings.CutSuffix(body, "}")
	if !ok {
		return 0, false
	}
	first, last, ok := strings.Cut(body, "..")
	if !ok {
		return 0, false
	}
	return rangeLength(first, last), true
}

func rangeLength(first string, last string) int {
	start, startErr := strconv.Atoi(first)
	end, endErr := strconv.Atoi(last)
	if startErr != nil || endErr != nil {
		return unknownIterations
	}
	return max(end-start, start-end) + 1
}

// wordValues renders each word's literal value, or an empty string when any
// part needs expansion at runtime.
func wordValues(words []*syntax.Word) []string {
	values := make([]string, 0, len(words))
	for _, word := range words {
		values = append(values, wordValue(word))
	}
	return values
}

func wordValue(word *syntax.Word) string {
	var builder strings.Builder
	for _, part := range word.Parts {
		switch typed := part.(type) {
		case *syntax.Lit:
			builder.WriteString(typed.Value)
		case *syntax.SglQuoted:
			builder.WriteString(typed.Value)
		case *syntax.DblQuoted:
			if len(typed.Parts) != 1 {
				return ""
			}
			lit, ok := typed.Parts[0].(*syntax.Lit)
			if !ok {
				return ""
			}
			builder.WriteString(lit.Value)
		default:
			return ""
		}
	}
	return builder.String()
}

// executableName resolves a command to its base name so /bin/sleep matches
// sleep. A relative path names a project script, never the system tool.
func executableName(value string) string {
	if strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return ""
	}
	base := filepath.Base(strings.ReplaceAll(value, `\`, "/"))
	return strings.TrimSuffix(strings.ToLower(base), ".exe")
}
