package cli

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
	keyring "github.com/zalando/go-keyring"
)

const (
	gatewayCredentialKeyringService = "compozy-gateway"
	// #nosec G101 -- this is an encrypted payload format marker, not credential material.
	gatewayCredentialCipherPrefix = "aes-gcm-v1:"
	// #nosec G101 -- this public type prefix contains no credential material.
	gatewayDeviceCredentialPrefix = "cpz_gwd_"
	gatewayCredentialKeyBytes     = 32
)

type credentialKeyring interface {
	Get(string, string) (string, error)
	Set(string, string, string) error
	Delete(string, string) error
}

type osCredentialKeyring struct{}

func (osCredentialKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (osCredentialKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (osCredentialKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

type gatewayCredentialStore struct {
	dir        string
	keys       credentialKeyring
	entropy    io.Reader
	removeFile func(string) error
}

// WriteGatewayCredential encrypts a remote profile credential and stores its file with mode 0600.
func WriteGatewayCredential(credentialsDir, profile, credential string) (string, error) {
	store := gatewayCredentialStore{dir: credentialsDir, keys: osCredentialKeyring{}, entropy: rand.Reader}
	return store.write(profile, credential)
}

// ReadGatewayCredential decrypts a remote profile credential using the operator's OS keyring.
func ReadGatewayCredential(credentialsDir, profile string) (string, error) {
	store := gatewayCredentialStore{dir: credentialsDir, keys: osCredentialKeyring{}, entropy: rand.Reader}
	return store.read(profile)
}

// RemoveGatewayCredential removes both the encrypted file and its OS-keyring key.
func RemoveGatewayCredential(credentialsDir, profile string) error {
	store := gatewayCredentialStore{dir: credentialsDir, keys: osCredentialKeyring{}, entropy: rand.Reader}
	return store.remove(profile)
}

func (s gatewayCredentialStore) write(profile, credential string) (string, error) {
	profile, err := normalizedGatewayProfileName(profile)
	if err != nil {
		return "", err
	}
	path, err := s.path(profile)
	if err != nil {
		return "", err
	}
	credential = strings.TrimSpace(credential)
	if !validGatewayDeviceCredential(credential) {
		return "", errors.New("cli: gateway device credential is invalid")
	}
	keyUser, err := s.keyringUser(profile)
	if err != nil {
		return "", err
	}
	key, keyCreated, err := s.loadOrCreateKey(keyUser)
	if err != nil {
		return "", err
	}
	ciphertext, err := encryptGatewayCredential(key, credential, keyUser, s.entropy)
	if err != nil {
		return "", s.cleanupCreatedKey(keyUser, keyCreated, err)
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", s.cleanupCreatedKey(
			keyUser,
			keyCreated,
			fmt.Errorf("cli: create gateway credential directory: %w", err),
		)
	}
	if err := os.Chmod(s.dir, 0o700); err != nil {
		return "", s.cleanupCreatedKey(
			keyUser,
			keyCreated,
			fmt.Errorf("cli: secure gateway credential directory: %w", err),
		)
	}
	if err := fileutil.AtomicWriteFile(path, []byte(ciphertext+"\n"), 0o600); err != nil {
		return "", s.cleanupCreatedKey(
			keyUser,
			keyCreated,
			fmt.Errorf("cli: write gateway credential: %w", err),
		)
	}
	return path, nil
}

func validGatewayDeviceCredential(credential string) bool {
	if !strings.HasPrefix(credential, gatewayDeviceCredentialPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(credential, gatewayDeviceCredentialPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	return err == nil && len(decoded) == gatewayCredentialKeyBytes
}

func (s gatewayCredentialStore) read(profile string) (string, error) {
	profile, err := normalizedGatewayProfileName(profile)
	if err != nil {
		return "", err
	}
	path, err := s.path(profile)
	if err != nil {
		return "", err
	}
	ciphertext, info, err := fileutil.ReadRegularFile(path)
	if err != nil {
		return "", fmt.Errorf("cli: read gateway credential file: %w", err)
	}
	if info.Mode().Perm() != 0o600 {
		return "", errors.New("cli: gateway credential file permissions must be 0600")
	}
	keyUser, err := s.keyringUser(profile)
	if err != nil {
		return "", err
	}
	encodedKey, err := s.keys.Get(gatewayCredentialKeyringService, keyUser)
	if err != nil {
		return "", fmt.Errorf("cli: read gateway credential key: %w", err)
	}
	key, err := decodeGatewayCredentialKey(encodedKey)
	if err != nil {
		return "", err
	}
	return decryptGatewayCredential(key, strings.TrimSpace(string(ciphertext)), keyUser)
}

func (s gatewayCredentialStore) remove(profile string) error {
	profile, err := normalizedGatewayProfileName(profile)
	if err != nil {
		return err
	}
	path, err := s.path(profile)
	if err != nil {
		return err
	}
	keyUser, err := s.keyringUser(profile)
	if err != nil {
		return err
	}

	_, _, fileReadErr := fileutil.ReadRegularFile(path)
	fileExists := fileReadErr == nil
	if fileReadErr != nil && !errors.Is(fileReadErr, os.ErrNotExist) {
		return fmt.Errorf("cli: inspect gateway credential file: %w", fileReadErr)
	}

	encodedKey, keyReadErr := s.keys.Get(gatewayCredentialKeyringService, keyUser)
	keyExists := keyReadErr == nil
	if keyReadErr != nil && !errors.Is(keyReadErr, keyring.ErrNotFound) {
		return fmt.Errorf("cli: read gateway credential key before removal: %w", keyReadErr)
	}
	if keyExists {
		if err := s.keys.Delete(gatewayCredentialKeyringService, keyUser); err != nil {
			return fmt.Errorf("cli: remove gateway credential key: %w", err)
		}
	}
	if !fileExists {
		return nil
	}
	removeFile := s.removeFile
	if removeFile == nil {
		removeFile = fileutil.AtomicRemoveFile
	}
	if err := removeFile(path); err != nil {
		removeErr := fmt.Errorf("cli: remove gateway credential file: %w", err)
		if !keyExists {
			return removeErr
		}
		if restoreErr := s.keys.Set(gatewayCredentialKeyringService, keyUser, encodedKey); restoreErr != nil {
			return errors.Join(removeErr, fmt.Errorf("cli: restore gateway credential key: %w", restoreErr))
		}
		return removeErr
	}
	return nil
}

func (s gatewayCredentialStore) path(profile string) (string, error) {
	profile = strings.TrimSpace(profile)
	if strings.TrimSpace(s.dir) == "" || !validGatewayProfileName(profile) {
		return "", errors.New("cli: gateway credential directory and safe profile name are required")
	}
	return filepath.Join(s.dir, profile+".cred"), nil
}

func (s gatewayCredentialStore) keyringUser(profile string) (string, error) {
	canonicalDir, err := canonicalGatewayCredentialDirectory(s.dir)
	if err != nil {
		return "", err
	}
	scope := sha256.Sum256([]byte(canonicalDir))
	return profile + "@" + hex.EncodeToString(scope[:16]), nil
}

func canonicalGatewayCredentialDirectory(directory string) (string, error) {
	absoluteDir, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("cli: resolve gateway credential directory identity: %w", err)
	}
	candidate := filepath.Clean(absoluteDir)
	missing := make([]string, 0, 2)
	for {
		resolved, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr == nil {
			for _, component := range slices.Backward(missing) {
				resolved = filepath.Join(resolved, component)
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", fmt.Errorf("cli: resolve gateway credential directory symlinks: %w", resolveErr)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", fmt.Errorf("cli: resolve gateway credential directory symlinks: %w", resolveErr)
		}
		missing = append(missing, filepath.Base(candidate))
		candidate = parent
	}
}

func (s gatewayCredentialStore) loadOrCreateKey(keyUser string) ([]byte, bool, error) {
	encoded, err := s.keys.Get(gatewayCredentialKeyringService, keyUser)
	if err == nil {
		key, decodeErr := decodeGatewayCredentialKey(encoded)
		return key, false, decodeErr
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return nil, false, fmt.Errorf("cli: read gateway credential key: %w", err)
	}
	key := make([]byte, gatewayCredentialKeyBytes)
	if _, err := io.ReadFull(s.entropy, key); err != nil {
		return nil, false, fmt.Errorf("cli: generate gateway credential key: %w", err)
	}
	encoded = base64.RawURLEncoding.EncodeToString(key)
	if err := s.keys.Set(gatewayCredentialKeyringService, keyUser, encoded); err != nil {
		return nil, false, fmt.Errorf("cli: store gateway credential key: %w", err)
	}
	return key, true, nil
}

func (s gatewayCredentialStore) cleanupCreatedKey(keyUser string, created bool, cause error) error {
	if !created {
		return cause
	}
	if err := s.keys.Delete(gatewayCredentialKeyringService, keyUser); err != nil &&
		!errors.Is(err, keyring.ErrNotFound) {
		return errors.Join(cause, fmt.Errorf("cli: remove unused gateway credential key: %w", err))
	}
	return cause
}

func encryptGatewayCredential(key []byte, plaintext, profile string, entropy io.Reader) (string, error) {
	aead, err := newGatewayCredentialAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(entropy, nonce); err != nil {
		return "", fmt.Errorf("cli: generate gateway credential nonce: %w", err)
	}
	payload := aead.Seal(nonce, nonce, []byte(plaintext), []byte(profile))
	return gatewayCredentialCipherPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func decryptGatewayCredential(key []byte, encoded, profile string) (string, error) {
	if !strings.HasPrefix(encoded, gatewayCredentialCipherPrefix) {
		return "", errors.New("cli: gateway credential ciphertext version is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, gatewayCredentialCipherPrefix))
	if err != nil {
		return "", fmt.Errorf("cli: decode gateway credential ciphertext: %w", err)
	}
	aead, err := newGatewayCredentialAEAD(key)
	if err != nil {
		return "", err
	}
	if len(payload) < aead.NonceSize() {
		return "", errors.New("cli: gateway credential ciphertext is truncated")
	}
	plaintext, err := aead.Open(nil, payload[:aead.NonceSize()], payload[aead.NonceSize():], []byte(profile))
	if err != nil {
		return "", errors.New("cli: decrypt gateway credential")
	}
	return string(plaintext), nil
}

func newGatewayCredentialAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cli: initialize gateway credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cli: initialize gateway credential AEAD: %w", err)
	}
	return aead, nil
}

func decodeGatewayCredentialKey(encoded string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(key) != gatewayCredentialKeyBytes {
		return nil, errors.New("cli: gateway credential key is invalid")
	}
	return key, nil
}

func validGatewayProfileName(profile string) bool {
	return compozyconfig.ValidGatewayConnectionName(profile)
}

func normalizedGatewayProfileName(profile string) (string, error) {
	profile = strings.TrimSpace(profile)
	if !validGatewayProfileName(profile) {
		return "", errors.New("cli: gateway profile name is invalid")
	}
	return profile, nil
}
