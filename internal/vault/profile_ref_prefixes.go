package vault

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ListProfileSecretPrefixes includes standalone credentials as well as refs used by live MCP authorization rows.
func ListProfileSecretPrefixes(ctx context.Context, q profileRefSQLQueryer, name string) ([]string, error) {
	rewrites, err := profileRenameRefPrefixes(ctx, q, name, name)
	if err != nil {
		return nil, err
	}
	prefixes := make([]string, 0, len(rewrites))
	for _, rewrite := range rewrites {
		prefixes = append(prefixes, rewrite.old)
	}
	return prefixes, nil
}

// RenameProfileSecretRef translates only references owned by the renamed profile.
func RenameProfileSecretRef(ref, oldName, newName string) string {
	oldPrefix, newPrefix, err := profileRenamePrefixes(oldName, newName)
	if err != nil {
		return ref
	}
	if strings.HasPrefix(ref, oldPrefix) {
		return newPrefix + strings.TrimPrefix(ref, oldPrefix)
	}
	if !strings.HasPrefix(ref, "vault:mcp/") {
		return ref
	}
	if prefix, ok := mcpProfileRefPrefix(ref, oldName, newName); ok {
		return prefix.new + strings.TrimPrefix(ref, prefix.old)
	}
	return ref
}

func profileRenameRefPrefixes(
	ctx context.Context,
	q profileRefSQLQueryer,
	oldName, newName string,
) ([]profileRefPrefix, error) {
	oldPrefix, newPrefix, err := profileRenamePrefixes(oldName, newName)
	if err != nil {
		return nil, err
	}
	prefixes := []profileRefPrefix{{old: oldPrefix, new: newPrefix}}
	oldMCP, newMCP, err := mcpProfileRenamePrefixes(oldName, newName)
	if err != nil {
		return nil, err
	}
	prefixes = append(prefixes, profileRefPrefix{old: oldMCP, new: newMCP})
	queries := make([]string, 0, len(profileRefLocations))
	for _, location := range profileRefLocations {
		// Table and column identifiers come only from the fixed ref-location inventory.
		queries = append(queries, "SELECT "+location.column+" AS ref FROM "+location.table+
			" WHERE "+location.column+" LIKE 'vault:mcp/%'")
	}
	rows, err := q.QueryContext(ctx, strings.Join(queries, " UNION ")+" ORDER BY ref")
	if err != nil {
		return nil, fmt.Errorf("vault: enumerate profile MCP secret owners: %w", err)
	}
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, errors.Join(fmt.Errorf("vault: scan profile MCP secret owner: %w", err), rows.Close())
		}
		if prefix, ok := mcpProfileRefPrefix(ref, strings.TrimSpace(oldName), strings.TrimSpace(newName)); ok {
			prefixes = append(prefixes, prefix)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("vault: finish profile MCP secret owners: %w", err)
	}
	slices.SortFunc(prefixes, func(a, b profileRefPrefix) int { return strings.Compare(a.old, b.old) })
	return slices.Compact(prefixes), nil
}

func mcpProfileRefPrefix(ref, oldName, newName string) (profileRefPrefix, bool) {
	parts := strings.Split(strings.TrimPrefix(ref, "vault:mcp/"), "/")
	scopeIndex := 0
	if len(parts) > 1 && parts[0] == "ext" {
		scopeIndex = 2
	}
	if len(parts) < scopeIndex+4 {
		return profileRefPrefix{}, false
	}
	owner := parts[scopeIndex+1]
	if encoded, ok := strings.CutPrefix(owner, vaultEncodedSegmentPrefix); ok {
		decoded, err := hex.DecodeString(encoded)
		if err != nil {
			return profileRefPrefix{}, false
		}
		owner = string(decoded)
	}
	if parts[scopeIndex+1] != collisionSafeVaultSegment(owner) {
		return profileRefPrefix{}, false
	}
	newOwner := newName
	switch parts[scopeIndex] {
	case MCPProfileScope:
		if owner != oldName {
			return profileRefPrefix{}, false
		}
	case "ws-profile":
		workspace, profile, ok := strings.Cut(owner, "@pf:")
		if !ok || workspace == "" || profile != oldName {
			return profileRefPrefix{}, false
		}
		newOwner = workspace + "@pf:" + newName
	default:
		return profileRefPrefix{}, false
	}
	prefix := "vault:mcp/" + strings.Join(parts[:scopeIndex+2], "/") + "/"
	parts[scopeIndex+1] = collisionSafeVaultSegment(newOwner)
	return profileRefPrefix{
		old: prefix,
		new: "vault:mcp/" + strings.Join(parts[:scopeIndex+2], "/") + "/",
	}, true
}
