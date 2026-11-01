package shellwait

import (
	"slices"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// whileClausePolls treats any sleep in a while or until loop, in its condition
// or its body, as polling.
func whileClausePolls(clause *syntax.WhileClause) bool {
	stmts := append(slices.Clone(clause.Cond), clause.Do...)
	seconds, known, found := loopSleep(stmts, 0)
	return found && (!known || seconds > 0)
}

// inputPaced reports a loop that drains a stream with read: its pace comes from
// the input, so its sleeps are not polling.
func inputPaced(clause *syntax.WhileClause) bool {
	return !clause.Until && conditionReads(clause.Cond)
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
	seconds, known, found := loopSleep(clause.Do, 0)
	if !found {
		return false
	}
	return !known || seconds*float64(loopIterations(clause.Loop)) >= MinDelaySeconds
}

// loopSleep totals the sleeps in body, reading through nested shell strings.
func loopSleep(body []*syntax.Stmt, depth int) (seconds float64, known bool, found bool) {
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
			if !isSleep && depth < maxNestedShells {
				value, valueKnown, isSleep = nestedLoopSleep(argv, depth)
			}
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

func nestedLoopSleep(argv []string, depth int) (float64, bool, bool) {
	script, ok := nestedShellScript(argv)
	if !ok {
		return 0, false, false
	}
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(script), "")
	if err != nil {
		return 0, false, false
	}
	return loopSleep(file.Stmts, depth+1)
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
	literal := word.Lit()
	if count, ok := braceRangeLength(literal); ok {
		return count
	}
	if literal == "" || strings.ContainsAny(literal, "*?[{") {
		return unknownIterations
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
