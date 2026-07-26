package clientstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

var sequenceKey = []byte("__meta.seq")

const stagedPurgeBucketPrefix = "__purge:ws:"

type stateStore struct {
	db *bolt.DB
}

func openStateStore(path string, timeout time.Duration) (*stateStore, error) {
	trimmedPath := filepath.Clean(path)
	if trimmedPath == "." {
		return nil, errors.New("clientstate: database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(trimmedPath), 0o700); err != nil {
		return nil, fmt.Errorf("clientstate: create database directory: %w", err)
	}
	db, err := bolt.Open(trimmedPath, 0o600, &bolt.Options{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("clientstate: open database %q: %w", trimmedPath, err)
	}
	if err := os.Chmod(trimmedPath, 0o600); err != nil {
		closeErr := db.Close()
		return nil, errors.Join(
			fmt.Errorf("clientstate: set database permissions %q: %w", trimmedPath, err),
			wrapCloseError(closeErr),
		)
	}
	return &stateStore{db: db}, nil
}

func wrapCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("clientstate: close database after open failure: %w", err)
}

func (s *stateStore) close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("clientstate: close database: %w", err)
	}
	return nil
}

func (s *stateStore) get(ctx context.Context, ws WorkspaceID, domain, key string) (Entry, error) {
	if err := contextError(ctx); err != nil {
		return Entry{}, err
	}
	if err := validateDomain(domain); err != nil {
		return Entry{}, err
	}
	if err := validateKey(key); err != nil {
		return Entry{}, err
	}
	var entry Entry
	err := s.db.View(func(tx *bolt.Tx) error {
		workspaceBucket := tx.Bucket(workspaceBucketName(ws))
		if workspaceBucket == nil {
			return ErrNotFound
		}
		domainBucket := workspaceBucket.Bucket([]byte(domain))
		if domainBucket == nil {
			return ErrNotFound
		}
		encoded := domainBucket.Get([]byte(key))
		if encoded == nil {
			return ErrNotFound
		}
		decoded, err := decodeEntry(domain, key, encoded)
		if err != nil {
			return err
		}
		if decoded.Deleted {
			return ErrNotFound
		}
		entry = decoded
		return nil
	})
	if err != nil {
		return Entry{}, fmt.Errorf("clientstate: get %s/%s: %w", domain, key, err)
	}
	return entry, nil
}

func (s *stateStore) list(ctx context.Context, ws WorkspaceID, domain string) ([]Entry, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := validateDomain(domain); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		workspaceBucket := tx.Bucket(workspaceBucketName(ws))
		if workspaceBucket == nil {
			return nil
		}
		return appendDomainEntries(workspaceBucket.Bucket([]byte(domain)), domain, &entries)
	})
	if err != nil {
		return nil, fmt.Errorf("clientstate: list domain %q: %w", domain, err)
	}
	sortEntries(entries)
	return entries, nil
}

func (s *stateStore) snapshot(
	ctx context.Context,
	ws WorkspaceID,
	domains map[string]struct{},
) (uint64, []Entry, error) {
	if err := contextError(ctx); err != nil {
		return 0, nil, err
	}
	entries := make([]Entry, 0)
	var sequence uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		workspaceBucket := tx.Bucket(workspaceBucketName(ws))
		if workspaceBucket == nil {
			return nil
		}
		var err error
		sequence, err = decodeSequence(workspaceBucket.Get(sequenceKey))
		if err != nil {
			return err
		}
		return workspaceBucket.ForEach(func(name, value []byte) error {
			if value != nil {
				return nil
			}
			domain := string(name)
			if len(domains) > 0 {
				if _, ok := domains[domain]; !ok {
					return nil
				}
			}
			return appendDomainEntries(workspaceBucket.Bucket(name), domain, &entries)
		})
	})
	if err != nil {
		return 0, nil, fmt.Errorf("clientstate: snapshot workspace %q: %w", ws, err)
	}
	sortEntries(entries)
	return sequence, entries, nil
}

func appendDomainEntries(bucket *bolt.Bucket, domain string, entries *[]Entry) error {
	if bucket == nil {
		return nil
	}
	return bucket.ForEach(func(key, encoded []byte) error {
		if encoded == nil {
			return fmt.Errorf("clientstate: nested bucket in domain %q is invalid", domain)
		}
		entry, err := decodeEntry(domain, string(key), encoded)
		if err != nil {
			return err
		}
		if !entry.Deleted {
			*entries = append(*entries, entry)
		}
		return nil
	})
}

func (s *stateStore) apply(
	ctx context.Context,
	ws WorkspaceID,
	domain string,
	ops []Op,
	limits Limits,
	now time.Time,
) ([]Entry, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := validateDomain(domain); err != nil {
		return nil, err
	}
	if err := validateOps(ops, limits); err != nil {
		return nil, err
	}
	results := make([]Entry, 0, len(ops))
	err := s.db.Update(func(tx *bolt.Tx) error {
		if err := contextError(ctx); err != nil {
			return err
		}
		transaction, err := beginApplyTransaction(tx, ws, domain, limits, now)
		if err != nil {
			return err
		}
		for index, op := range ops {
			if err := transaction.apply(index, op); err != nil {
				return err
			}
		}
		if err := transaction.writeSequence(); err != nil {
			return err
		}
		results = cloneEntries(transaction.results)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("clientstate: apply workspace %q domain %q: %w", ws, domain, err)
	}
	return cloneEntries(results), nil
}

func (s *stateStore) stagePurge(ctx context.Context, ws WorkspaceID) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := contextError(ctx); err != nil {
			return err
		}
		name := workspaceBucketName(ws)
		stagedName := stagedPurgeBucketName(ws)
		if tx.Bucket(stagedName) != nil {
			return nil
		}
		workspaceBucket := tx.Bucket(name)
		if workspaceBucket == nil {
			return nil
		}
		stagedBucket, err := tx.CreateBucket(stagedName)
		if err != nil {
			return fmt.Errorf("create staged purge bucket: %w", err)
		}
		if err := copyBucket(workspaceBucket, stagedBucket); err != nil {
			return err
		}
		return tx.DeleteBucket(name)
	}); err != nil {
		return fmt.Errorf("clientstate: stage purge workspace %q: %w", ws, err)
	}
	return nil
}

func (s *stateStore) commitPurge(ctx context.Context, ws WorkspaceID) error {
	return s.deleteBucket(ctx, stagedPurgeBucketName(ws), "commit purge", ws)
}

func (s *stateStore) rollbackPurge(ctx context.Context, ws WorkspaceID) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := contextError(ctx); err != nil {
			return err
		}
		stagedName := stagedPurgeBucketName(ws)
		stagedBucket := tx.Bucket(stagedName)
		if stagedBucket == nil {
			return nil
		}
		name := workspaceBucketName(ws)
		if tx.Bucket(name) != nil {
			return errors.New("live workspace bucket already exists")
		}
		workspaceBucket, err := tx.CreateBucket(name)
		if err != nil {
			return fmt.Errorf("restore workspace bucket: %w", err)
		}
		if err := copyBucket(stagedBucket, workspaceBucket); err != nil {
			return err
		}
		return tx.DeleteBucket(stagedName)
	}); err != nil {
		return fmt.Errorf("clientstate: roll back purge workspace %q: %w", ws, err)
	}
	return nil
}

func (s *stateStore) stagedPurges(ctx context.Context) ([]WorkspaceID, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	workspaces := make([]WorkspaceID, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			bucketName := string(name)
			workspaceID, staged := strings.CutPrefix(bucketName, stagedPurgeBucketPrefix)
			if staged {
				workspaces = append(workspaces, WorkspaceID(workspaceID))
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("clientstate: list staged workspace purges: %w", err)
	}
	return workspaces, nil
}

func (s *stateStore) deleteBucket(
	ctx context.Context,
	name []byte,
	operation string,
	ws WorkspaceID,
) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := contextError(ctx); err != nil {
			return err
		}
		if tx.Bucket(name) == nil {
			return nil
		}
		return tx.DeleteBucket(name)
	}); err != nil {
		return fmt.Errorf("clientstate: %s workspace %q: %w", operation, ws, err)
	}
	return nil
}

func copyBucket(source, target *bolt.Bucket) error {
	return source.ForEach(func(name, value []byte) error {
		if value != nil {
			return target.Put(name, value)
		}
		sourceChild := source.Bucket(name)
		if sourceChild == nil {
			return fmt.Errorf("clientstate: source child bucket %q is missing", name)
		}
		targetChild, err := target.CreateBucket(name)
		if err != nil {
			return fmt.Errorf("clientstate: create target child bucket %q: %w", name, err)
		}
		return copyBucket(sourceChild, targetChild)
	})
}

func workspaceBucketName(ws WorkspaceID) []byte {
	return []byte("ws:" + string(ws))
}

func stagedPurgeBucketName(ws WorkspaceID) []byte {
	return []byte(stagedPurgeBucketPrefix + string(ws))
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("clientstate: context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("clientstate: context: %w", err)
	}
	return nil
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Domain == entries[j].Domain {
			return entries[i].Key < entries[j].Key
		}
		return entries[i].Domain < entries[j].Domain
	})
}
