package clientstate

import (
	"encoding/binary"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const stateStoreFormatVersion uint32 = 1

var (
	stateStoreMetadataBucket = []byte("__clientstate_meta")
	stateStoreFormatKey      = []byte("format_version")
)

type storeFormatInitialization struct {
	discardedPreContractData bool
}

func initializeStoreFormat(db *bolt.DB) (storeFormatInitialization, error) {
	initialization := storeFormatInitialization{}
	err := db.Update(func(tx *bolt.Tx) error {
		metadata := tx.Bucket(stateStoreMetadataBucket)
		if metadata != nil {
			version, err := decodeStoreFormatVersion(metadata.Get(stateStoreFormatKey))
			if err != nil {
				return err
			}
			switch {
			case version == stateStoreFormatVersion:
				return nil
			case version > stateStoreFormatVersion:
				return &StoreFormatTooNewError{
					Version:          version,
					SupportedVersion: stateStoreFormatVersion,
				}
			}
		}

		bucketNames, err := topLevelBucketNames(tx)
		if err != nil {
			return err
		}
		initialization.discardedPreContractData = len(bucketNames) > 0
		if err := deletePreContractBuckets(tx, bucketNames); err != nil {
			return err
		}
		return writeStoreFormatVersion(tx)
	})
	if err != nil {
		return storeFormatInitialization{}, fmt.Errorf("clientstate: initialize store format: %w", err)
	}
	return initialization, nil
}

func topLevelBucketNames(tx *bolt.Tx) ([][]byte, error) {
	names := make([][]byte, 0)
	if err := tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
		names = append(names, append([]byte(nil), name...))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("clientstate: list store buckets: %w", err)
	}
	return names, nil
}

func deletePreContractBuckets(tx *bolt.Tx, bucketNames [][]byte) error {
	for _, name := range bucketNames {
		if err := tx.DeleteBucket(name); err != nil {
			return fmt.Errorf("clientstate: delete pre-contract bucket %q: %w", name, err)
		}
	}
	return nil
}

func writeStoreFormatVersion(tx *bolt.Tx) error {
	metadata, err := tx.CreateBucket(stateStoreMetadataBucket)
	if err != nil {
		return fmt.Errorf("clientstate: create store metadata: %w", err)
	}
	encoded := make([]byte, 4)
	binary.BigEndian.PutUint32(encoded, stateStoreFormatVersion)
	if err := metadata.Put(stateStoreFormatKey, encoded); err != nil {
		return fmt.Errorf("clientstate: write store format version: %w", err)
	}
	return nil
}

func decodeStoreFormatVersion(encoded []byte) (uint32, error) {
	if len(encoded) != 4 {
		return 0, fmt.Errorf("clientstate: store format version is %d bytes, want 4", len(encoded))
	}
	return binary.BigEndian.Uint32(encoded), nil
}
