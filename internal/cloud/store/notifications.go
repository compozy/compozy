package store

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/remoteproto"
)

type ControlEvent struct {
	Type          remoteproto.ControlType `json:"type"`
	AccountID     string                  `json:"account_id,omitempty"`
	ComputerID    remoteproto.ComputerID  `json:"computer_id,omitempty"`
	SecurityEpoch uint64                  `json:"security_epoch,omitzero"`
	ResourceID    string                  `json:"resource_id,omitempty"`
}

func (e ControlEvent) Valid() bool {
	if len(e.AccountID) > 128 || len(e.ComputerID) > 128 || len(e.ResourceID) > 128 {
		return false
	}
	switch e.Type {
	case remoteproto.ControlSnapshot, remoteproto.ControlRevokeDevice, remoteproto.ControlScopesChanged,
		remoteproto.ControlUnlink:
		return e.AccountID != "" && e.SecurityEpoch > 0
	case remoteproto.ControlApprovalDecided:
		return e.AccountID != "" && e.ComputerID != "" && e.SecurityEpoch > 0 && e.ResourceID != ""
	case remoteproto.ControlJWKS:
		return true
	case remoteproto.ControlPeers:
		return e.AccountID != ""
	default:
		return false
	}
}

// NotifyControlEvent emits a routing hint only after the surrounding transaction commits.
func (q *Queries) NotifyControlEvent(ctx context.Context, event ControlEvent) error {
	if !q.inTx {
		return errors.New("cloud control events require a transaction")
	}
	if !event.Valid() {
		return errors.New("invalid cloud control event")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode cloud control event: %w", err)
	}
	if err := q.generated.NotifyControlEvent(ctx, string(payload)); err != nil {
		return fmt.Errorf("notify cloud control event: %w", err)
	}
	return nil
}
