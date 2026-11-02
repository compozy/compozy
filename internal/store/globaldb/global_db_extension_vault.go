package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/vault"
)

// ListExtensionVaultMetadata filters instance-qualified ownership before materializing metadata.
// The existing ref index bounds the global and workspace namespaces; only matching owners are returned.
func (r *VaultRepo) ListExtensionVaultMetadata(ctx context.Context, name string) ([]vault.Metadata, error) {
	if err := r.checkReady(ctx, "list extension vault metadata"); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("store: extension secret owner is required")
	}
	prefix := vault.ExtensionSecretOwnerPrefix(name, "")
	segment := strings.TrimSuffix(strings.TrimPrefix(prefix, "vault:extensions/global/"), "/")
	rows, err := r.queries.ListExtensionVaultMetadata(ctx, sqlcgen.ListExtensionVaultMetadataParams{
		GlobalPrefix: prefix, GlobalEnd: vaultPrefixRangeEnd(prefix), OwnerSegment: segment,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list extension vault metadata: %w", err)
	}
	result := make([]vault.Metadata, 0, len(rows))
	for _, row := range rows {
		created, err := store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		updated, err := store.ParseTimestamp(row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, vault.Metadata{Ref: row.Ref, Kind: row.Kind, Present: row.Present,
			CreatedAt: created, UpdatedAt: updated})
	}
	return result, nil
}
