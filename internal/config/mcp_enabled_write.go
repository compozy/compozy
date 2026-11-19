package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

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
	return editMCPSidecarServer(home, root, target, func(doc *editableMCPJSONDocument) error {
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
			if strings.EqualFold(key, "disabled") {
				delete(fields, key)
			}
		}
		fields["disabled"] = json.RawMessage(strconv.FormatBool(disabled))
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
	path := []string{"mcp_servers", "disabled"}
	value, err := renderBareValue(disabled)
	if err != nil {
		return err
	}
	for _, block := range doc.arrayTableBlocks(path[:1]) {
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
	return fmt.Errorf("config: MCP server %q has no declaration in this file", name)
}
