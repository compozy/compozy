package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ callspkg.Store = (*CallRepo)(nil)

func (g *CallRepo) PutContract(ctx context.Context, contract contracts.Contract) error {
	if err := g.checkReady(ctx, "put call contract"); err != nil {
		return err
	}
	if err := putCallContract(ctx, g.db, contract, g.now()); err != nil {
		return err
	}
	return nil
}

func (g *CallRepo) GetContract(ctx context.Context, digest string) (contracts.Contract, error) {
	if err := g.checkReady(ctx, "get call contract"); err != nil {
		return contracts.Contract{}, err
	}
	row, err := g.queries.GetCallContract(ctx, strings.TrimSpace(digest))
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.Contract{}, contracts.ErrContractNotFound
	}
	if err != nil {
		return contracts.Contract{}, fmt.Errorf("store: get call contract %q: %w", digest, err)
	}
	contract := contracts.Contract{Digest: row.Digest, Schema: json.RawMessage(row.Schema)}
	if contracts.OutputRefForPayload(contract.Schema) != contract.Digest {
		return contracts.Contract{}, fmt.Errorf("store: call contract %q failed digest verification", digest)
	}
	return contract, nil
}

func putCallContract(
	ctx context.Context,
	exec taskSQLExecutor,
	contract contracts.Contract,
	at time.Time,
) error {
	if contracts.OutputRefForPayload(contract.Schema) != strings.TrimSpace(contract.Digest) {
		return fmt.Errorf("store: call contract digest does not match schema bytes")
	}
	err := sqlcgen.New(exec).PutCallContract(ctx, sqlcgen.PutCallContractParams{
		Digest: contract.Digest, Schema: string(contract.Schema), CreatedAt: store.FormatTimestamp(at),
	})
	if err != nil {
		return fmt.Errorf("store: put call contract %q: %w", contract.Digest, err)
	}
	return nil
}

func putCallPayload(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID string,
	ref string,
	payload []byte,
	at time.Time,
) error {
	if contracts.OutputRefForPayload(json.RawMessage(payload)) != strings.TrimSpace(ref) {
		return fmt.Errorf("store: call payload digest does not match bytes")
	}
	err := sqlcgen.New(exec).PutCallPayload(ctx, sqlcgen.PutCallPayloadParams{
		WorkspaceID: strings.TrimSpace(workspaceID), Ref: strings.TrimSpace(ref),
		Bytes: append([]byte(nil), payload...), ByteSize: int64(len(payload)),
		CreatedAt: store.FormatTimestamp(at), LastUsedAt: store.FormatTimestamp(at),
	})
	if err != nil {
		return fmt.Errorf("store: put call payload %q: %w", ref, err)
	}
	return nil
}

func (g *CallRepo) GetCall(
	ctx context.Context,
	scope callspkg.CallScope,
	callID string,
) (callspkg.CallRecord, error) {
	if err := g.checkReady(ctx, "get call"); err != nil {
		return callspkg.CallRecord{}, err
	}
	return getCallWithExecutor(ctx, g.db, scope, callID)
}

func (g *CallRepo) GetCallByChild(
	ctx context.Context,
	scope callspkg.CallScope,
	childSessionID string,
) (callspkg.CallRecord, error) {
	if err := g.checkReady(ctx, "get call by child"); err != nil {
		return callspkg.CallRecord{}, err
	}
	row := g.db.QueryRowContext(ctx, `SELECT `+callSelectColumnsSQL+`
		FROM calls WHERE profile_id = ? AND scope = ? AND workspace_id = ?
		AND child_session_id = ? AND state IN ('queued', 'running')
		ORDER BY created_at DESC, call_id DESC LIMIT 1`,
		strings.TrimSpace(scope.ProfileID), string(scope.Scope), strings.TrimSpace(scope.WorkspaceID),
		strings.TrimSpace(childSessionID),
	)
	record, err := scanCallRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.CallRecord{}, callNotFound(childSessionID)
	}
	return record, err
}

func (g *CallRepo) GetCallForSettlement(
	ctx context.Context,
	callID string,
) (callspkg.CallRecord, error) {
	if err := g.checkReady(ctx, "get call for settlement"); err != nil {
		return callspkg.CallRecord{}, err
	}
	return getCallByIDWithExecutor(ctx, g.db, callID)
}

func (g *CallRepo) GetOpenCallForChild(
	ctx context.Context,
	childSessionID string,
) (callspkg.CallRecord, error) {
	if err := g.checkReady(ctx, "get open call for child"); err != nil {
		return callspkg.CallRecord{}, err
	}
	record, err := scanCallRecord(g.db.QueryRowContext(ctx, `SELECT `+callSelectColumnsSQL+`
		FROM calls WHERE child_session_id = ? AND state = 'running'
		ORDER BY created_at DESC, call_id DESC LIMIT 1`, strings.TrimSpace(childSessionID)))
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.CallRecord{}, &callspkg.Error{
			Code:    callspkg.CodeReturnUnbound,
			Message: fmt.Sprintf("session %q has no open call", childSessionID),
		}
	}
	if err != nil {
		return callspkg.CallRecord{}, fmt.Errorf("store: get open call for child %q: %w", childSessionID, err)
	}
	return record, nil
}

func callNotFound(id string) error {
	return &callspkg.Error{Code: callspkg.CodeNotFound, Message: fmt.Sprintf("call %q was not found", id)}
}
