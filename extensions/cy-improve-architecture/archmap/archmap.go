// Package archmap validates the Architecture Depth Map grammar defined by ADR-007.
// It is a test-only contract guard and has no runtime consumers.
package archmap

import (
	"fmt"
	"path"
	"strings"
	"time"
)

const (
	kindDeep         = "deep"
	kindSeam         = "seam"
	kindAvoid        = "avoid"
	emptyStateMarker = "# No areas audited yet."
)

var canonicalHeader = [...]string{
	"# Architecture Depth Map (active)",
	"# @import'd into agent memory. Route behavior INTO deep modules; do NOT widen seams;",
	"# do NOT re-propose avoided deepenings. Detail: .compozy/arch-reviews/<area>.md",
}

// Map is a parsed architecture depth map.
type Map struct {
	Areas []Area
}

// Area is one audited area section in an architecture depth map.
type Area struct {
	Name    string
	Audited string
	Report  string
	Entries []Entry
}

// Entry is one active deep-module, seam, or avoided-deepening instruction.
type Entry struct {
	Kind   string
	Target string
	Note   string
	Date   string
}

type documentState struct {
	headerIndex    int
	emptyStateSeen bool
}

// ErrorKind classifies an architecture-map grammar violation.
type ErrorKind string

const (
	// ErrorUnknownKind identifies an entry kind outside deep, seam, and avoid.
	ErrorUnknownKind ErrorKind = "unknown_kind"
	// ErrorArity identifies a header or entry with the wrong number of fields.
	ErrorArity ErrorKind = "wrong_arity"
	// ErrorReservedDelimiter identifies a literal pipe inside a field.
	ErrorReservedDelimiter ErrorKind = "reserved_delimiter"
	// ErrorAreaOrder identifies sections that are not strictly ascending by area.
	ErrorAreaOrder ErrorKind = "area_order"
	// ErrorDate identifies a date that is not a valid YYYY-MM-DD value.
	ErrorDate ErrorKind = "date_format"
	// ErrorGroupOrder identifies entries outside deep, seam, avoid group order.
	ErrorGroupOrder ErrorKind = "group_order"
	// ErrorHeader identifies a malformed section header or an entry before a section.
	ErrorHeader ErrorKind = "header_format"
	// ErrorField identifies an empty field or an invalid report path.
	ErrorField ErrorKind = "invalid_field"
)

// ParseError describes one classifiable grammar violation at a source line.
type ParseError struct {
	Kind   ErrorKind
	Line   int
	Detail string
}

// Error returns a message containing the violation class, line, and detail.
func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("architecture map %s at line %d: %s", e.Kind, e.Line, e.Detail)
}

// Parse validates and parses an Architecture Depth Map document.
func Parse(data []byte) (*Map, error) {
	result := &Map{}
	var current *Area
	state := documentState{}
	lastArea := ""
	lastEntryKind := ""

	lines := strings.Split(string(data), "\n")
	for index, rawLine := range lines {
		lineNumber := index + 1
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		skip, err := state.consumeComment(lineNumber, line, len(result.Areas))
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}

		switch {
		case strings.HasPrefix(line, "## "):
			if err := state.validateArea(lineNumber); err != nil {
				return nil, err
			}
			nextArea, nextLastArea, err := addArea(result, lineNumber, line, lastArea)
			if err != nil {
				return nil, err
			}
			current = nextArea
			lastArea = nextLastArea
			lastEntryKind = ""
		case strings.HasPrefix(line, "##"):
			return nil, parseError(lineNumber, ErrorHeader, "section header must start with %q", "## ")
		default:
			if current == nil {
				return nil, parseError(lineNumber, ErrorHeader, "entry appears before an area section")
			}

			entry, err := parseEntry(lineNumber, line)
			if err != nil {
				return nil, err
			}
			if lastEntryKind != "" && entryRank(entry.Kind) < entryRank(lastEntryKind) {
				return nil, parseError(
					lineNumber,
					ErrorGroupOrder,
					"entry kind %q cannot follow %q; expected deep, seam, avoid group order",
					entry.Kind,
					lastEntryKind,
				)
			}

			current.Entries = append(current.Entries, entry)
			lastEntryKind = entry.Kind
		}
	}
	if err := state.finish(len(lines)+1, len(result.Areas)); err != nil {
		return nil, err
	}

	return result, nil
}

func addArea(result *Map, lineNumber int, line string, lastArea string) (*Area, string, error) {
	area, err := parseAreaHeader(lineNumber, line)
	if err != nil {
		return nil, "", err
	}
	if lastArea != "" && area.Name <= lastArea {
		return nil, "", parseError(
			lineNumber,
			ErrorAreaOrder,
			"area %q must sort after %q",
			area.Name,
			lastArea,
		)
	}

	result.Areas = append(result.Areas, area)
	return &result.Areas[len(result.Areas)-1], area.Name, nil
}

func (s *documentState) consumeComment(lineNumber int, line string, areaCount int) (bool, error) {
	if s.headerIndex < len(canonicalHeader) {
		if line != canonicalHeader[s.headerIndex] {
			return false, parseError(
				lineNumber,
				ErrorHeader,
				"document header line %d must be %q",
				s.headerIndex+1,
				canonicalHeader[s.headerIndex],
			)
		}
		s.headerIndex++
		return true, nil
	}
	if isCanonicalHeaderLine(line) {
		return false, parseError(lineNumber, ErrorHeader, "document header must appear exactly once")
	}
	if !strings.HasPrefix(line, "#") || strings.HasPrefix(line, "##") {
		return false, nil
	}
	if line != emptyStateMarker {
		return true, nil
	}
	if areaCount != 0 {
		return false, parseError(lineNumber, ErrorHeader, "empty-state marker requires no area sections")
	}
	if s.emptyStateSeen {
		return false, parseError(lineNumber, ErrorHeader, "empty-state marker must appear exactly once")
	}
	s.emptyStateSeen = true
	return true, nil
}

func (s *documentState) validateArea(lineNumber int) error {
	if s.emptyStateSeen {
		return parseError(lineNumber, ErrorHeader, "area section cannot follow the empty-state marker")
	}
	return nil
}

func (s *documentState) finish(lineNumber int, areaCount int) error {
	if s.headerIndex != len(canonicalHeader) {
		return parseError(lineNumber, ErrorHeader, "document header is incomplete")
	}
	if areaCount == 0 && !s.emptyStateSeen {
		return parseError(lineNumber, ErrorHeader, "empty map requires %q", emptyStateMarker)
	}
	return nil
}

func isCanonicalHeaderLine(line string) bool {
	for _, headerLine := range canonicalHeader {
		if line == headerLine {
			return true
		}
	}
	return false
}

func parseAreaHeader(lineNumber int, line string) (Area, error) {
	fields, err := splitFields(lineNumber, strings.TrimPrefix(line, "## "))
	if err != nil {
		return Area{}, err
	}
	if len(fields) != 3 {
		return Area{}, parseError(lineNumber, ErrorArity, "section header has %d fields; expected 3", len(fields))
	}
	if fields[0] == "" {
		return Area{}, parseError(lineNumber, ErrorField, "area must not be empty")
	}

	audited, ok := strings.CutPrefix(fields[1], "audited ")
	if !ok || audited == "" {
		return Area{}, parseError(lineNumber, ErrorHeader, "second header field must be %q", "audited <YYYY-MM-DD>")
	}
	if err := validateDate(lineNumber, audited); err != nil {
		return Area{}, err
	}

	report, ok := strings.CutPrefix(fields[2], "report ")
	if !ok || report == "" {
		return Area{}, parseError(lineNumber, ErrorHeader, "third header field must be %q", "report <relative-path|->")
	}
	if report != "-" && path.IsAbs(report) {
		return Area{}, parseError(lineNumber, ErrorField, "report path %q must be relative or -", report)
	}

	return Area{
		Name:    fields[0],
		Audited: audited,
		Report:  report,
		Entries: make([]Entry, 0),
	}, nil
}

func parseEntry(lineNumber int, line string) (Entry, error) {
	fields, err := splitFields(lineNumber, line)
	if err != nil {
		return Entry{}, err
	}
	expectedFields := 0
	switch fields[0] {
	case kindDeep, kindSeam:
		expectedFields = 3
	case kindAvoid:
		expectedFields = 4
	default:
		return Entry{}, parseError(lineNumber, ErrorUnknownKind, "unknown entry kind %q", fields[0])
	}
	if len(fields) != expectedFields {
		return Entry{}, parseError(
			lineNumber,
			ErrorArity,
			"%s entry has %d fields; expected %d",
			fields[0],
			len(fields),
			expectedFields,
		)
	}
	for fieldIndex := 1; fieldIndex < len(fields); fieldIndex++ {
		if fields[fieldIndex] == "" {
			return Entry{}, parseError(
				lineNumber,
				ErrorField,
				"%s entry field %d must not be empty",
				fields[0],
				fieldIndex+1,
			)
		}
	}

	if fields[0] == kindAvoid {
		if err := validateDate(lineNumber, fields[1]); err != nil {
			return Entry{}, err
		}
		return Entry{
			Kind:   fields[0],
			Target: fields[2],
			Note:   fields[3],
			Date:   fields[1],
		}, nil
	}

	return Entry{
		Kind:   fields[0],
		Target: fields[1],
		Note:   fields[2],
	}, nil
}

func splitFields(lineNumber int, line string) ([]string, error) {
	rawFields := strings.Split(line, " | ")
	fields := make([]string, len(rawFields))
	for index, field := range rawFields {
		fields[index] = strings.TrimSpace(field)
		if strings.Contains(fields[index], "|") {
			return nil, parseError(
				lineNumber,
				ErrorReservedDelimiter,
				"field %d contains reserved delimiter |; render it as /",
				index+1,
			)
		}
	}
	return fields, nil
}

func validateDate(lineNumber int, value string) error {
	if len(value) != len("YYYY-MM-DD") {
		return parseError(lineNumber, ErrorDate, "date %q must use YYYY-MM-DD", value)
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return parseError(lineNumber, ErrorDate, "date %q must be a valid YYYY-MM-DD date", value)
	}
	return nil
}

func entryRank(kind string) int {
	switch kind {
	case kindDeep:
		return 0
	case kindSeam:
		return 1
	case kindAvoid:
		return 2
	default:
		return -1
	}
}

func parseError(line int, kind ErrorKind, format string, args ...any) error {
	return &ParseError{
		Kind:   kind,
		Line:   line,
		Detail: fmt.Sprintf(format, args...),
	}
}
