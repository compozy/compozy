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

var (
	_ callspkg.Store            = (*CallRepo)(nil)
	_ callspkg.MailboxStore     = (*CallRepo)(nil)
	_ callspkg.PayloadStore     = (*CallRepo)(nil)
	_ callspkg.CallListStore    = (*CallRepo)(nil)
	_ callspkg.CallReadStore    = (*CallRepo)(nil)
	_ callspkg.MessageReadStore = (*CallRepo)(nil)
	_ callspkg.PublicationStore = (*CallRepo)(nil)
)

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
	verified, err := verifyCallBlob("contract", contract.Digest, contract.Schema, nil)
	if err != nil {
		return contracts.Contract{}, err
	}
	contract.Schema = json.RawMessage(verified)
	return contract, nil
}

func putCallContract(
	ctx context.Context,
	exec taskSQLExecutor,
	contract contracts.Contract,
	at time.Time,
) error {
	verified, err := verifyCallBlob("contract", contract.Digest, contract.Schema, nil)
	if err != nil {
		return err
	}
	err = sqlcgen.New(exec).PutCallContract(ctx, sqlcgen.PutCallContractParams{
		Digest: strings.TrimSpace(contract.Digest), Schema: string(verified), CreatedAt: store.FormatTimestamp(at),
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
	verified, err := verifyCallBlob("payload", ref, payload, nil)
	if err != nil {
		return err
	}
	err = sqlcgen.New(exec).PutCallPayload(ctx, sqlcgen.PutCallPayloadParams{
		WorkspaceID: strings.TrimSpace(workspaceID), Ref: strings.TrimSpace(ref),
		Bytes: verified, ByteSize: int64(len(verified)),
		CreatedAt: store.FormatTimestamp(at), LastUsedAt: store.FormatTimestamp(at),
	})
	if err != nil {
		return fmt.Errorf("store: put call payload %q: %w", ref, err)
	}
	return nil
}

func verifyCallBlob(kind, ref string, payload []byte, persistedSize *int64) ([]byte, error) {
	ref = strings.TrimSpace(ref)
	if persistedSize != nil && int64(len(payload)) != *persistedSize {
		return nil, fmt.Errorf("store: call %s %q failed byte-size verification", kind, ref)
	}
	if contracts.OutputRefForPayload(json.RawMessage(payload)) != ref {
		return nil, fmt.Errorf("store: call %s %q failed digest verification", kind, ref)
	}
	return append([]byte(nil), payload...), nil
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
