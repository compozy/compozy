package vault

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
)

const keyringReceiptName = "vault.keyring-ready"
const keyringReceiptVersion = "compozy-vault-keyring-v1\n"

// RequireKeyringMigration checks local opt-in evidence without querying the OS.
// Selecting os-keyring in config alone must never trigger a permission prompt.
func RequireKeyringMigration(home string) error {
	path := filepath.Join(home, keyringReceiptName)
	info, err := os.Lstat(path)
	if err != nil || validateKeyFile(path, info) != nil {
		return errors.New("vault: OS keyring has not been explicitly provisioned; run compozy vault key migrate")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != keyringReceiptVersion {
		return errors.New("vault: invalid OS keyring migration receipt; run compozy vault key migrate")
	}
	return nil
}

func prepareKeyringReceipt(home string) (bool, error) {
	path := filepath.Join(home, keyringReceiptName)
	if _, err := os.Lstat(path); err == nil {
		return false, RequireKeyringMigration(home)
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, fileutil.AtomicWritePrivateFile(path, []byte(keyringReceiptVersion))
}

func removeKeyringReceipt(home string) error {
	err := fileutil.AtomicRemoveFile(filepath.Join(home, keyringReceiptName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func activateKeyringMigration(home string, activate func() error) (err error) {
	created, err := prepareKeyringReceipt(home)
	defer func() {
		if created && err != nil {
			err = errors.Join(err, removeKeyringReceipt(home))
		}
	}()
	if err != nil {
		return err
	}
	return activate()
}
