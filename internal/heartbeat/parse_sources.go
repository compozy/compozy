package heartbeat

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/compozy/compozy/internal/heartbeat/source"
)

func sourceDiagnosticCode(err error) string {
	for _, code := range []string{
		"heartbeat_action_invalid", "heartbeat_action_duplicate_id", "heartbeat_action_limit_exceeded",
	} {
		if strings.HasPrefix(err.Error(), code) {
			return code
		}
	}
	if strings.HasPrefix(err.Error(), "heartbeat_check_ins_invalid:") {
		return "heartbeat_check_ins_invalid"
	}
	if e, ok := errors.AsType[*source.Error](err); ok {
		return string(e.Code)
	}
	return heartbeatHeartbeatInvalidFieldTypeKey
}
func parseSources(value any) ([]SourceDecl, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, source.Failure("heartbeat_source_ref_invalid", "context.sources must be a list")
	}
	refs := make([]SourceDecl, 0, len(items))
	seen := map[string]bool{}
	for index, item := range items {
		data, err := json.Marshal(item, json.Deterministic(true))
		if err != nil {
			return nil, err
		}
		var ref SourceDecl
		if err := json.Unmarshal(data, &ref, json.RejectUnknownMembers(true)); err != nil {
			id := ""
			if fields, ok := item.(map[string]any); ok {
				if value, ok := fields["id"].(string); ok {
					id = value
				}
			}
			return nil, &sourceDeclError{
				index: index,
				err: source.Failure(
					"heartbeat_source_ref_invalid",
					fmt.Sprintf("source %q: invalid source declaration", id),
				),
			}
		}
		if err := ref.Validate(); err != nil {
			return nil, &sourceDeclError{index: index, err: err}
		}
		if seen[ref.ID] {
			return nil, &sourceDeclError{
				index: index,
				err: source.Failure(
					"heartbeat_source_duplicate_id",
					fmt.Sprintf("source %q: source id is declared twice", ref.ID),
				),
			}
		}
		seen[ref.ID] = true
		refs = append(refs, ref)
	}
	return refs, nil
}

type sourceDeclError struct {
	index int
	err   error
}

func (e *sourceDeclError) Error() string { return e.err.Error() }
func (e *sourceDeclError) Unwrap() error { return e.err }
func sourceEntryLocation(metadata []byte, index int) (int, int) {
	file, err := parser.ParseBytes(metadata, 0)
	if err != nil {
		return frontmatterKeyLocation(metadata, "sources")
	}
	path, err := yaml.PathString(fmt.Sprintf("$.context.sources[%d]", index))
	if err != nil {
		return frontmatterKeyLocation(metadata, "sources")
	}
	node, err := path.FilterFile(file)
	if err != nil || node == nil {
		return frontmatterKeyLocation(metadata, "sources")
	}
	switch mapping := node.(type) {
	case *ast.MappingValueNode:
		node = mapping.Key
	case *ast.MappingNode:
		if len(mapping.Values) > 0 {
			node = mapping.Values[0].Key
		}
	}
	pos := node.GetToken().Position
	return pos.Line + 1, pos.Column
}

func sourceLimitDiagnostic(metadata []byte, refs []SourceDecl, index int, path string) Diagnostic {
	line, column := sourceEntryLocation(metadata, index)
	return Diagnostic{
		Code:       "heartbeat_source_limit_exceeded",
		Severity:   diagnosticError,
		Message:    fmt.Sprintf("source %q: too many sources", refs[index].ID),
		SourcePath: path,
		Line:       line,
		Column:     column,
	}
}
