package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	memcontract "github.com/compozy/compozy/internal/memory/contract"
	storepkg "github.com/compozy/compozy/internal/store"
)

type catalogIdentity struct {
	profileID   string
	scope       memcontract.Scope
	workspaceID string
	agentName   string
	agentTier   memcontract.AgentTier
}

func newCatalogIdentity(
	profileID string,
	scope memcontract.Scope,
	workspaceID string,
	agentName string,
	agentTier memcontract.AgentTier,
) catalogIdentity {
	normalizedScope := scope.Normalize()
	identity := catalogIdentity{
		profileID:   strings.TrimSpace(profileID),
		scope:       normalizedScope,
		workspaceID: strings.TrimSpace(workspaceID),
	}
	if normalizedScope == memcontract.ScopeAgent {
		identity.agentName = strings.TrimSpace(agentName)
		identity.agentTier = agentTier.Normalize()
	}
	return identity
}

func (i catalogIdentity) stateKey() string {
	base := fmt.Sprintf("%s::%s", catalogScopeStateKey(i.scope, i.workspaceID), i.profileID)
	if i.scope != memcontract.ScopeAgent {
		return base
	}
	return fmt.Sprintf("%s::%s::%s", base, i.agentName, i.agentTier)
}

func (c *catalog) upsertCatalogIdentityStateTx(
	ctx context.Context,
	tx catalogWriteExecutor,
	identity catalogIdentity,
) error {
	if err := upsertCatalogStateTx(
		ctx,
		tx,
		identity.stateKey(),
		storepkg.FormatTimestamp(c.now().UTC()),
	); err != nil {
		return fmt.Errorf(
			"memory: persist catalog identity state %q/%q/%q/%q/%q: %w",
			identity.profileID,
			identity.scope,
			identity.workspaceID,
			identity.agentName,
			identity.agentTier,
			err,
		)
	}
	return nil
}

func (c *catalog) invalidateCatalogIdentity(ctx context.Context, identity catalogIdentity) error {
	return c.withCatalogWriteTx(ctx, "catalog identity invalidation", func(tx *storepkg.WriteTx) error {
		return invalidateCatalogIdentityTx(ctx, tx, identity)
	})
}

func invalidateCatalogIdentityTx(
	ctx context.Context,
	tx catalogWriteExecutor,
	identity catalogIdentity,
) error {
	if tx == nil {
		return errors.New("catalog transaction is required")
	}
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM memory_catalog_state WHERE key = ?`,
		identity.stateKey(),
	); err != nil {
		return fmt.Errorf("memory: invalidate catalog identity state %q: %w", identity.stateKey(), err)
	}
	return nil
}

func (c *catalog) identityReady(ctx context.Context, identity catalogIdentity) (bool, error) {
	raw, ok, err := c.stateValue(ctx, identity.stateKey())
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if _, err := storepkg.ParseTimestamp(raw); err != nil {
		return false, fmt.Errorf(
			"memory: parse catalog identity state %q timestamp %q: %w",
			identity.stateKey(),
			raw,
			err,
		)
	}
	return true, nil
}

func (c *catalog) scopeReady(
	ctx context.Context,
	profileID string,
	scope memcontract.Scope,
	workspaceID string,
) (bool, error) {
	return c.identityReady(ctx, newCatalogIdentity(profileID, scope, workspaceID, "", ""))
}
