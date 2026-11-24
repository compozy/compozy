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
	keyTimeFormat = "20060102T150405.000000Z"
)

type eventBatch struct {
	PreviousThrough time.Time             `json:"previous_through"`
	Through         time.Time             `json:"through"`
	PreviousID      int64                 `json:"previous_id"`
	Events          []store.ArchivedEvent `json:"events"`
}

func (b eventBatch) lastID() int64 {
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
	payload, err := json.Marshal(batch)
	if err != nil || len(payload) > maxArchiveSize {
		return time.Time{}, errors.New("encode security archive: batch exceeds limit or is invalid")
	}
	if err := bucket.Write(ctx, archiveKey(at), payload); err != nil {
		return time.Time{}, err
	}
	return at, nil
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
		events = append(events, batch.Events...)
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
	id := batch.PreviousID
	for _, event := range batch.Events {
		if event.ID <= id || event.At.IsZero() || event.Type == "" || event.Actor == "" {
			return eventBatch{}, errors.New("invalid security archive event")
		}
		id = event.ID
	}
	return batch, nil
}

func parseArchiveKey(key string) (time.Time, error) {
	raw, ok := strings.CutPrefix(key, archivePrefix)
	stamp, suffix := strings.CutSuffix(raw, ".json")
	if !ok || !suffix {
		return time.Time{}, errors.New("unexpected security archive object key")
	}
	at, err := time.Parse(keyTimeFormat, stamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse security archive object timestamp: %w", err)
	}
	return at, nil
}

func archiveKey(at time.Time) string { return archivePrefix + at.UTC().Format(keyTimeFormat) + ".json" }
