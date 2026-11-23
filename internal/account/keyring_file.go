package account

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/vault"
	"github.com/zalando/go-keyring"
)

const (
	sealedCredentialsFile = "keys.enc"
	credentialSealFile    = "seal"
	sealSize              = 32
	sealPurpose           = "compozy.account/v1"
)

type sealedKeyring struct{ dir string }

var _ vault.Keyring = (*sealedKeyring)(nil)

func (k *sealedKeyring) Get(_, user string) (string, error) {
	dir, err := fileutil.OpenDirectory(k.dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", keyring.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	values, err := k.readAndClose(dir)
	if err != nil {
		return "", err
	}
	value, ok := values[user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (k *sealedKeyring) readAndClose(dir *fileutil.Directory) (map[string]string, error) {
	values, err := readSealedCredentials(dir)
	return values, errors.Join(err, dir.Close())
}

func (k *sealedKeyring) Set(_, user, value string) error { return k.change(user, value) }
func (k *sealedKeyring) Delete(_, user string) error     { return k.change(user, "") }

func (k *sealedKeyring) change(user, value string) (err error) {
	dir, err := fileutil.OpenOrCreateDirectory(k.dir, 0o700)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dir.Close()) }()
	values, err := readSealedCredentials(dir)
	if err != nil {
		return err
	}
	if value == "" {
		if _, ok := values[user]; !ok {
			return nil
		}
		delete(values, user)
	} else {
		values[user] = value
	}
	seal, err := credentialSeal(dir, true)
	if err != nil {
		return err
	}
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	// #nosec G407 -- NewGCMWithRandomNonce prepends a randomly generated nonce.
	return dir.AtomicWriteFile(sealedCredentialsFile, seal.Seal(nil, nil, data, []byte(sealPurpose)), 0o600, true)
}

func readSealedCredentials(dir *fileutil.Directory) (map[string]string, error) {
	values := make(map[string]string)
	data, info, err := dir.ReadRegularFile(sealedCredentialsFile)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm() != 0o600 {
		return nil, errKeyring
	}
	seal, err := credentialSeal(dir, false)
	if err != nil {
		return nil, err
	}
	plain, err := seal.Open(nil, nil, data, []byte(sealPurpose))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plain, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, errKeyring
	}
	return values, nil
}

func credentialSeal(dir *fileutil.Directory, create bool) (cipher.AEAD, error) {
	secret, info, err := dir.ReadRegularFile(credentialSealFile)
	if create && errors.Is(err, os.ErrNotExist) {
		secret = make([]byte, sealSize)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		if err := dir.AtomicWriteFile(credentialSealFile, secret, 0o600, false); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if len(secret) != sealSize || info.Mode().Perm() != 0o600 {
		return nil, errKeyring
	}
	key, err := hkdf.Key(sha256.New, secret, nil, sealPurpose, sealSize)
	if err != nil {
		return nil, err
	}
	cipherBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(cipherBlock)
}
