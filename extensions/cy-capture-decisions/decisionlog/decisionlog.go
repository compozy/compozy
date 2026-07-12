// Package decisionlog parses and validates the decision-log file-format
// contract used by the cy-capture-decisions extension: the per-decision record
// bodies (.compozy/decisions/AD-NNN.md) and the terse active-proven index
// (.compozy/DECISIONS.md). It ships no runtime behavior — it is a CI/test-only
// asset (ADR-004) that gives `make verify` a deterministic gate over the log
// contract authored in the extension's references/ files.
package decisionlog

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"gopkg.in/yaml.v3"
)

// Status values a decision record may carry (see references/decision-record-template.md).
const (
	statusProven     = "proven"
	statusCandidate  = "candidate"
	statusSuperseded = "superseded"
)

const (
	indexFileName    = "DECISIONS.md"
	decisionsDirName = "decisions"
	recordFileExt    = ".md"
	indexFieldCount  = 6
)

// requiredMetaKeys are the frontmatter keys every AD-NNN.md record must declare.
// Presence is checked against the raw mapping keys, so a `null` or `[]` value
// still counts as present (only an absent key is an error).
var requiredMetaKeys = []string{
	"id", "title", "status", "tags", "source_slug",
	"source_adr", "promoted_at", "supersedes", "superseded_by", "evidence",
}

// requiredBodySection is the one body section that distinguishes a captured
// decision from a raw ADR copy (references/decision-record-template.md → Rules).
const requiredBodySection = "Reconciliation"

var (
	adIDPattern      = regexp.MustCompile(`^AD-\d{3,}$`)
	reconciliationRe = regexp.MustCompile(`(?m)^##\s+` + requiredBodySection + `\s*$`)
)

var (
	errMissingField   = errors.New("required frontmatter field missing")
	errInvalidStatus  = errors.New("invalid status")
	errEvidenceReq    = errors.New("evidence required for proven")
	errMissingSection = errors.New("required body section missing")
	errIndexGrammar   = errors.New("index line grammar violation")
	errIndexNotProven = errors.New("index must contain only active proven decisions")
	errBrokenRef      = errors.New("index references a missing decision body")
	errBrokenLink     = errors.New("supersession link is not bidirectional")
	errSupersedeChain = errors.New("supersession chain does not resolve to a single active head")
	errUnknownRecord  = errors.New("referenced decision record does not exist")
	errDuplicateID    = errors.New("duplicate decision id")
	// errIDFilenameMismatch: a record's frontmatter id must equal its filename stem.
	errIDFilenameMismatch = errors.New("record id does not match filename")
	// errStatusLinkMismatch: a record is superseded iff it names its successor.
	errStatusLinkMismatch = errors.New("status and superseded_by are inconsistent")
	// errIndexBodyMismatch: an index line's denormalized title/source_slug must
	// equal the body it points at (index-format.md: the line is a copy of body fields).
	errIndexBodyMismatch = errors.New("index line disagrees with body")
	// errMissingIndexLine: every active-proven body must carry an index line
	// (index-format.md membership rule is a biconditional: proven+active iff indexed).
	errMissingIndexLine = errors.New("active-proven record absent from index")
)

// DecisionRecordMeta is the YAML frontmatter of a .compozy/decisions/AD-NNN.md
// decision record. Field order and tags mirror the canonical template so a
// hand-authored or skill-produced record round-trips through this validator.
type DecisionRecordMeta struct {
	ID           string   `yaml:"id"`
	Title        string   `yaml:"title"`
	Status       string   `yaml:"status"`
	Tags         []string `yaml:"tags"`
	SourceSlug   string   `yaml:"source_slug"`
	SourceADR    string   `yaml:"source_adr"`
	PromotedAt   string   `yaml:"promoted_at"`
	Supersedes   []string `yaml:"supersedes"`
	SupersededBy string   `yaml:"superseded_by"`
	Evidence     string   `yaml:"evidence"`
}

// indexLine is a parsed DECISIONS.md decision line (six pipe-delimited fields).
type indexLine struct {
	ID         string
	Title      string
	Status     string
	Tags       []string
	Rationale  string
	SourceSlug string
}

// parseDecisionRecord parses an AD-NNN.md body and validates its frontmatter
// schema (required fields, status enum, evidence-required-for-proven) and the
// presence of the distinguishing Reconciliation body section.
func parseDecisionRecord(content string) (DecisionRecordMeta, error) {
	var node yaml.Node
	body, err := frontmatter.Parse(content, &node)
	if err != nil {
		return DecisionRecordMeta{}, fmt.Errorf("parse decision frontmatter: %w", err)
	}
	present := mappingKeys(&node)
	var meta DecisionRecordMeta
	if err := node.Decode(&meta); err != nil {
		return DecisionRecordMeta{}, fmt.Errorf("decode decision frontmatter: %w", err)
	}
	if err := validateMeta(meta, present); err != nil {
		return DecisionRecordMeta{}, err
	}
	if !reconciliationRe.MatchString(body) {
		return DecisionRecordMeta{}, fmt.Errorf("section %q: %w", requiredBodySection, errMissingSection)
	}
	return meta, nil
}

// validateMeta enforces the frontmatter contract on already-decoded metadata.
func validateMeta(meta DecisionRecordMeta, present map[string]struct{}) error {
	for _, key := range requiredMetaKeys {
		if _, ok := present[key]; !ok {
			return fmt.Errorf("field %q: %w", key, errMissingField)
		}
	}
	if !isValidStatus(meta.Status) {
		return fmt.Errorf("status %q: %w", meta.Status, errInvalidStatus)
	}
	if !adIDPattern.MatchString(meta.ID) {
		return fmt.Errorf("id %q: %w", meta.ID, errIndexGrammar)
	}
	if meta.Status == statusProven && strings.TrimSpace(meta.Evidence) == "" {
		return fmt.Errorf("record %s: %w", meta.ID, errEvidenceReq)
	}
	return nil
}

func isValidStatus(status string) bool {
	switch status {
	case statusProven, statusCandidate, statusSuperseded:
		return true
	default:
		return false
	}
}

// mappingKeys returns the set of scalar keys declared in the frontmatter
// mapping node so required-field presence can be checked independently of the
// decoded (zero-able) struct values.
func mappingKeys(node *yaml.Node) map[string]struct{} {
	mapping := node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return map[string]struct{}{}
		}
		mapping = node.Content[0]
	}
	keys := make(map[string]struct{})
	if mapping.Kind != yaml.MappingNode {
		return keys
	}
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		if keyNode.Kind == yaml.ScalarNode {
			keys[keyNode.Value] = struct{}{}
		}
	}
	return keys
}

// parseIndexLine parses a single DECISIONS.md decision line into its six fields
// and validates the grammar (field count, id shape, bracketed tag list).
func parseIndexLine(line string) (indexLine, error) {
	fields := strings.Split(line, "|")
	if len(fields) != indexFieldCount {
		return indexLine{}, fmt.Errorf("line %q has %d fields, want %d: %w",
			line, len(fields), indexFieldCount, errIndexGrammar)
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	id := fields[0]
	if !adIDPattern.MatchString(id) {
		return indexLine{}, fmt.Errorf("line %q id %q: %w", line, id, errIndexGrammar)
	}
	tags, err := parseTagList(fields[3])
	if err != nil {
		return indexLine{}, fmt.Errorf("line %q: %w", line, err)
	}
	return indexLine{
		ID:         id,
		Title:      fields[1],
		Status:     fields[2],
		Tags:       tags,
		Rationale:  fields[4],
		SourceSlug: fields[5],
	}, nil
}

// parseTagList parses the bracketed comma list rendered in the index tag field,
// e.g. "[orders, async]" -> {"orders", "async"} and "[]" -> {}.
func parseTagList(field string) ([]string, error) {
	if !strings.HasPrefix(field, "[") || !strings.HasSuffix(field, "]") {
		return nil, fmt.Errorf("tag field %q not bracketed: %w", field, errIndexGrammar)
	}
	inner := strings.TrimSpace(field[1 : len(field)-1])
	if inner == "" {
		return nil, nil
	}
	parts := strings.Split(inner, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags, nil
}

// validateIndex parses a DECISIONS.md file, skipping header/comment lines, and
// enforces that every decision line is active-proven and that no id is listed
// twice (index-format.md: the set is regenerated whole on each capture, never
// appended, so a duplicate line would double-load a decision). An empty-state
// index (header only, zero decision lines) is valid and yields zero records.
func validateIndex(content string) ([]indexLine, error) {
	var lines []indexLine
	seen := make(map[string]struct{})
	for _, raw := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		line, err := parseIndexLine(trimmed)
		if err != nil {
			return nil, err
		}
		if line.Status != statusProven {
			return nil, fmt.Errorf("line %q is %s: %w", trimmed, line.Status, errIndexNotProven)
		}
		if _, dup := seen[line.ID]; dup {
			return nil, fmt.Errorf("index id %s: %w", line.ID, errDuplicateID)
		}
		seen[line.ID] = struct{}{}
		lines = append(lines, line)
	}
	return lines, nil
}

// validateSupersession verifies that supersession metadata is internally
// consistent: every link is bidirectional (superseded_by <-> supersedes), every
// referenced id exists, and no chain cycles (so each chain resolves to a single
// active head).
func validateSupersession(records []DecisionRecordMeta) error {
	byID := make(map[string]DecisionRecordMeta, len(records))
	for i := range records {
		id := records[i].ID
		if _, dup := byID[id]; dup {
			return fmt.Errorf("id %s: %w", id, errDuplicateID)
		}
		byID[id] = records[i]
	}
	for i := range records {
		if err := checkRecordLinks(records[i], byID); err != nil {
			return err
		}
	}
	return checkNoCycles(records, byID)
}

// checkRecordLinks verifies a record's status/link agreement (superseded iff it
// names a successor) and that its supersession links are bidirectional.
func checkRecordLinks(rec DecisionRecordMeta, byID map[string]DecisionRecordMeta) error {
	if (rec.Status == statusSuperseded) != (rec.SupersededBy != "") {
		return fmt.Errorf("%s: status %q inconsistent with superseded_by %q: %w",
			rec.ID, rec.Status, rec.SupersededBy, errStatusLinkMismatch)
	}
	if rec.SupersededBy != "" {
		successor, ok := byID[rec.SupersededBy]
		if !ok {
			return fmt.Errorf("%s superseded_by %s: %w", rec.ID, rec.SupersededBy, errUnknownRecord)
		}
		if !contains(successor.Supersedes, rec.ID) {
			return fmt.Errorf("%s superseded_by %s but %s does not supersede it: %w",
				rec.ID, rec.SupersededBy, rec.SupersededBy, errBrokenLink)
		}
	}
	for _, oldID := range rec.Supersedes {
		predecessor, ok := byID[oldID]
		if !ok {
			return fmt.Errorf("%s supersedes %s: %w", rec.ID, oldID, errUnknownRecord)
		}
		if predecessor.SupersededBy != rec.ID {
			return fmt.Errorf("%s supersedes %s but %s.superseded_by is %q: %w",
				rec.ID, oldID, oldID, predecessor.SupersededBy, errBrokenLink)
		}
	}
	return nil
}

// checkNoCycles walks each superseded_by chain and fails on a cycle, which would
// leave a chain with no single active head.
func checkNoCycles(records []DecisionRecordMeta, byID map[string]DecisionRecordMeta) error {
	for i := range records {
		seen := make(map[string]struct{}, len(records))
		for cur := records[i]; cur.SupersededBy != ""; {
			if _, loop := seen[cur.ID]; loop {
				return fmt.Errorf("chain through %s: %w", records[i].ID, errSupersedeChain)
			}
			seen[cur.ID] = struct{}{}
			cur = byID[cur.SupersededBy]
		}
	}
	return nil
}

// validateLog validates a full decision log rooted at fsys: the DECISIONS.md
// index, every AD-NNN.md body under decisions/, that each index line resolves to
// an existing active-proven body whose denormalized fields match, that every
// active-proven body is itself indexed (the membership biconditional), and that
// supersession metadata is consistent.
func validateLog(fsys fs.FS) error {
	content, err := fs.ReadFile(fsys, indexFileName)
	if err != nil {
		return fmt.Errorf("read %s: %w", indexFileName, err)
	}
	lines, err := validateIndex(string(content))
	if err != nil {
		return err
	}
	byID, records, err := loadRecords(fsys)
	if err != nil {
		return err
	}
	if err := checkIndexRefs(fsys, lines, byID); err != nil {
		return err
	}
	if err := checkIndexMembership(lines, records); err != nil {
		return err
	}
	return validateSupersession(records)
}

// loadRecords parses every AD-*.md body under decisions/. A missing decisions/
// directory yields zero records (valid for an empty-state log).
func loadRecords(fsys fs.FS) (map[string]DecisionRecordMeta, []DecisionRecordMeta, error) {
	entries, err := fs.ReadDir(fsys, decisionsDirName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]DecisionRecordMeta{}, nil, nil
		}
		return nil, nil, fmt.Errorf("read %s dir: %w", decisionsDirName, err)
	}
	byID := make(map[string]DecisionRecordMeta)
	records := make([]DecisionRecordMeta, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, recordFileExt) {
			continue
		}
		path := decisionsDirName + "/" + name
		body, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", path, err)
		}
		meta, err := parseDecisionRecord(string(body))
		if err != nil {
			return nil, nil, fmt.Errorf("record %s: %w", path, err)
		}
		if stem := strings.TrimSuffix(name, recordFileExt); meta.ID != stem {
			return nil, nil, fmt.Errorf("record %s: id %q does not match filename: %w",
				path, meta.ID, errIDFilenameMismatch)
		}
		byID[meta.ID] = meta
		records = append(records, meta)
	}
	return byID, records, nil
}

// checkIndexRefs verifies every index line points at an existing body file that
// is itself active-proven and whose denormalized title/source_slug match the
// body they copy (defending the index membership and consistency rules against
// the bodies, not just the line text).
func checkIndexRefs(fsys fs.FS, lines []indexLine, byID map[string]DecisionRecordMeta) error {
	for _, line := range lines {
		path := decisionsDirName + "/" + line.ID + recordFileExt
		if _, err := fs.Stat(fsys, path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("index line %s -> %s: %w", line.ID, path, errBrokenRef)
			}
			return fmt.Errorf("stat %s: %w", path, err)
		}
		meta, ok := byID[line.ID]
		if !ok {
			return fmt.Errorf("index line %s: %w", line.ID, errBrokenRef)
		}
		if meta.Status != statusProven || meta.SupersededBy != "" {
			return fmt.Errorf("index line %s references non-active-proven body: %w",
				line.ID, errIndexNotProven)
		}
		if line.Title != meta.Title || line.SourceSlug != meta.SourceSlug {
			return fmt.Errorf("index line %s title/source_slug disagree with body: %w",
				line.ID, errIndexBodyMismatch)
		}
	}
	return nil
}

// checkIndexMembership enforces the reverse half of the membership biconditional
// (index-format.md: include a record iff it is proven and not superseded): every
// active-proven body must carry a matching index line, so a proven decision
// cannot be silently dropped from the loaded index surface.
func checkIndexMembership(lines []indexLine, records []DecisionRecordMeta) error {
	indexed := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		indexed[line.ID] = struct{}{}
	}
	for i := range records {
		rec := records[i]
		if rec.Status != statusProven || rec.SupersededBy != "" {
			continue
		}
		if _, ok := indexed[rec.ID]; !ok {
			return fmt.Errorf("active-proven %s: %w", rec.ID, errMissingIndexLine)
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
