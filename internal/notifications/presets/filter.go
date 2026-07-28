package presets

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Filter struct {
	root filterNode
}

type filterNode interface {
	Eval(Event) bool
}

type filterComparison struct {
	field string
	op    string
	value string
}

type filterAnd struct {
	left  filterNode
	right filterNode
}

type filterOr struct {
	left  filterNode
	right filterNode
}

type filterTokenKind string

const (
	filterTokenEOF    filterTokenKind = "eof"
	filterTokenIdent  filterTokenKind = "ident"
	filterTokenString filterTokenKind = "string"
	filterTokenOp     filterTokenKind = "op"
	filterTokenAnd    filterTokenKind = "and"
	filterTokenOr     filterTokenKind = "or"
	filterTokenLParen filterTokenKind = "lparen"
	filterTokenRParen filterTokenKind = "rparen"
)

const (
	filterFieldSeverity  = "severity"
	filterFieldOutcome   = "outcome"
	filterFieldWorkspace = "workspace"
	filterFieldAgent     = "agent"
	filterFieldProvider  = "provider"
	filterFieldEvent     = "event"
	filterFieldTask      = "task"
	filterFieldRun       = "run"
)

type filterToken struct {
	kind  filterTokenKind
	value string
}

type filterParser struct {
	tokens []filterToken
	pos    int
}

func CompileFilter(expr string) (*Filter, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return &Filter{}, nil
	}
	tokens, err := tokenizeFilter(trimmed)
	if err != nil {
		return nil, err
	}
	parser := &filterParser{tokens: tokens}
	node, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	if parser.peek().kind != filterTokenEOF {
		return nil, fmt.Errorf("%w: unexpected token %q", ErrInvalidPreset, parser.peek().value)
	}
	return &Filter{root: node}, nil
}

func (f *Filter) Eval(event Event) bool {
	if f == nil || f.root == nil {
		return true
	}
	return f.root.Eval(event)
}

func (n filterComparison) Eval(event Event) bool {
	left := filterFieldValue(event, n.field)
	right := strings.TrimSpace(n.value)
	if n.field == filterFieldSeverity || n.field == filterFieldOutcome {
		return compareSeverity(left, n.op, right)
	}
	switch n.op {
	case "=":
		return strings.EqualFold(left, right)
	case "!=":
		return !strings.EqualFold(left, right)
	case ">":
		return left > right
	case ">=":
		return strings.Compare(left, right) >= 0
	case "<":
		return left < right
	case "<=":
		return strings.Compare(left, right) <= 0
	default:
		return false
	}
}

func (n filterAnd) Eval(event Event) bool {
	return n.left.Eval(event) && n.right.Eval(event)
}

func (n filterOr) Eval(event Event) bool {
	return n.left.Eval(event) || n.right.Eval(event)
}

func (p *filterParser) parseOr() (filterNode, error) {
	node, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == filterTokenOr {
		p.next()
		right, parseErr := p.parseAnd()
		if parseErr != nil {
			return nil, parseErr
		}
		node = filterOr{left: node, right: right}
	}
	return node, nil
}

func (p *filterParser) parseAnd() (filterNode, error) {
	node, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == filterTokenAnd {
		p.next()
		right, parseErr := p.parsePrimary()
		if parseErr != nil {
			return nil, parseErr
		}
		node = filterAnd{left: node, right: right}
	}
	return node, nil
}

func (p *filterParser) parsePrimary() (filterNode, error) {
	if p.peek().kind == filterTokenLParen {
		p.next()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != filterTokenRParen {
			return nil, fmt.Errorf("%w: missing closing parenthesis", ErrInvalidPreset)
		}
		p.next()
		return node, nil
	}
	return p.parseComparison()
}

func (p *filterParser) parseComparison() (filterNode, error) {
	field := p.next()
	if field.kind != filterTokenIdent {
		return nil, fmt.Errorf("%w: filter field is required", ErrInvalidPreset)
	}
	fieldName := normalizeFilterField(field.value)
	if fieldName == "" {
		return nil, fmt.Errorf("%w: unsupported filter field %q", ErrInvalidPreset, field.value)
	}
	op := p.next()
	if op.kind != filterTokenOp {
		return nil, fmt.Errorf("%w: filter operator is required after %q", ErrInvalidPreset, field.value)
	}
	value := p.next()
	if value.kind != filterTokenIdent && value.kind != filterTokenString {
		return nil, fmt.Errorf("%w: filter value is required after %q", ErrInvalidPreset, op.value)
	}
	return filterComparison{field: fieldName, op: op.value, value: value.value}, nil
}

func (p *filterParser) peek() filterToken {
	if p.pos >= len(p.tokens) {
		return filterToken{kind: filterTokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *filterParser) next() filterToken {
	token := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return token
}

func tokenizeFilter(expr string) ([]filterToken, error) {
	tokens := make([]filterToken, 0)
	runes := []rune(expr)
	for i := 0; i < len(runes); {
		char := runes[i]
		switch {
		case unicode.IsSpace(char):
			i++
		case char == '(':
			tokens = append(tokens, filterToken{kind: filterTokenLParen, value: "("})
			i++
		case char == ')':
			tokens = append(tokens, filterToken{kind: filterTokenRParen, value: ")"})
			i++
		case char == '\'' || char == '"':
			value, next, err := readQuotedFilterValue(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, filterToken{kind: filterTokenString, value: value})
			i = next
		case strings.ContainsRune("=!<>", char):
			value, next, err := readFilterOperator(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, filterToken{kind: filterTokenOp, value: value})
			i = next
		default:
			value, next := readFilterIdentifier(runes, i)
			if value == "" {
				return nil, fmt.Errorf("%w: invalid filter token near %q", ErrInvalidPreset, string(char))
			}
			upper := strings.ToUpper(value)
			switch upper {
			case "AND":
				tokens = append(tokens, filterToken{kind: filterTokenAnd, value: value})
			case "OR":
				tokens = append(tokens, filterToken{kind: filterTokenOr, value: value})
			default:
				tokens = append(tokens, filterToken{kind: filterTokenIdent, value: value})
			}
			i = next
		}
	}
	tokens = append(tokens, filterToken{kind: filterTokenEOF})
	return tokens, nil
}

func readQuotedFilterValue(runes []rune, start int) (string, int, error) {
	quote := runes[start]
	var builder strings.Builder
	for i := start + 1; i < len(runes); i++ {
		if runes[i] == quote {
			return builder.String(), i + 1, nil
		}
		builder.WriteRune(runes[i])
	}
	return "", 0, fmt.Errorf("%w: unterminated filter string", ErrInvalidPreset)
}

func readFilterOperator(runes []rune, start int) (string, int, error) {
	if start+1 < len(runes) {
		candidate := string(runes[start : start+2])
		if candidate == "!=" || candidate == ">=" || candidate == "<=" {
			return candidate, start + 2, nil
		}
	}
	switch runes[start] {
	case '=', '>', '<':
		return string(runes[start]), start + 1, nil
	default:
		return "", 0, fmt.Errorf("%w: unsupported filter operator", ErrInvalidPreset)
	}
}

func readFilterIdentifier(runes []rune, start int) (string, int) {
	var builder strings.Builder
	for i := start; i < len(runes); i++ {
		char := runes[i]
		if unicode.IsSpace(char) || strings.ContainsRune("()=!<>", char) {
			return builder.String(), i
		}
		builder.WriteRune(char)
	}
	return builder.String(), len(runes)
}

func normalizeFilterField(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case filterFieldSeverity, filterFieldOutcome:
		return filterFieldSeverity
	case filterFieldWorkspace, "workspace_id":
		return filterFieldWorkspace
	case filterFieldAgent, "agent_name":
		return filterFieldAgent
	case filterFieldProvider:
		return filterFieldProvider
	case filterFieldEvent, "type", "event_type":
		return filterFieldEvent
	case filterFieldTask, "task_id":
		return filterFieldTask
	case filterFieldRun, "run_id":
		return filterFieldRun
	default:
		return ""
	}
}

func filterFieldValue(event Event, field string) string {
	switch field {
	case filterFieldSeverity:
		return string(event.Outcome)
	case filterFieldWorkspace:
		return event.WorkspaceID
	case filterFieldAgent:
		return event.AgentName
	case filterFieldProvider:
		return event.Provider
	case filterFieldEvent:
		return event.Type
	case filterFieldTask:
		return event.TaskID
	case filterFieldRun:
		return event.RunID
	default:
		return ""
	}
}

func compareSeverity(left string, op string, right string) bool {
	leftRank, leftOK := severityRank(left)
	rightRank, rightOK := severityRank(right)
	if !leftOK || !rightOK {
		return false
	}
	switch op {
	case "=":
		return leftRank == rightRank
	case "!=":
		return leftRank != rightRank
	case ">":
		return leftRank > rightRank
	case ">=":
		return leftRank >= rightRank
	case "<":
		return leftRank < rightRank
	case "<=":
		return leftRank <= rightRank
	default:
		return false
	}
}

func severityRank(value string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info":
		return 0, true
	case "success":
		return 1, true
	case "warning", "warn":
		return 2, true
	case "failure", "error", "failed":
		return 3, true
	default:
		if numeric, err := strconv.Atoi(value); err == nil {
			return numeric, true
		}
		return 0, false
	}
}
