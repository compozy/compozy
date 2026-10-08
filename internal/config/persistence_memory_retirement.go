package config

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

const (
	retiredMemoryArchiveHeader     = "\n# Archived retired memory and compaction settings; these values are inactive.\n"
	retiredMemoryAutomationKey     = "automation"
	retiredMemoryTriggersKey       = "triggers"
	retiredMemoryConsolidatedEvent = "memory.consolidated"
)

// This boundary archive is removed in v0.6.0.
func archiveRetiredMemorySettings(contents []byte, source string) ([]byte, []string, error) {
	var values map[string]any
	if _, err := burnttoml.Decode(string(contents), &values); err != nil {
		return nil, nil, err
	}
	_, retired, names := splitRetiredMemoryValues(values, nil)
	if len(names) == 0 {
		return contents, nil, nil
	}
	var archive bytes.Buffer
	if err := burnttoml.NewEncoder(&archive).Encode(retired); err != nil {
		return nil, nil, fmt.Errorf("archive retired memory settings: %w", err)
	}
	editor, err := newOverlayEditor(source, contents)
	if err != nil {
		return nil, nil, err
	}
	if err := removeRetiredMemorySettings(editor); err != nil {
		return nil, nil, err
	}
	rendered, err := editor.Bytes()
	if err != nil {
		return nil, nil, err
	}
	result := bytes.NewBuffer(rendered)
	result.WriteString(retiredMemoryArchiveHeader)
	for line := range strings.SplitSeq(strings.TrimRight(archive.String(), "\n"), "\n") {
		result.WriteString("# " + line + "\n")
	}
	slices.Sort(names)
	names = slices.Compact(names)
	return result.Bytes(), names, nil
}

func isRetiredMemoryPath(path []string) bool {
	if len(path) == 0 {
		return false
	}
	if path[0] == "memory" {
		return true
	}
	if len(path) >= 2 {
		retiredRoles := []string{"dream", "checkpoint_summary", "memory_extractor", "memory_controller"}
		if path[0] == "roles" && slices.Contains(retiredRoles, path[1]) {
			return true
		}
		if path[0] == "session" && path[1] == "compaction" {
			return true
		}
	}
	if path[0] != "hooks" || len(path) < 3 {
		return false
	}
	key := path[len(path)-1]
	return path[len(path)-2] == "matcher" && (key == "compaction_reason" || key == "compaction_strategy")
}

func splitRetiredMemoryValues(value any, path []string) (any, any, []string) {
	if isRetiredMemoryPath(path) {
		return nil, value, retiredMemoryTableNames(value, path)
	}
	switch item := value.(type) {
	case map[string]any:
		kept, removed := make(map[string]any), make(map[string]any)
		var names []string
		for key, child := range item {
			childPath := append(clonePath(path), key)
			remaining, retired, childNames := splitRetiredMemoryValues(child, childPath)
			if len(childNames) == 0 {
				kept[key] = child
				continue
			}
			names = append(names, childNames...)
			if remaining != nil {
				kept[key] = remaining
			}
			removed[key] = retired
		}
		return kept, removed, names
	case []map[string]any:
		elements := make([]any, len(item))
		for i := range item {
			elements[i] = item[i]
		}
		return splitRetiredMemoryValues(elements, path)
	case []any:
		var kept, removed []any
		var names []string
		for _, child := range item {
			if pathsEqual(path, []string{retiredMemoryAutomationKey, retiredMemoryTriggersKey}) {
				if entry, ok := child.(map[string]any); ok && entry["event"] == retiredMemoryConsolidatedEvent {
					removed = append(removed, child)
					names = append(names, "automation.triggers")
					continue
				}
			}
			remaining, retired, childNames := splitRetiredMemoryValues(child, path)
			if len(childNames) == 0 {
				kept = append(kept, child)
				continue
			}
			names = append(names, childNames...)
			if remaining != nil {
				kept = append(kept, remaining)
			}
			if original, ok := child.(map[string]any); ok {
				if fragment, ok := retired.(map[string]any); ok {
					if name, exists := original["name"]; exists {
						fragment["name"] = name
					}
				}
			}
			removed = append(removed, retired)
		}
		return kept, removed, names
	default:
		return value, nil, nil
	}
}

func retiredMemoryTableNames(value any, path []string) []string {
	names := []string{strings.Join(path, ".")}
	if table, ok := value.(map[string]any); ok {
		for key, child := range table {
			childPath := append(clonePath(path), key)
			switch nested := child.(type) {
			case map[string]any:
				names = append(names, retiredMemoryTableNames(nested, childPath)...)
			case []map[string]any:
				for _, entry := range nested {
					names = append(names, retiredMemoryTableNames(entry, childPath)...)
				}
			}
		}
	}
	return names
}

type retiredMemoryEdit struct {
	start, end  int
	replacement []byte
}

func removeRetiredMemorySettings(editor *OverlayEditor) error {
	document, err := parseOverlayDocument(editor.content)
	if err != nil {
		return err
	}
	retiredExpressions := make(map[int]bool)
	for _, block := range document.arrayTableBlocks([]string{retiredMemoryAutomationKey, retiredMemoryTriggersKey}) {
		if event, ok := document.blockStringField(block, "event"); ok && event == retiredMemoryConsolidatedEvent {
			for i := block.startIdx; i <= block.endIdx; i++ {
				retiredExpressions[i] = true
			}
		}
	}
	var edits []retiredMemoryEdit
	parser := tomlast.Parser{KeepComments: true}
	parser.Reset(editor.content)
	currentTable := []string{}
	index := 0
	for parser.NextExpression() {
		node := parser.Expression()
		switch node.Kind {
		case tomlast.Table, tomlast.ArrayTable, tomlast.KeyValue, tomlast.Comment:
		default:
			continue
		}
		expr := document.expressions[index]
		retired := retiredExpressions[index]
		index++
		if node.Kind == tomlast.Table || node.Kind == tomlast.ArrayTable {
			currentTable = expr.path
		}
		if node.Kind == tomlast.Comment {
			continue
		}
		if retired || isRetiredMemoryPath(expr.path) {
			edits = append(edits, retiredMemoryEdit{start: rangeStart(expr.raw), end: rangeEnd(expr.raw)})
			continue
		}
		if node.Kind != tomlast.KeyValue {
			continue
		}
		path, err := nodePath(node)
		if err != nil {
			return err
		}
		path = append(clonePath(currentTable), path...)
		nested, err := retiredMemoryInlineEdits(editor.content, node.Value(), path)
		if err != nil {
			return err
		}
		edits = append(edits, nested...)
	}
	if err := parser.Error(); err != nil {
		return err
	}
	slices.SortFunc(edits, func(a, b retiredMemoryEdit) int { return a.start - b.start })
	for _, edit := range slices.Backward(edits) {
		editor.content = replaceOffsets(editor.content, edit.start, edit.end, edit.replacement)
	}
	return nil
}

func retiredMemoryInlineEdits(source []byte, node *tomlast.Node, path []string) ([]retiredMemoryEdit, error) {
	if node.Kind != tomlast.InlineTable && node.Kind != tomlast.Array {
		return nil, nil
	}
	var children []*tomlast.Node
	iterator := node.Children()
	for iterator.Next() {
		children = append(children, iterator.Node())
	}
	var edits []retiredMemoryEdit
	var removed []bool
	for _, child := range children {
		childPath := path
		value := child
		if child.Kind == tomlast.KeyValue {
			key, err := nodePath(child)
			if err != nil {
				return nil, err
			}
			childPath = append(clonePath(path), key...)
			value = child.Value()
		}
		retired := isRetiredMemoryPath(childPath)
		retiredTrigger, err := isRetiredMemoryInlineTrigger(child, path)
		if err != nil {
			return nil, err
		}
		retired = retired || retiredTrigger
		removed = append(removed, retired)
		if retired {
			continue
		}
		nested, err := retiredMemoryInlineEdits(source, value, childPath)
		if err != nil {
			return nil, err
		}
		edits = append(edits, nested...)
	}
	for i := 0; i < len(children); i++ {
		if !removed[i] {
			continue
		}
		first := i
		for i+1 < len(children) && removed[i+1] {
			i++
		}
		start := rangeStart(expressionRange(children[first]))
		end := retiredMemoryNodeEnd(source, children[i])
		switch {
		case i+1 < len(children):
			nextStart := rangeStart(expressionRange(children[i+1]))
			if comma := retiredMemorySeparator(source[end:nextStart]); comma >= 0 {
				end += comma + 1
			}
		case first > 0:
			previousEnd := retiredMemoryNodeEnd(source, children[first-1])
			if comma := retiredMemorySeparator(source[previousEnd:start]); comma >= 0 {
				start = previousEnd + comma
			}
		default:
			if comma := retiredMemorySeparator(source[end:]); comma >= 0 {
				end += comma + 1
			}
		}
		edits = append(edits, retiredMemoryEdit{start: start, end: end})
	}
	return edits, nil
}

func retiredMemoryNodeEnd(source []byte, node *tomlast.Node) int {
	end := rangeEnd(expressionRange(node))
	if node.Kind == tomlast.InlineTable {
		for end < len(source) && (source[end] == ' ' || source[end] == '\t' || source[end] == '\r' || source[end] == '\n') {
			end++
		}
		if end < len(source) && source[end] == '}' {
			end++
		}
	}
	return end
}

func retiredMemorySeparator(source []byte) int {
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case ' ', '\t', '\r', '\n':
			continue
		case '#':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case ',':
			return i
		default:
			return -1
		}
	}
	return -1
}

func isRetiredMemoryInlineTrigger(child *tomlast.Node, path []string) (bool, error) {
	if !pathsEqual(path, []string{retiredMemoryAutomationKey, retiredMemoryTriggersKey}) ||
		child.Kind != tomlast.InlineTable {
		return false, nil
	}
	fields := child.Children()
	for fields.Next() {
		field := fields.Node()
		key, err := nodePath(field)
		if err != nil {
			return false, err
		}
		if pathsEqual(key, []string{"event"}) && string(field.Value().Data) == retiredMemoryConsolidatedEvent {
			return true, nil
		}
	}
	return false, nil
}
