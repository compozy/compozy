package runtimemcp

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func cursorProjectSlug(root string) string {
	return strings.Trim(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(root, "-"), "-")
}

func (r *reader) applyCursorApprovals(entries map[string]entry) {
	root := cmp.Or(r.context.WorktreeRoot, r.context.GitRoot, r.context.CWD)
	dir := filepath.Join(r.context.homeDir("CURSOR_DATA_DIR", ".cursor"), "projects", cursorProjectSlug(root))
	value, _ := r.load(filepath.Join(dir, "mcp-approvals.json"))
	approved := names(value)
	disabled, _ := r.load(filepath.Join(dir, "mcp-disabled.json"))
	for name, item := range entries {
		item.server.Approval = "pending"
		if key := cursorApprovalKey(root, name, item); key != "" && slices.Contains(approved, key) {
			item.server.Approval = "approved"
		}
		if contains(disabled, name) {
			item.server.EnabledInRuntime = false
		}
		entries[name] = item
	}
}

// Cursor 2026.09.26 hashes JSON.stringify({path: projectRoot, server: parsedServer}).
// Its schema establishes field order; nested header/env order comes from the file.
// Definitions needing process-environment expansion stay pending rather than claiming a grant.
func cursorApprovalKey(root, name string, item entry) string {
	data, _, err := fileutil.ReadRegularFile(item.server.SourcePath, maxFileBytes)
	if err != nil {
		return ""
	}
	var doc struct {
		Servers map[string]map[string]jsontext.Value `json:"mcpServers"`
	}
	if json.Unmarshal(stripJSONComments(data), &doc) != nil {
		return ""
	}
	definition := doc.Servers[name]
	if definition == nil || digest(definition) != item.server.DefinitionDigest {
		return ""
	}
	fields := []string{"type", "command", "args", "env", "cwd"}
	if !has(item.definition, "command") {
		fields = []string{"type", "url", "headers", "auth", "tls", "placement"}
	}
	var server bytes.Buffer
	server.WriteByte('{')
	for _, field := range fields {
		value, ok := definition[field]
		if !ok {
			continue
		}
		if bytes.Contains(value, []byte("${")) {
			return ""
		}
		if server.Len() > 1 {
			server.WriteByte(',')
		}
		key, _ := json.Marshal(field)
		server.Write(key)
		server.WriteByte(':')
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		server.Write(encoded)
	}
	server.WriteByte('}')
	path, _ := json.Marshal(root)
	payload := append([]byte(`{"path":`), path...)
	payload = append(payload, []byte(`,"server":`)...)
	payload = append(payload, server.Bytes()...)
	payload = append(payload, '}')
	sum := sha256.Sum256(payload)
	return name + "-" + hex.EncodeToString(sum[:8])
}
