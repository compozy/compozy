package memory

import (
	"database/sql"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
)

type catalogFilter struct {
	scope         memcontract.Scope
	workspaceRoot string
	workspaceID   string
}

func catalogFiltersAllow(filters []catalogFilter, scope memcontract.Scope, workspaceID string) bool {
	if len(filters) == 0 {
		return true
	}
	normalizedScope := scope.Normalize()
	normalizedWorkspaceID := strings.TrimSpace(workspaceID)
	for _, filter := range filters {
		switch filter.scope.Normalize() {
		case memcontract.ScopeGlobal:
			if normalizedScope == "" || normalizedScope == memcontract.ScopeGlobal {
				return true
			}
		case memcontract.ScopeWorkspace:
			if normalizedScope == memcontract.ScopeWorkspace &&
				normalizedWorkspaceID == strings.TrimSpace(filter.workspaceID) {
				return true
			}
		case memcontract.ScopeAgent:
			if normalizedScope == memcontract.ScopeAgent &&
				normalizedWorkspaceID == strings.TrimSpace(filter.workspaceID) {
				return true
			}
		}
	}
	return false
}

func appendCatalogScopeFilter(base string, args []any, scope memcontract.Scope, workspaceID string) (string, []any) {
	switch scope.Normalize() {
	case memcontract.ScopeGlobal:
		return base + "\nAND e.scope = 'global'", args
	case memcontract.ScopeWorkspace:
		return base + "\nAND e.scope = 'workspace' AND e.workspace_id = ?", append(
			args,
			strings.TrimSpace(workspaceID),
		)
	case memcontract.ScopeAgent:
		return base + "\nAND e.scope = 'agent' AND e.workspace_id = ?", append(
			args,
			strings.TrimSpace(workspaceID),
		)
	default:
		trimmedWorkspace := strings.TrimSpace(workspaceID)
		if trimmedWorkspace == "" {
			return base + "\nAND e.scope = 'global'", args
		}
		return base + "\nAND (e.scope = 'global' OR (e.scope = 'workspace' AND e.workspace_id = ?))",
			append(args, trimmedWorkspace)
	}
}

func scanCatalogEntry(scanner interface{ Scan(dest ...any) error }) (catalogDocument, error) {
	var (
		doc          catalogDocument
		scopeRaw     string
		agentTierRaw string
		typeRaw      string
		updatedRaw   string
	)
	if err := scanner.Scan(
		&doc.ID,
		&scopeRaw,
		&doc.WorkspaceID,
		&doc.AgentName,
		&agentTierRaw,
		&doc.Filename,
		&typeRaw,
		&doc.Name,
		&doc.Description,
		&doc.Content,
		&doc.ContentHash,
		&updatedRaw,
	); err != nil {
		return catalogDocument{}, fmt.Errorf("memory: scan catalog entry: %w", err)
	}

	updatedAt, err := storepkg.ParseTimestamp(updatedRaw)
	if err != nil {
		return catalogDocument{}, fmt.Errorf("memory: parse catalog updated_at %q: %w", updatedRaw, err)
	}
	doc.Scope = memcontract.Scope(scopeRaw).Normalize()
	doc.AgentTier = memcontract.AgentTier(agentTierRaw).Normalize()
	doc.Type = memcontract.Type(typeRaw).Normalize()
	doc.UpdatedAt = updatedAt.UTC()
	return doc, nil
}

func scanSearchResult(scanner interface{ Scan(dest ...any) error }) (memcontract.SearchResult, error) {
	var (
		result     memcontract.SearchResult
		scopeRaw   string
		typeRaw    string
		updatedRaw string
		snippet    sql.NullString
	)
	if err := scanner.Scan(
		&scopeRaw,
		&result.Workspace,
		&result.Filename,
		&typeRaw,
		&result.Name,
		&result.Description,
		&updatedRaw,
		&result.Score,
		&snippet,
	); err != nil {
		return memcontract.SearchResult{}, fmt.Errorf("memory: scan search result: %w", err)
	}

	updatedAt, err := storepkg.ParseTimestamp(updatedRaw)
	if err != nil {
		return memcontract.SearchResult{}, fmt.Errorf("memory: parse search result updated_at %q: %w", updatedRaw, err)
	}
	result.Scope = memcontract.Scope(scopeRaw).Normalize()
	result.Type = memcontract.Type(typeRaw).Normalize()
	result.ModTime = updatedAt.UTC()
	if snippet.Valid {
		result.Snippet = cleanSnippet(snippet.String)
	}
	if result.Snippet == "" {
		result.Snippet = result.Description
	}
	return result, nil
}

func scanOperationRecord(scanner interface{ Scan(dest ...any) error }) (memcontract.OperationRecord, error) {
	var (
		record       memcontract.OperationRecord
		id           int64
		operationRaw string
		scopeRaw     sql.NullString
		workspaceID  sql.NullString
		agentName    sql.NullString
		targetID     sql.NullString
		metadataRaw  string
		tsMillis     int64
	)
	if err := scanner.Scan(
		&id,
		&operationRaw,
		&scopeRaw,
		&workspaceID,
		&agentName,
		&targetID,
		&metadataRaw,
		&tsMillis,
	); err != nil {
		return memcontract.OperationRecord{}, fmt.Errorf("memory: scan operation history row: %w", err)
	}
	metadata, err := parseEventMetadata(metadataRaw)
	if err != nil {
		return memcontract.OperationRecord{}, err
	}
	record.ID = fmt.Sprintf("%d", id)
	record.Operation = operationFromEventOp(operationRaw, metadata)
	record.Scope = memcontract.Scope(scopeRaw.String).Normalize()
	record.Workspace = strings.TrimSpace(workspaceID.String)
	record.Filename = firstNonEmpty(metadata[memoryEventMetadataFilenameKey], targetID.String)
	record.AgentName = strings.TrimSpace(agentName.String)
	record.Summary = strings.TrimSpace(metadata[memoryEventMetadataSummaryKey])
	record.Summary = diagnostics.RedactAndBound(record.Summary, maxOperationSummaryBytes)
	record.Timestamp = timeFromUnixMillis(tsMillis)
	return record, nil
}
