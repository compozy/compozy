package network

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

type envelopeInput struct {
	workspaceID, channel, from           string
	directIdentityFrom, directIdentityTo string
	surface                              *Surface
	threadID, directID, to               *string
	kind                                 Kind
	mentions                             []string
	body                                 json.RawMessage
	workID, replyTo, traceID             *string
	causationID, id                      *string
	expiresAt                            *int64
	ext                                  ExtensionMap
}

func buildEnvelope(input envelopeInput, now time.Time, maxReplayAge time.Duration) (Envelope, error) {
	id := normalizeOptionalIdentifier(input.id)
	if id == nil {
		id = new(store.NewID("msg"))
	}
	envelope := Envelope{
		Protocol:    ProtocolV0,
		ID:          *id,
		WorkspaceID: input.workspaceID,
		Kind:        Kind(strings.TrimSpace(string(input.kind))),
		Channel:     input.channel,
		Surface:     normalizeOptionalSurface(input.surface),
		ThreadID:    normalizeOptionalIdentifier(input.threadID),
		DirectID:    normalizeOptionalIdentifier(input.directID),
		From:        strings.TrimSpace(input.from),
		To:          normalizeOptionalIdentifier(input.to),
		Mentions:    normalizeEnvelopeMentions(input.mentions),
		WorkID:      normalizeOptionalIdentifier(input.workID),
		ReplyTo:     normalizeOptionalIdentifier(input.replyTo),
		TraceID:     normalizeOptionalIdentifier(input.traceID),
		CausationID: normalizeOptionalIdentifier(input.causationID),
		TS:          now.Unix(),
		ExpiresAt:   cloneInt64Ptr(input.expiresAt),
		Body:        cloneRawMessage(input.body),
		Ext:         cloneExtensionMap(input.ext),
	}
	if isConversationKind(envelope.Kind) && envelope.Surface != nil &&
		*envelope.Surface == SurfaceDirect && envelope.To != nil {
		if input.directIdentityFrom == "" || input.directIdentityTo == "" {
			return Envelope{}, fmt.Errorf("%w: direct session identity is required", ErrInvalidField)
		}
		directID, _, _, err := store.NetworkDirectRoomIdentity(
			envelope.WorkspaceID,
			envelope.Channel,
			input.directIdentityFrom,
			input.directIdentityTo,
		)
		if err != nil {
			return Envelope{}, err
		}
		if envelope.DirectID != nil && strings.TrimSpace(*envelope.DirectID) != directID {
			return Envelope{}, fmt.Errorf(
				"%w: direct_id=%q does not belong to the sender and target sessions",
				ErrDirectRoomCollision,
				strings.TrimSpace(*envelope.DirectID),
			)
		}
		envelope.DirectID = new(directID)
	}
	return NormalizeEnvelope(envelope, ValidateOptions{Now: now, MaxReplayAge: maxReplayAge})
}
