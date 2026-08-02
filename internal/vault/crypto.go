package vault

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	keySizeBytes        = 32
	keyDirectoryMode    = 0o700
	keyFileMode         = 0o600
	keyTempNameAttempts = 16
	keyTempSuffixBytes  = 12
)

// KeyProvider loads the daemon-local vault encryption key.
type KeyProvider interface {
	Key() ([]byte, error)
}

type fileKeyProvider struct {
	path      string
	lookupEnv func(string) (string, bool)
	mu        sync.Mutex
	cached    []byte
}

// NewFileKeyProvider returns a non-interactive key provider backed by COMPOZY_VAULT_KEY or a 0600 key file.
func NewFileKeyProvider(homeDir string, lookupEnv func(string) (string, bool)) KeyProvider {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	return &fileKeyProvider{
		path:      filepath.Join(strings.TrimSpace(homeDir), "vault.key"),
		lookupEnv: lookupEnv,
	}
}

func (p *fileKeyProvider) Key() ([]byte, error) {
	lookupEnv := p.lookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if value, ok := lookupEnv("COMPOZY_VAULT_KEY"); ok && strings.TrimSpace(value) != "" {
		key, err := decodeKey(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("vault: decode COMPOZY_VAULT_KEY: %w", err)
		}
		return key, nil
	}
	if strings.TrimSpace(p.path) == "" {
		return nil, errors.New("vault: key path is required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.cached) == keySizeBytes {
		return append([]byte(nil), p.cached...), nil
	}
	key, err := readOrCreateKeyFile(p.path)
	if err != nil {
		return nil, err
	}
	p.cached = append([]byte(nil), key...)
	return append([]byte(nil), key...), nil
}

func readOrCreateKeyFile(path string) ([]byte, error) {
	key, err := readKeyFile(path)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return createKeyFile(path)
}

func readKeyFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("vault: inspect key file %q: %w", path, err)
	}
	if err := validateKeyFile(path, info); err != nil {
		return nil, err
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vault: read key file %q: %w", path, err)
	}
	key, err := decodeKey(strings.TrimSpace(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("vault: decode key file %q: %w", path, err)
	}
	return key, nil
}

func validateKeyFile(path string, info os.FileInfo) error {
	if info == nil {
		return fmt.Errorf("vault: inspect key file %q: file info is required", path)
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		return fmt.Errorf("vault: key file %q must not be a symlink", path)
	}
	if !mode.IsRegular() {
		return fmt.Errorf("vault: key file %q must be a regular file", path)
	}
	if mode.Perm()&0o077 != 0 {
		return fmt.Errorf(
			"vault: key file %q permissions %o must not grant group or other permissions",
			path,
			mode.Perm(),
		)
	}
	return nil
}

func createKeyFile(path string) ([]byte, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, keyDirectoryMode); err != nil {
		return nil, fmt.Errorf("vault: create key directory %q: %w", dir, err)
	}
	if err := os.Chmod(dir, keyDirectoryMode); err != nil {
		return nil, fmt.Errorf("vault: secure key directory %q: %w", dir, err)
	}
	key, err := generateKey()
	if err != nil {
		return nil, err
	}
	tempPath, err := writeTempKeyFile(dir, key)
	if err != nil {
		return nil, err
	}
	linkErr := os.Link(tempPath, path)
	cleanupErr := removeTempKeyFile(tempPath)
	if linkErr != nil {
		linkErr = fmt.Errorf("vault: install key file %q: %w", path, linkErr)
		if cleanupErr != nil {
			return nil, errors.Join(linkErr, cleanupErr)
		}
		if errors.Is(linkErr, os.ErrExist) {
			return readKeyFile(path)
		}
		return nil, linkErr
	}
	if cleanupErr != nil {
		return nil, cleanupErr
	}
	if err := fileutil.SyncDir(dir); err != nil {
		return nil, fmt.Errorf("vault: sync key directory %q: %w", dir, err)
	}
	return key, nil
}

func generateKey() ([]byte, error) {
	key := make([]byte, keySizeBytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("vault: generate key: %w", err)
	}
	return key, nil
}

func writeTempKeyFile(dir string, key []byte) (string, error) {
	file, tempPath, err := createExclusiveTempKeyFile(dir)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	writeErr := writeOpenKeyFile(file, tempPath, encoded)
	if writeErr != nil {
		cleanupErr := removeTempKeyFile(tempPath)
		if cleanupErr != nil {
			return "", errors.Join(writeErr, cleanupErr)
		}
		return "", writeErr
	}
	return tempPath, nil
}

func createExclusiveTempKeyFile(dir string) (*os.File, string, error) {
	for range keyTempNameAttempts {
		suffix, err := randomKeyTempSuffix()
		if err != nil {
			return nil, "", err
		}
		tempPath := filepath.Join(dir, ".vault-key-"+suffix)
		file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, keyFileMode)
		if err == nil {
			return file, tempPath, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return nil, "", fmt.Errorf("vault: create temp key file %q: %w", tempPath, err)
	}
	return nil, "", fmt.Errorf("vault: create temp key file in %q: exhausted unique names", dir)
}

func randomKeyTempSuffix() (string, error) {
	buf := make([]byte, keyTempSuffixBytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("vault: generate temp key filename: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func writeOpenKeyFile(file *os.File, path string, encoded string) error {
	var err error
	if err = file.Chmod(keyFileMode); err == nil {
		_, err = file.WriteString(encoded + "\n")
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("vault: write temp key file %q: %w", path, errors.Join(err, closeErr))
	}
	if closeErr != nil {
		return fmt.Errorf("vault: close temp key file %q: %w", path, closeErr)
	}
	return nil
}

func removeTempKeyFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("vault: remove temp key file %q: %w", path, err)
	}
	return nil
}

func decodeKey(value string) ([]byte, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("key is required")
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == keySizeBytes {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == keySizeBytes {
		return decoded, nil
	}
	if len(value) == keySizeBytes {
		return []byte(value), nil
	}
	return nil, fmt.Errorf("key must be %d bytes as raw text, hex, or base64", keySizeBytes)
}
