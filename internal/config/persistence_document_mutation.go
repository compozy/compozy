package config

import (
	"bytes"

	"slices"

	"strings"

	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

type overlayDocument struct {
	source      []byte
	expressions []overlayExpression
}

type overlayExpression struct {
	kind          tomlast.Kind
	path          []string
	containerPath []string
	raw           tomlast.Range
	value         tomlast.Range
}

type overlayBlock struct {
	path     []string
	startIdx int
	endIdx   int
	start    int
	end      int
}

func setValueInOverlayDocument(source []byte, path []string, value any) ([]byte, error) {
	document, err := parseOverlayDocument(source)
	if err != nil {
		return nil, err
	}

	if existing := document.findKeyValue(path); existing != nil {
		if isUsableValueRange(existing.raw, existing.value) {
			rendered, err := renderBareValue(value)
			if err != nil {
				return nil, err
			}
			return replaceRange(source, existing.value, []byte(rendered)), nil
		}

		relativePath := path
		if len(existing.containerPath) > 0 && len(path) >= len(existing.containerPath) {
			relativePath = path[len(existing.containerPath):]
		}
		line, err := renderKeyValueLine(relativePath, value)
		if err != nil {
			return nil, err
		}
		start, end := lineReplacementOffsets(source, existing.raw)
		return replaceOffsets(source, start, end, []byte(line)), nil
	}

	tablePath := path[:len(path)-1]
	if err := document.ensureNoScalarPrefix(tablePath); err != nil {
		return nil, err
	}
	if document.findTable(path) != nil || len(document.arrayTableBlocks(path)) > 0 {
		return nil, unsupportedTOMLMutation(path, "value path collides with a table")
	}
	if len(document.arrayTableBlocks(tablePath)) > 0 {
		return nil, unsupportedTOMLMutation(path, "array-of-tables require item-level edits")
	}

	if document.findTable(tablePath) != nil {
		insertAt, err := document.tableInsertOffset(tablePath)
		if err != nil {
			return nil, err
		}
		line, err := renderKeyValueLine([]string{path[len(path)-1]}, value)
		if err != nil {
			return nil, err
		}
		return insertAtOffset(source, insertAt, line), nil
	}

	fragment, err := renderTableFragment(tablePath, map[string]any{path[len(path)-1]: value})
	if err != nil {
		return nil, err
	}
	return appendFragment(source, fragment), nil
}

func setTableInOverlayDocument(source []byte, path []string, values map[string]any) ([]byte, error) {
	document, err := parseOverlayDocument(source)
	if err != nil {
		return nil, err
	}
	if err := document.ensureNoScalarPrefix(path); err != nil {
		return nil, err
	}
	if document.findKeyValue(path) != nil || len(document.arrayTableBlocks(path)) > 0 {
		return nil, unsupportedTOMLMutation(path, "table path collides with a scalar or array-of-tables")
	}

	fragment, err := renderTableFragment(path, values)
	if err != nil {
		return nil, err
	}

	block, ok := document.tableBlockIncludingNested(path)
	if !ok {
		return appendFragment(source, fragment), nil
	}
	return replaceOffsets(source, block.start, block.end, []byte(fragment)), nil
}

func upsertArrayTableItemInOverlayDocument(
	source []byte,
	path []string,
	nameField string,
	name string,
	values map[string]any,
) ([]byte, error) {
	document, err := parseOverlayDocument(source)
	if err != nil {
		return nil, err
	}
	if err := document.ensureNoScalarPrefix(path); err != nil {
		return nil, err
	}
	if document.findKeyValue(path) != nil || document.findTable(path) != nil {
		return nil, unsupportedTOMLMutation(path, "array-of-tables path collides with a scalar or table")
	}

	fragment, err := renderArrayTableFragment(path, values)
	if err != nil {
		return nil, err
	}

	blocks := document.arrayTableBlocks(path)
	for _, block := range blocks {
		currentName, ok := document.blockStringField(block, nameField)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(currentName), name) {
			return replaceOffsets(source, block.start, block.end, []byte(fragment)), nil
		}
	}

	if len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		return insertAtOffset(source, last.end, fragment), nil
	}
	return appendFragment(source, fragment), nil
}

func deletePathInOverlayDocument(source []byte, path []string) ([]byte, bool, error) {
	document, err := parseOverlayDocument(source)
	if err != nil {
		return nil, false, err
	}

	if existing := document.findKeyValue(path); existing != nil {
		return replaceOffsets(source, rangeStart(existing.raw), rangeEnd(existing.raw), nil), true, nil
	}
	if block, ok := document.tableBlockIncludingNested(path); ok {
		return replaceOffsets(source, block.start, block.end, nil), true, nil
	}
	blocks := document.descendantTableBlocks(path)
	if len(blocks) > 0 {
		rendered := append([]byte(nil), source...)
		for _, block := range slices.Backward(blocks) {
			rendered = replaceOffsets(rendered, block.start, block.end, nil)
		}
		return rendered, true, nil
	}

	return append([]byte(nil), source...), false, nil
}

func deleteArrayTableItemInOverlayDocument(
	source []byte,
	path []string,
	nameField string,
	name string,
) ([]byte, bool, error) {
	document, err := parseOverlayDocument(source)
	if err != nil {
		return nil, false, err
	}
	blocks := document.arrayTableBlocks(path)
	for _, block := range blocks {
		currentName, ok := document.blockStringField(block, nameField)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(currentName), name) {
			return replaceOffsets(source, block.start, block.end, nil), true, nil
		}
	}
	return append([]byte(nil), source...), false, nil
}

func parseOverlayDocument(source []byte) (*overlayDocument, error) {
	document := &overlayDocument{
		source:      append([]byte(nil), source...),
		expressions: make([]overlayExpression, 0),
	}
	if len(bytes.TrimSpace(source)) == 0 {
		return document, nil
	}

	parser := tomlast.Parser{KeepComments: true}
	parser.Reset(source)

	currentTable := []string{}
	for parser.NextExpression() {
		node := parser.Expression()
		expr := overlayExpression{
			kind:          node.Kind,
			raw:           expressionRange(node),
			containerPath: clonePath(currentTable),
		}

		switch node.Kind {
		case tomlast.Table, tomlast.ArrayTable:
			path, err := nodePath(node)
			if err != nil {
				return nil, err
			}
			expr.path = path
			expr.containerPath = clonePath(path)
			expr.raw = tableHeaderRange(source, node)
			currentTable = clonePath(path)
		case tomlast.KeyValue:
			path, err := nodePath(node)
			if err != nil {
				return nil, err
			}
			expr.path = append(clonePath(currentTable), path...)
			expr.value = node.Value().Raw
		case tomlast.Comment:
			// Comments remain scoped to the current table.
		default:
			continue
		}

		document.expressions = append(document.expressions, expr)
	}

	if err := parser.Error(); err != nil {
		return nil, err
	}
	return document, nil
}
