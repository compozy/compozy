package core

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/gin-gonic/gin"
)

const bridgeHealthStreamIDsQueryKey = "bridge_ids"

type bridgeHealthStreamQuery struct {
	scope     bridgeScopeQuery
	bridgeIDs []string
}

// StreamBridgeHealth streams health only for the bounded bridge page requested by ID.
func (h *BaseHandlers) StreamBridgeHealth(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}
	query, err := h.parseBridgeHealthStreamQuery(c)
	if err != nil {
		h.respondError(c, bridgeCatalogQueryErrorStatus(err), err)
		return
	}
	observer, err := h.bridgeCatalogObserver()
	if err != nil {
		h.respondError(c, bridgeCatalogReadErrorStatus(err), err)
		return
	}
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	snapshot, err := h.bridgeHealthStreamSnapshot(c.Request.Context(), bridges, observer, query)
	if err != nil {
		h.writeSSEBestEffort(writer, SSEMessage{
			Name: bridgesErrorKey,
			Data: ErrorPayloadForError(err),
		})
		return
	}
	if err := h.writeBridgeHealthSnapshot(writer, snapshot); err != nil {
		if h.Logger != nil {
			h.Logger.Warn("api: failed to emit initial bridge health snapshot", bridgesErrorKey, err)
		}
		return
	}
	lastSnapshot := snapshot.BridgeHealth
	streamDone := h.StreamDoneChannel()
	if bridgeHealthStreamStopped(c.Request.Context(), streamDone) {
		return
	}

	ticker := time.NewTicker(h.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-streamDone:
			return
		case <-ticker.C:
			if bridgeHealthStreamStopped(c.Request.Context(), streamDone) {
				return
			}
			nextSnapshot, pollErr := h.bridgeHealthStreamSnapshot(c.Request.Context(), bridges, observer, query)
			if pollErr != nil {
				h.writeSSEBestEffort(writer, SSEMessage{
					Name: bridgesErrorKey,
					Data: ErrorPayloadForError(pollErr),
				})
				return
			}
			if reflect.DeepEqual(nextSnapshot.BridgeHealth, lastSnapshot) {
				continue
			}
			if err := h.writeBridgeHealthSnapshot(writer, nextSnapshot); err != nil {
				if h.Logger != nil {
					h.Logger.Warn("api: failed to emit bridge health snapshot", bridgesErrorKey, err)
				}
				return
			}
			lastSnapshot = nextSnapshot.BridgeHealth
		}
	}
}

func (h *BaseHandlers) parseBridgeHealthStreamQuery(c *gin.Context) (bridgeHealthStreamQuery, error) {
	scope, err := h.parseBridgeScopeQuery(c.Request.Context(), c)
	if err != nil {
		return bridgeHealthStreamQuery{}, err
	}
	bridgeIDs, err := normalizeBridgeHealthStreamIDs(c.Query(bridgeHealthStreamIDsQueryKey))
	if err != nil {
		return bridgeHealthStreamQuery{}, fmt.Errorf("%s: %w", h.transportName(), err)
	}
	return bridgeHealthStreamQuery{scope: scope, bridgeIDs: bridgeIDs}, nil
}

func normalizeBridgeHealthStreamIDs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s is required", bridgeHealthStreamIDsQueryKey)
	}
	parts := strings.Split(raw, ",")
	if len(parts) > bridgepkg.MaxBridgeCatalogLimit {
		return nil, fmt.Errorf(
			"%s cannot contain more than %d ids",
			bridgeHealthStreamIDsQueryKey,
			bridgepkg.MaxBridgeCatalogLimit,
		)
	}
	ids := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			return nil, fmt.Errorf("%s contains an empty id", bridgeHealthStreamIDsQueryKey)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (h *BaseHandlers) bridgeHealthStreamSnapshot(
	ctx context.Context,
	bridges BridgeService,
	observer BridgeCatalogObserver,
	query bridgeHealthStreamQuery,
) (contract.BridgeHealthStreamPayload, error) {
	instances, err := bridges.ListInstancesByIDs(ctx, query.bridgeIDs)
	if err != nil {
		return contract.BridgeHealthStreamPayload{}, err
	}
	instances = bridgeHealthStreamInstances(instances, query)
	health, err := bridgeCatalogHealthFromObserver(ctx, observer, instances)
	if err != nil {
		return contract.BridgeHealthStreamPayload{}, err
	}
	if health == nil {
		health = map[string]contract.BridgeHealthPayload{}
	}
	return contract.BridgeHealthStreamPayload{
		GeneratedAt:  h.Now().UTC(),
		BridgeHealth: health,
	}, nil
}

func bridgeHealthStreamInstances(
	instances []bridgepkg.BridgeInstance,
	query bridgeHealthStreamQuery,
) []bridgepkg.BridgeInstance {
	requested := make(map[string]struct{}, len(query.bridgeIDs))
	for _, id := range query.bridgeIDs {
		requested[id] = struct{}{}
	}
	visible := make(map[string]bridgepkg.BridgeInstance, len(query.bridgeIDs))
	for _, instance := range instances {
		id := strings.TrimSpace(instance.ID)
		if _, ok := requested[id]; !ok || !bridgeInstanceMatchesListQuery(instance, query.scope) {
			continue
		}
		visible[id] = instance
	}
	selected := make([]bridgepkg.BridgeInstance, 0, len(visible))
	for _, id := range query.bridgeIDs {
		if instance, ok := visible[id]; ok {
			selected = append(selected, instance)
		}
	}
	return selected
}

func (h *BaseHandlers) writeBridgeHealthSnapshot(
	writer FlushWriter,
	snapshot contract.BridgeHealthStreamPayload,
) error {
	return WriteSSE(writer, SSEMessage{
		ID:   bridgeHealthSnapshotID(snapshot),
		Name: "snapshot",
		Data: snapshot,
	})
}

func bridgeHealthSnapshotID(snapshot contract.BridgeHealthStreamPayload) string {
	timestamp := snapshot.GeneratedAt.UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(snapshot.BridgeHealth)
	if err != nil {
		return timestamp
	}
	hasher := fnv.New64a()
	if _, err := hasher.Write(payload); err != nil {
		return timestamp
	}
	return fmt.Sprintf("%s|%016x", timestamp, hasher.Sum64())
}

func bridgeHealthStreamStopped(ctx context.Context, streamDone <-chan struct{}) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}
	select {
	case <-streamDone:
		return true
	default:
		return false
	}
}
