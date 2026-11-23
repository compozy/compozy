package control

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/compozy/compozy/internal/cloud/store"
)

type connectDirect func(context.Context) (*pgx.Conn, error)

func (h *Hub) Listen(ctx context.Context, connect connectDirect, logger *slog.Logger) {
	backoff := 100 * time.Millisecond
	for ctx.Err() == nil {
		connected, err := h.listenOnce(ctx, connect)
		if ctx.Err() != nil {
			return
		}
		logger.Warn("Cloud event listener disconnected", "error", err)
		if connected {
			backoff = 100 * time.Millisecond
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			backoff = min(2*backoff, 2*time.Second)
		}
	}
}

func (h *Hub) listenOnce(ctx context.Context, connect connectDirect) (_ bool, retErr error) {
	conn, err := connect(ctx)
	if err != nil {
		return false, err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		retErr = errors.Join(retErr, conn.Close(closeCtx))
	}()
	var schema string
	if err := conn.QueryRow(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
		return false, fmt.Errorf("resolve cloud event schema: %w", err)
	}
	if _, err := conn.Exec(ctx, "LISTEN cloud_events"); err != nil {
		return false, fmt.Errorf("subscribe to cloud events: %w", err)
	}
	h.setConnected(true)
	defer h.setConnected(false)
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return true, fmt.Errorf("receive cloud event: %w", err)
		}
		var payload struct {
			Schema string             `json:"schema"`
			Event  store.ControlEvent `json:"event"`
		}
		if err := json.Unmarshal([]byte(notification.Payload), &payload); err == nil &&
			payload.Schema == schema && payload.Event.Valid() {
			h.publish(payload.Event)
		}
	}
}
