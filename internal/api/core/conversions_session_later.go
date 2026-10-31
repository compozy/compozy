package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
)

func sessionLaterPayload(later *store.SessionLater) *contract.SessionLaterPayload {
	if later == nil {
		return nil
	}
	return &contract.SessionLaterPayload{
		ParkedAt:        later.ParkedAt,
		Reason:          later.Reason,
		ReturnAt:        cloneTimePtr(later.ReturnAt),
		PendingMessages: later.PendingMessages,
	}
}
