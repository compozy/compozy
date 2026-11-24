package core

import (
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) StreamAccount(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	after := c.GetHeader("Last-Event-ID")
	if after == "" {
		after = c.Query("after_sequence")
	}
	sub, err := h.Account.Subscribe(after)
	if err != nil {
		h.accountResult(c, http.StatusOK, nil, err)
		return
	}
	defer sub.Close()
	serveAccountEvents(c, h, sub.Done, sub.Events, sub.Replay, sub.Eviction, func(event account.Event) SSEMessage {
		return SSEMessage{ID: event.ID, Name: event.Event, Data: event}
	})
}

func (h *BaseHandlers) StreamAccountSignIn(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	events, unsubscribe := h.Account.SubscribeSignIn()
	defer unsubscribe()
	serveAccountEvents(c, h, nil, events, nil, nil, func(event account.SignInEvent) SSEMessage {
		if event.AttemptID == "" {
			return SSEMessage{Name: contract.AccountStateEvent, Data: contract.AccountStatePayload{
				State: event.State, SignedOutReason: optionalAccountString(event.SignedOutReason),
			}}
		}
		return SSEMessage{Name: contract.SignInProgressEvent, Data: contract.AccountSignInProgress{
			AttemptID: event.AttemptID, State: event.State, Code: event.Code,
		}}
	})
}

func serveAccountEvents[T any](
	c *gin.Context, h *BaseHandlers, generationDone <-chan struct{}, updates <-chan T, replay []T,
	final func() *T,
	message func(T) SSEMessage,
) {
	writer, err := PrepareSSE(c)
	if err != nil {
		return
	}
	cancel, shutdown := c.Request.Context().Done(), h.StreamDoneChannel()
	defer func() {
		if final != nil && c.Request.Context().Err() == nil {
			if event := final(); event != nil {
				h.writeSSEBestEffort(writer, message(*event))
			}
		}
	}()
	for _, event := range replay {
		select {
		case <-cancel:
			return
		case <-shutdown:
			return
		case <-generationDone:
			return
		default:
		}
		if err := WriteSSE(writer, message(event)); err != nil {
			return
		}
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-cancel:
			return
		case <-shutdown:
			return
		case <-generationDone:
			return
		case <-heartbeat.C:
			if err := WriteSSEComment(writer, "keepalive"); err != nil {
				return
			}
		case event, ok := <-updates:
			if !ok {
				return
			}
			select {
			case <-generationDone:
				return
			default:
			}
			if err := WriteSSE(writer, message(event)); err != nil {
				return
			}
		}
	}
}
