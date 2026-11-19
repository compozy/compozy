package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

const mcpDisabledField = "disabled"

// ResolveMCPEnabledWriteTarget selects the file declaring a scoped MCP server.
func ResolveMCPEnabledWriteTarget(
	home HomePaths, root string, scope WriteScope, profile string, sidecar bool,
) (WriteTarget, error) {
	if scope != WriteScopeProfile || root == "" {
		return resolveWriteTarget(home, root, scope, sidecar, profile)
	}
	if err := ValidateResourceProfileName(profile); err != nil {
		return WriteTarget{}, err
	}
	root, err := resolveWorkspaceRoot(root)
	if err != nil {
		return WriteTarget{}, err
	}
	target := WriteTarget{scope: scope, workspaceRoot: root, profileName: profile}
	if sidecar {
		target.kind, target.path = WriteTargetWorkspaceProfileMCPSidecar, workspaceProfileMCPJSONFile(root, profile)
	} else {
		target.kind, target.path = WriteTargetWorkspaceProfileConfig, workspaceProfileConfigFile(root, profile)
	}
	return target, nil
}

// SetMCPSidecarServerDisabled edits the control field of an existing JSON declaration.
func SetMCPSidecarServerDisabled(
	home HomePaths,
	root string,
	target WriteTarget,
	name string,
	disabled bool,
) (Config, error) {
	return editMCPSidecarServer(home, root, target, true, func(doc *editableMCPJSONDocument) error {
		name = normalizeMCPServerName(name)
		collection := doc.collectionForPut(name)
		actualName, ok := collection.nameIndex[name]
		if !ok {
			return fmt.Errorf("config: MCP server %q has no declaration in this file", name)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(collection.entries[actualName], &fields); err != nil {
			return err
		}
		for key := range fields {
			if strings.EqualFold(key, mcpDisabledField) {
				delete(fields, key)
			}
		}
		fields[mcpDisabledField] = json.RawMessage(strconv.FormatBool(disabled))
		encoded, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		collection.entries[actualName] = encoded
		return nil
	})
}

// SetMCPServerDisabled updates just the control field of an existing TOML declaration.
func (e *OverlayEditor) SetMCPServerDisabled(name string, disabled bool) error {
	doc, err := parseOverlayDocument(e.content)
	if err != nil {
		return err
	}
	path := []string{"mcp_servers", mcpDisabledField}
	value, err := renderBareValue(disabled)
	if err != nil {
		return err
	}
	for _, block := range slices.Backward(doc.arrayTableBlocks(path[:1])) {
		current, ok := doc.blockStringField(block, "name")
		if !ok || normalizeMCPServerName(current) != name {
			continue
		}
		for _, expr := range doc.expressions[block.startIdx+1 : block.endIdx+1] {
			if expr.kind == tomlast.KeyValue && pathsEqual(expr.path, path) {
				e.content = replaceRange(e.content, expr.value, []byte(value))
				return nil
			}
		}
		start := lineEndOffset(e.content, doc.expressions[block.startIdx].raw)
		e.content = insertAtOffset(e.content, start, "disabled = "+value+"\n")
		return nil
	}
	return e.setInlineMCPServerDisabled(name, value)
}

// Update the last matching element, as the config loader merges entries in order.
func (e *OverlayEditor) setInlineMCPServerDisabled(name, value string) error {
	var parser tomlast.Parser
	parser.Reset(e.content)
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind == tomlast.Table || node.Kind == tomlast.ArrayTable {
			break
		}
		if node.Kind != tomlast.KeyValue {
			continue
		}
		path, err := nodePath(node)
		if err != nil {
			return err
		}
		if !pathsEqual(path, []string{"mcp_servers"}) || node.Value().Kind != tomlast.Array {
			continue
		}
		var selected *tomlast.Node
		entries := node.Value().Children()
		for entries.Next() {
			entry := entries.Node()
			if entry.Kind != tomlast.InlineTable {
				continue
			}
			fields := entry.Children()
			for fields.Next() {
				field := fields.Node()
				if field.Kind != tomlast.KeyValue {
					continue
				}
				fieldPath, err := nodePath(field)
				if err != nil {
					return err
				}
				if pathsEqual(fieldPath, []string{"name"}) &&
					normalizeMCPServerName(string(field.Value().Data)) == name {
					selected = entry
					break
				}
			}
		}
		if selected != nil {
			return e.editInlineMCPServerDisabled(selected, value)
		}
	}
	if err := parser.Error(); err != nil {
		return err
	}
	return fmt.Errorf("config: MCP server %q has no declaration in this file", name)
}

func (e *OverlayEditor) editInlineMCPServerDisabled(entry *tomlast.Node, value string) error {
	fields := entry.Children()
	for fields.Next() {
		field := fields.Node()
		if field.Kind != tomlast.KeyValue {
			continue
		}
		path, err := nodePath(field)
		if err != nil {
			return err
		}
		if pathsEqual(path, []string{mcpDisabledField}) {
			e.content = replaceRange(e.content, field.Value().Raw, []byte(value))
			return nil
		}
	}
	offset := rangeEnd(entry.Raw)
	e.content = replaceOffsets(e.content, offset, offset, []byte("disabled = "+value+", "))
	return nil
}
