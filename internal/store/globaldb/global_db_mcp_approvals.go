package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *GlobalDB) ListMCPServerApprovals(ctx context.Context, profileID, workspaceID string) ([]store.MCPApproval, error) {
	rows, err := sqlcgen.New(g.readDB).ListMCPServerApprovals(ctx, sqlcgen.ListMCPServerApprovalsParams{ProfileID: profileID, WorkspaceID: workspaceID})
	if err != nil {
		return nil, fmt.Errorf("store: list MCP approvals: %w", err)
	}
	records := make([]store.MCPApproval, 0, len(rows))
	for _, row := range rows {
		approvedAt, err := store.ParseTimestamp(row.ApprovedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, store.MCPApproval{ID: row.ID, ProfileID: row.ProfileID, WorkspaceID: row.WorkspaceID, ServerName: row.ServerName, SourceID: row.SourceID, DefinitionDigest: row.DefinitionDigest, ApprovedAt: approvedAt})
	}
	return records, nil
}

func (g *GlobalDB) PutMCPServerApproval(ctx context.Context, a store.MCPApproval) error {
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		return sqlcgen.New(tx).PutMCPServerApproval(ctx, sqlcgen.PutMCPServerApprovalParams{ID: a.ID, ProfileID: a.ProfileID, WorkspaceID: a.WorkspaceID, ServerName: a.ServerName, SourceID: a.SourceID, DefinitionDigest: a.DefinitionDigest, ApprovedAt: store.FormatTimestamp(a.ApprovedAt)})
	})
}

func (g *GlobalDB) DeleteMCPServerApproval(ctx context.Context, profileID, workspaceID, serverName, sourceID string) error {
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		return sqlcgen.New(tx).DeleteMCPServerApproval(ctx, sqlcgen.DeleteMCPServerApprovalParams{ProfileID: profileID, WorkspaceID: workspaceID, ServerName: serverName, SourceID: sourceID})
	})
}
