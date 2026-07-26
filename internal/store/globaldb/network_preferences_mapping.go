package globaldb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

func normalizeNetworkSubscriptionEntry(entry store.NetworkSubscriptionEntry) store.NetworkSubscriptionEntry {
	normalized := store.NetworkSubscriptionEntry{
		WorkspaceID: strings.TrimSpace(entry.WorkspaceID), Channel: strings.TrimSpace(entry.Channel),
		ThreadID: strings.TrimSpace(entry.ThreadID), SessionID: strings.TrimSpace(entry.SessionID),
		Mode: strings.TrimSpace(entry.Mode), CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
	}
	return normalized
}

func normalizeNetworkTaskThreadOrigin(origin store.NetworkTaskThreadOrigin) store.NetworkTaskThreadOrigin {
	return store.NetworkTaskThreadOrigin{
		TaskID: strings.TrimSpace(origin.TaskID), WorkspaceID: strings.TrimSpace(origin.WorkspaceID),
		Channel: strings.TrimSpace(origin.Channel), ThreadID: strings.TrimSpace(origin.ThreadID),
		OriginMessageID: strings.TrimSpace(origin.OriginMessageID), Digest: strings.TrimSpace(origin.Digest),
		SourceMessageIDs: normalizeStringSlice(origin.SourceMessageIDs),
		CreatedAt:        origin.CreatedAt, UpdatedAt: origin.UpdatedAt,
	}
}

func scanNetworkSubscription(scanner rowScanner) (store.NetworkSubscriptionEntry, error) {
	var entry store.NetworkSubscriptionEntry
	var createdAtRaw, updatedAtRaw string
	if err := scanner.Scan(
		&entry.WorkspaceID, &entry.Channel, &entry.ThreadID, &entry.SessionID, &entry.Mode,
		&createdAtRaw, &updatedAtRaw,
	); err != nil {
		return store.NetworkSubscriptionEntry{}, fmt.Errorf("store: scan network subscription: %w", err)
	}
	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return store.NetworkSubscriptionEntry{}, fmt.Errorf("store: parse network subscription created_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return store.NetworkSubscriptionEntry{}, fmt.Errorf("store: parse network subscription updated_at: %w", err)
	}
	entry.CreatedAt = createdAt
	entry.UpdatedAt = updatedAt
	return entry, nil
}

func scanNetworkTaskThreadOrigin(scanner rowScanner) (store.NetworkTaskThreadOrigin, error) {
	var origin store.NetworkTaskThreadOrigin
	var sourceRaw, createdAtRaw, updatedAtRaw string
	if err := scanner.Scan(
		&origin.TaskID, &origin.WorkspaceID, &origin.Channel, &origin.ThreadID,
		&origin.OriginMessageID, &origin.Digest, &sourceRaw, &createdAtRaw, &updatedAtRaw,
	); err != nil {
		return store.NetworkTaskThreadOrigin{}, fmt.Errorf("store: scan network task thread origin: %w", err)
	}
	origin.SourceMessageIDs = stringSliceFromJSON(sourceRaw)
	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return store.NetworkTaskThreadOrigin{}, fmt.Errorf(
			"store: parse network task thread origin created_at: %w",
			err,
		)
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return store.NetworkTaskThreadOrigin{}, fmt.Errorf(
			"store: parse network task thread origin updated_at: %w",
			err,
		)
	}
	origin.CreatedAt = createdAt
	origin.UpdatedAt = updatedAt
	return origin, nil
}

func scanTaskDesignationRollup(scanner rowScanner) (store.TaskDesignationRollup, error) {
	var rollup store.TaskDesignationRollup
	var summaryRaw, createdRaw string
	if err := scanner.Scan(&rollup.DesignationGroupID, &rollup.TaskID, &summaryRaw, &createdRaw); err != nil {
		return store.TaskDesignationRollup{}, fmt.Errorf("store: scan task designation rollup: %w", err)
	}
	rollup.SummaryJSON = json.RawMessage(summaryRaw)
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return store.TaskDesignationRollup{}, fmt.Errorf("store: parse task designation rollup created_at: %w", err)
	}
	rollup.CreatedAt = createdAt
	return rollup, nil
}

func normalizeStringSlice(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func stringSliceJSONString(values []string) string {
	raw, err := json.Marshal(normalizeStringSlice(values))
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func stringSliceFromJSON(raw string) []string {
	var values []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &values); err != nil {
		return nil
	}
	return normalizeStringSlice(values)
}
