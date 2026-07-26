package clientstate

import (
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

type applyTransaction struct {
	workspaceBucket *bolt.Bucket
	domainBucket    *bolt.Bucket
	sequence        uint64
	retainedKeys    int
	limits          Limits
	now             time.Time
	results         []Entry
	domain          string
}

func beginApplyTransaction(
	tx *bolt.Tx,
	ws WorkspaceID,
	domain string,
	limits Limits,
	now time.Time,
) (*applyTransaction, error) {
	workspaceBucket, err := tx.CreateBucketIfNotExists(workspaceBucketName(ws))
	if err != nil {
		return nil, fmt.Errorf("create workspace bucket: %w", err)
	}
	domainBucket, err := workspaceBucket.CreateBucketIfNotExists([]byte(domain))
	if err != nil {
		return nil, fmt.Errorf("create domain bucket: %w", err)
	}
	sequence, err := decodeSequence(workspaceBucket.Get(sequenceKey))
	if err != nil {
		return nil, err
	}
	retainedKeys, err := countRetainedKeys(workspaceBucket)
	if err != nil {
		return nil, err
	}
	return &applyTransaction{
		workspaceBucket: workspaceBucket,
		domainBucket:    domainBucket,
		sequence:        sequence,
		retainedKeys:    retainedKeys,
		limits:          limits,
		now:             now.UTC(),
		results:         make([]Entry, 0),
		domain:          domain,
	}, nil
}

func (a *applyTransaction) apply(index int, op Op) error {
	current, exists, err := a.currentEntry(op.Key)
	if err != nil {
		return err
	}
	if err := a.validateMutation(index, op, current, exists); err != nil {
		return err
	}
	if current.Rev >= maxSafeJSONInteger || a.sequence >= maxSafeJSONInteger {
		return errors.New("clientstate: revision or sequence exhausted")
	}
	if op.Kind == OpPut && !exists {
		a.retainedKeys++
	}
	a.sequence++
	entry := Entry{
		Domain:    a.domain,
		Key:       op.Key,
		Value:     append([]byte(nil), op.Value...),
		Rev:       current.Rev + 1,
		Seq:       a.sequence,
		Deleted:   op.Kind == OpDelete,
		UpdatedAt: a.now,
	}
	if entry.Deleted {
		entry.Value = nil
	}
	encoded, err := encodeEntry(entry)
	if err != nil {
		return err
	}
	if err := a.domainBucket.Put([]byte(op.Key), encoded); err != nil {
		return fmt.Errorf("write key %q: %w", op.Key, err)
	}
	a.results = append(a.results, entry)
	return nil
}

func (a *applyTransaction) currentEntry(key string) (Entry, bool, error) {
	encoded := a.domainBucket.Get([]byte(key))
	if encoded == nil {
		return Entry{}, false, nil
	}
	entry, err := decodeEntry(a.domain, key, encoded)
	if err != nil {
		return Entry{}, false, err
	}
	return entry, true, nil
}

func (a *applyTransaction) validateMutation(
	index int,
	op Op,
	current Entry,
	exists bool,
) error {
	if op.Kind == OpDelete && (!exists || current.Deleted) {
		return fmt.Errorf("op %d key %q: %w", index, op.Key, ErrNotFound)
	}
	if op.IfRev > 0 && current.Rev != op.IfRev {
		return fmt.Errorf(
			"op %d key %q: current rev %d: %w",
			index,
			op.Key,
			current.Rev,
			ErrRevConflict,
		)
	}
	if op.Kind == OpPut && !exists && a.retainedKeys >= a.limits.MaxKeysPerWorkspace {
		return fmt.Errorf(
			"op %d key %q: limit %d: %w",
			index,
			op.Key,
			a.limits.MaxKeysPerWorkspace,
			ErrKeyQuota,
		)
	}
	return nil
}

func (a *applyTransaction) writeSequence() error {
	encoded, err := encodeSequence(a.sequence)
	if err != nil {
		return err
	}
	if err := a.workspaceBucket.Put(sequenceKey, encoded); err != nil {
		return fmt.Errorf("write workspace sequence: %w", err)
	}
	return nil
}

func countRetainedKeys(workspaceBucket *bolt.Bucket) (int, error) {
	count := 0
	err := workspaceBucket.ForEach(func(name, value []byte) error {
		if value != nil {
			return nil
		}
		domainBucket := workspaceBucket.Bucket(name)
		if domainBucket == nil {
			return fmt.Errorf("clientstate: missing domain bucket %q", name)
		}
		return domainBucket.ForEach(func(_, encoded []byte) error {
			if encoded == nil {
				return errors.New("clientstate: nested domain bucket is invalid")
			}
			count++
			return nil
		})
	})
	return count, err
}
