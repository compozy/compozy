package store

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
)

const (
	initialMigrationScanBuffer = 16 * 1024
	maxMigrationScanToken      = 4 * 1024 * 1024
)

type migrationSQLDirection string

const (
	migrationSQLUp   migrationSQLDirection = "up"
	migrationSQLDown migrationSQLDirection = "down"
)

type migrationSQLParserState uint8

const (
	migrationSQLStart migrationSQLParserState = iota
	migrationSQLUpSection
	migrationSQLStatementBeginUp
	migrationSQLStatementEndUp
	migrationSQLDownSection
	migrationSQLStatementBeginDown
	migrationSQLStatementEndDown
)

type migrationSQLAnnotation string

const (
	migrationAnnotationUp             migrationSQLAnnotation = "Up"
	migrationAnnotationDown           migrationSQLAnnotation = "Down"
	migrationAnnotationStatementBegin migrationSQLAnnotation = "StatementBegin"
	migrationAnnotationStatementEnd   migrationSQLAnnotation = "StatementEnd"
	migrationAnnotationNoTransaction  migrationSQLAnnotation = "NO TRANSACTION"
)

type parsedMigrationSQL struct {
	useTx bool
	up    []string
	down  []string
}

type migrationSQLLineAction uint8

const (
	migrationSQLProcessLine migrationSQLLineAction = iota
	migrationSQLSkipLine
	migrationSQLFinishStatement
)

type migrationSQLDirectionParser struct {
	direction  migrationSQLDirection
	state      migrationSQLParserState
	useTx      bool
	statements []string
	statement  strings.Builder
}

func parseMigrationSQL(path string, contents []byte) (parsedMigrationSQL, error) {
	up, useTx, err := parseMigrationSQLDirection(contents, migrationSQLUp)
	if err != nil {
		return parsedMigrationSQL{}, fmt.Errorf("parse %s up migration: %w", path, err)
	}
	down, _, err := parseMigrationSQLDirection(contents, migrationSQLDown)
	if err != nil {
		return parsedMigrationSQL{}, fmt.Errorf("parse %s down migration: %w", path, err)
	}
	return parsedMigrationSQL{useTx: useTx, up: up, down: down}, nil
}

func parseMigrationSQLDirection(
	contents []byte,
	direction migrationSQLDirection,
) ([]string, bool, error) {
	scanner := bufio.NewScanner(bytes.NewReader(contents))
	scanner.Buffer(make([]byte, initialMigrationScanBuffer), maxMigrationScanToken)
	parser := migrationSQLDirectionParser{direction: direction, useTx: true}

	for scanner.Scan() {
		if err := parser.consumeLine(scanner.Text()); err != nil {
			return nil, false, err
		}
	}
	return parser.finish(scanner.Err())
}

func (p *migrationSQLDirectionParser) consumeLine(line string) error {
	trimmed := strings.TrimSpace(line)
	if p.state == migrationSQLStart && trimmed == "" {
		return nil
	}
	action, err := p.consumeAnnotation(line, trimmed)
	if err != nil {
		return err
	}
	if action == migrationSQLSkipLine {
		return nil
	}
	if action == migrationSQLProcessLine &&
		p.statement.Len() == 0 &&
		(strings.HasPrefix(trimmed, "--") || line == "") {
		return nil
	}
	if action == migrationSQLProcessLine {
		p.statement.WriteString(line)
		p.statement.WriteByte('\n')
	}
	matches, err := p.matchesDirection(line)
	if err != nil {
		return err
	}
	if !matches {
		p.statement.Reset()
		p.leaveStatementEnd()
		return nil
	}
	p.completeStatement(line)
	return nil
}

func (p *migrationSQLDirectionParser) consumeAnnotation(
	line string,
	trimmed string,
) (migrationSQLLineAction, error) {
	if !strings.HasPrefix(trimmed, "--") || !strings.Contains(line, "+goose") {
		return migrationSQLProcessLine, nil
	}
	annotation, err := extractMigrationSQLAnnotation(line)
	if err != nil {
		return migrationSQLSkipLine, fmt.Errorf("parse annotation line %q: %w", line, err)
	}
	return p.applyAnnotation(annotation)
}

func (p *migrationSQLDirectionParser) applyAnnotation(
	annotation migrationSQLAnnotation,
) (migrationSQLLineAction, error) {
	switch annotation {
	case migrationAnnotationUp:
		if p.state != migrationSQLStart {
			return migrationSQLSkipLine, fmt.Errorf(
				"duplicate Up annotation in parser state %d",
				p.state,
			)
		}
		p.state = migrationSQLUpSection
	case migrationAnnotationDown:
		if err := p.startDownSection(); err != nil {
			return migrationSQLSkipLine, err
		}
	case migrationAnnotationStatementBegin:
		if err := p.startStatementBlock(); err != nil {
			return migrationSQLSkipLine, err
		}
	case migrationAnnotationStatementEnd:
		if err := p.endStatementBlock(); err != nil {
			return migrationSQLSkipLine, err
		}
		return migrationSQLFinishStatement, nil
	case migrationAnnotationNoTransaction:
		p.useTx = false
	}
	return migrationSQLSkipLine, nil
}

func (p *migrationSQLDirectionParser) startDownSection() error {
	if p.state != migrationSQLUpSection && p.state != migrationSQLStatementEndUp {
		return fmt.Errorf(
			"down annotation before a complete Up section in parser state %d",
			p.state,
		)
	}
	if remaining := strings.TrimSpace(p.statement.String()); remaining != "" {
		return missingMigrationSemicolonError(p.state, p.direction, remaining)
	}
	p.state = migrationSQLDownSection
	return nil
}

func (p *migrationSQLDirectionParser) startStatementBlock() error {
	switch p.state {
	case migrationSQLUpSection, migrationSQLStatementEndUp:
		p.state = migrationSQLStatementBeginUp
	case migrationSQLDownSection, migrationSQLStatementEndDown:
		p.state = migrationSQLStatementBeginDown
	default:
		return fmt.Errorf("StatementBegin annotation in parser state %d", p.state)
	}
	return nil
}

func (p *migrationSQLDirectionParser) endStatementBlock() error {
	switch p.state {
	case migrationSQLStatementBeginUp:
		p.state = migrationSQLStatementEndUp
	case migrationSQLStatementBeginDown:
		p.state = migrationSQLStatementEndDown
	default:
		return fmt.Errorf("StatementEnd annotation in parser state %d", p.state)
	}
	return nil
}

func (p *migrationSQLDirectionParser) matchesDirection(line string) (bool, error) {
	switch p.state {
	case migrationSQLUpSection, migrationSQLStatementBeginUp, migrationSQLStatementEndUp:
		return p.direction == migrationSQLUp, nil
	case migrationSQLDownSection, migrationSQLStatementBeginDown, migrationSQLStatementEndDown:
		return p.direction == migrationSQLDown, nil
	default:
		return false, fmt.Errorf("unexpected parser state %d on line %q", p.state, line)
	}
}

func (p *migrationSQLDirectionParser) completeStatement(line string) {
	switch p.state {
	case migrationSQLUpSection, migrationSQLDownSection:
		if migrationLineEndsStatement(line) {
			p.statements = append(p.statements, strings.TrimSpace(p.statement.String()))
			p.statement.Reset()
		}
	case migrationSQLStatementEndUp, migrationSQLStatementEndDown:
		p.statements = append(p.statements, strings.TrimSpace(p.statement.String()))
		p.statement.Reset()
		p.leaveStatementEnd()
	}
}

func (p *migrationSQLDirectionParser) leaveStatementEnd() {
	switch p.state {
	case migrationSQLStatementEndUp:
		p.state = migrationSQLUpSection
	case migrationSQLStatementEndDown:
		p.state = migrationSQLDownSection
	}
}

func (p *migrationSQLDirectionParser) finish(scanErr error) ([]string, bool, error) {
	if scanErr != nil {
		return nil, false, fmt.Errorf("scan migration: %w", scanErr)
	}
	switch p.state {
	case migrationSQLStart:
		return nil, false, errors.New("migration must start with an Up annotation")
	case migrationSQLStatementBeginUp, migrationSQLStatementBeginDown:
		return nil, false, errors.New("migration has a StatementBegin annotation without StatementEnd")
	}
	if remaining := strings.TrimSpace(p.statement.String()); remaining != "" {
		return nil, false, missingMigrationSemicolonError(p.state, p.direction, remaining)
	}
	return p.statements, p.useTx, nil
}

func extractMigrationSQLAnnotation(line string) (migrationSQLAnnotation, error) {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return "", errors.New("annotation contains leading whitespace")
	}
	command := strings.ReplaceAll(line, "--", "")
	command = strings.Replace(command, "+goose", "", 1)
	if strings.Contains(command, "+goose") {
		return "", errors.New("annotation contains multiple +goose markers")
	}
	command = strings.TrimSpace(command)
	for _, annotation := range []migrationSQLAnnotation{
		migrationAnnotationUp,
		migrationAnnotationDown,
		migrationAnnotationStatementBegin,
		migrationAnnotationStatementEnd,
		migrationAnnotationNoTransaction,
	} {
		if strings.EqualFold(command, string(annotation)) {
			return annotation, nil
		}
	}
	return "", fmt.Errorf("unsupported annotation %q", command)
}

func migrationLineEndsStatement(line string) bool {
	lastWord := ""
	for word := range strings.FieldsSeq(line) {
		if strings.HasPrefix(word, "--") {
			break
		}
		lastWord = word
	}
	return strings.HasSuffix(lastWord, ";")
}

func missingMigrationSemicolonError(
	state migrationSQLParserState,
	direction migrationSQLDirection,
	statement string,
) error {
	return fmt.Errorf(
		"parser state %d direction %s has unfinished SQL %q: missing semicolon",
		state,
		direction,
		statement,
	)
}
