package vault

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// MigrateToKeyring imports the existing file key and verifies every ciphertext
// with the key read back from the OS store before activation. The caller owns
// the migration lock and an atomic activation callback, including file removal
// when requested. A failed activation removes only the new keyring entry.
func MigrateToKeyring(
	ctx context.Context,
	home string,
	store Store,
	ring Keyring,
	activate func() error,
) (verified int, err error) {
	if ctx == nil || store == nil || activate == nil {
		return 0, errors.New("vault: migration requires context, store, and activation")
	}
	if ring == nil {
		ring = systemKeyring{}
	}
	account, err := keyringAccount(home)
	if err != nil {
		return 0, err
	}
	source, err := migrationSourceKey(home, ring)
	if err != nil {
		return 0, fmt.Errorf("vault: migration requires the existing vault.key: %w", err)
	}
	defer clear(source)
	created, err := inspectMigrationDestination(ring, account, source)
	if err != nil {
		return 0, err
	}
	activated := false
	defer func() {
		if created && !activated {
			if cleanupErr := ring.Delete(
				keyringService,
				account,
			); cleanupErr != nil &&
				!errors.Is(cleanupErr, keyring.ErrNotFound) {
				err = errors.Join(
					err,
					errors.New(
						"vault: failed to remove uncommitted OS keyring entry; original file key and provider remain authoritative",
					),
				)
			}
		}
	}()
	if created {
		if err := ring.Set(keyringService, account, base64.StdEncoding.EncodeToString(source)); err != nil {
			return 0, errors.New("vault: could not store encryption key in OS keyring")
		}
	}
	readback, err := NewOSKeyProvider(home, ring).Key()
	if err != nil {
		return 0, err
	}
	defer clear(readback)
	if !bytes.Equal(readback, source) {
		return 0, errors.New("vault: OS keyring readback did not match the existing key")
	}
	verified, err = verifyStoredKeys(ctx, store, readback)
	if err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := activateKeyringMigration(home, activate); err != nil {
		return 0, fmt.Errorf("vault: activate OS keyring provider: %w", err)
	}
	activated = true
	return verified, nil
}

func migrationSourceKey(home string, ring Keyring) ([]byte, error) {
	key, err := readKeyFile(filepath.Join(home, "vault.key"))
	if errors.Is(err, os.ErrNotExist) {
		// A repeated migration after explicit file removal uses the existing OS
		// entry and still verifies every stored ciphertext before activation.
		return NewOSKeyProvider(home, ring).Key()
	}
	return key, err
}

func inspectMigrationDestination(ring Keyring, account string, source []byte) (bool, error) {
	existing, err := ring.Get(keyringService, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, errors.New("vault: OS keyring is unavailable or locked; migration did not change the provider")
	}
	stored, decodeErr := decodeKey(existing)
	defer clear(stored)
	if decodeErr != nil || !bytes.Equal(stored, source) {
		return false, errors.New("vault: OS keyring already contains a different key; refusing to overwrite it")
	}
	return false, nil
}

func verifyStoredKeys(ctx context.Context, store Store, key []byte) (int, error) {
	records, err := store.ListVaultSecrets(ctx, "")
	if err != nil {
		return 0, fmt.Errorf("vault: enumerate records for migration: %w", err)
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		identity, err := newCiphertextIdentity(record.Ref, record.Kind)
		if err != nil {
			return 0, err
		}
		if _, err := decryptValue(key, record.EncryptedValue, identity); err != nil {
			return 0, errors.New(
				"vault: an existing record could not be decrypted with the OS keyring key; provider unchanged",
			)
		}
	}
	return len(records), nil
}
