// SPDX-License-Identifier: FSL-1.1-MIT

package backup

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
)

const (
	archivePrefix = "security-events/"
	partsPrefix   = "security-event-parts/"
	manifestExt   = ".manifest.json"
	keyTimeFormat = "20060102T150405.000000Z"
)

type eventBatch struct {
	PreviousThrough time.Time             `json:"previous_through"`
	Through         time.Time             `json:"through"`
	PreviousID      int64                 `json:"previous_id"`
	Events          []store.ArchivedEvent `json:"events"`
	PartCount       int                   `json:"part_count,omitzero"`
	LastEventID     int64                 `json:"last_event_id,omitzero"`
}

func (b eventBatch) lastID() int64 {
	if b.PartCount > 0 {
		return b.LastEventID
	}
	if len(b.Events) == 0 {
		return b.PreviousID
	}
	return b.Events[len(b.Events)-1].ID
}

func ShipSecurityEvents(ctx context.Context, bucket *R2, db *store.Store) (time.Time, error) {
	var at time.Time
	err := db.WithBackupRecoveryLock(ctx, func() error {
		var err error
		at, err = shipSecurityEvents(ctx, bucket, db)
		return err
	})
	return at, err
}

func shipSecurityEvents(ctx context.Context, bucket *R2, db *store.Store) (time.Time, error) {
	keys, err := bucket.archiveKeys(ctx)
	if err != nil {
		return time.Time{}, err
	}
	var previous eventBatch
	if len(keys) != 0 {
		previous, err = bucket.readBatch(ctx, keys[len(keys)-1])
		if err != nil {
			return time.Time{}, err
		}
	}
	at, events, err := db.ReadSecurityArchive(ctx, previous.lastID())
	if err != nil {
		return time.Time{}, err
	}
	if !at.After(previous.Through) {
		return time.Time{}, errors.New("security archive clock did not advance")
	}
	batch := eventBatch{PreviousThrough: previous.Through, Through: at, PreviousID: previous.lastID(), Events: events}
	if err := bucket.writeBatch(ctx, batch); err != nil {
		return time.Time{}, err
	}
	return at, nil
}

func (r *R2) writeBatch(ctx context.Context, batch eventBatch) error {
	key := archiveKey(batch.Through)
	if len(batch.Events) != 0 {
		count, err := r.writeParts(ctx, batch.Through, batch.Events)
		if err != nil {
			return err
		}
		batch.LastEventID, batch.PartCount = batch.lastID(), count
		batch.Events = nil
		key = archivePrefix + batch.Through.UTC().Format(keyTimeFormat) + manifestExt
	}
	payload, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	// Only this final write publishes the coverage and event cursor to readers.
	return r.Write(ctx, key, payload)
}

func (r *R2) writeParts(ctx context.Context, at time.Time, events []store.ArchivedEvent) (int, error) {
	payload := []byte{'['}
	count := 0
	flush := func() error {
		payload = append(payload, ']')
		if err := r.Write(ctx, partKey(at, count), payload); err != nil {
			return err
		}
		count++
		payload = payload[:1]
		return nil
	}
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			return 0, err
		}
		if len(encoded)+2 > maxArchiveSize {
			return 0, fmt.Errorf("security archive event %d exceeds the object limit", event.ID)
		}
		if len(payload) > 1 && len(payload)+len(encoded)+2 > maxArchiveSize {
			if err := flush(); err != nil {
				return 0, err
			}
		}
		if len(payload) > 1 {
			payload = append(payload, ',')
		}
		payload = append(payload, encoded...)
	}
	err := flush()
	return count, err
}

func (r *R2) RestoreEvents(
	ctx context.Context, point, through time.Time,
) (int64, []store.ArchivedEvent, error) {
	keys, err := r.archiveKeys(ctx)
	if err != nil {
		return 0, nil, err
	}
	var events []store.ArchivedEvent
	var lastID int64
	var covered time.Time
	for _, key := range keys {
		at, err := parseArchiveKey(key)
		if err != nil {
			return 0, nil, err
		}
		if !at.After(point) {
			continue
		}
		batch, err := r.readBatch(ctx, key)
		if err != nil {
			return 0, nil, err
		}
		switch {
		case covered.IsZero():
			if batch.PreviousThrough.After(point) {
				return 0, nil, errors.New("security archive is missing the restore point")
			}
		case !batch.PreviousThrough.Equal(covered) || batch.PreviousID != lastID:
			return 0, nil, errors.New("security archive is discontinuous")
		}
		events, err = r.appendBatch(ctx, events, batch)
		if err != nil {
			return 0, nil, err
		}
		covered, lastID = batch.Through, batch.lastID()
		if !covered.Before(through) {
			return lastID, events, nil
		}
	}
	return 0, nil, errors.New("security archive does not reach the requested restore cutoff")
}

func (r *R2) archiveKeys(ctx context.Context) ([]string, error) {
	keys, err := r.List(ctx, archivePrefix)
	if err != nil {
		return nil, err
	}
	slices.Sort(keys)
	return keys, nil
}

func (r *R2) readBatch(ctx context.Context, key string) (eventBatch, error) {
	at, err := parseArchiveKey(key)
	if err != nil {
		return eventBatch{}, err
	}
	body, err := r.Read(ctx, key)
	if err != nil {
		return eventBatch{}, err
	}
	var batch eventBatch
	if err := json.Unmarshal(body, &batch); err != nil || !batch.Through.Equal(at) ||
		!batch.Through.After(batch.PreviousThrough) || batch.PreviousID < 0 {
		return eventBatch{}, errors.New("invalid security archive batch")
	}
	if strings.HasSuffix(key, manifestExt) {
		if len(batch.Events) != 0 || batch.PartCount <= 0 || batch.LastEventID <= batch.PreviousID ||
			int64(batch.PartCount) > batch.LastEventID-batch.PreviousID {
			return eventBatch{}, errors.New("invalid security archive manifest")
		}
	} else if batch.PartCount != 0 || batch.LastEventID != 0 {
		return eventBatch{}, errors.New("invalid inline security archive batch")
	} else if _, err := validateArchiveEvents(batch.PreviousID, batch.Events); err != nil {
		return eventBatch{}, err
	}
	return batch, nil
}

func (r *R2) appendBatch(
	ctx context.Context,
	events []store.ArchivedEvent,
	batch eventBatch,
) ([]store.ArchivedEvent, error) {
	if batch.PartCount == 0 {
		return append(events, batch.Events...), nil
	}
	lastID := batch.PreviousID
	for i := range batch.PartCount {
		raw, err := r.Read(ctx, partKey(batch.Through, i))
		if err != nil {
			return nil, err
		}
		var part []store.ArchivedEvent
		if err := json.Unmarshal(raw, &part); err != nil || len(part) == 0 {
			return nil, errors.New("invalid security archive part")
		}
		lastID, err = validateArchiveEvents(lastID, part)
		if err != nil {
			return nil, err
		}
		events = append(events, part...)
	}
	if lastID != batch.LastEventID {
		return nil, errors.New("security archive parts do not reach the manifest cursor")
	}
	return events, nil
}

func validateArchiveEvents(afterID int64, events []store.ArchivedEvent) (int64, error) {
	for _, event := range events {
		if event.ID <= afterID || event.At.IsZero() || event.Type == "" || event.Actor == "" {
			return 0, errors.New("invalid security archive event")
		}
		afterID = event.ID
	}
	return afterID, nil
}

func parseArchiveKey(key string) (time.Time, error) {
	raw, ok := strings.CutPrefix(key, archivePrefix)
	stamp, validSuffix := strings.CutSuffix(raw, manifestExt)
	if !validSuffix {
		stamp, validSuffix = strings.CutSuffix(raw, ".json")
	}
	if !ok || !validSuffix {
		return time.Time{}, errors.New("unexpected security archive object key")
	}
	at, err := time.Parse(keyTimeFormat, stamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse security archive object timestamp: %w", err)
	}
	return at, nil
}

func archiveKey(at time.Time) string { return archivePrefix + at.UTC().Format(keyTimeFormat) + ".json" }

func partKey(at time.Time, part int) string {
	return fmt.Sprintf("%s%s/%d.json", partsPrefix, at.UTC().Format(keyTimeFormat), part)
}
