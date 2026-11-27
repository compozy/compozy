package account

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/compozy/compozy/internal/vault"
	"github.com/zalando/go-keyring"
)

const (
	keyringService   = "compozy.account"
	deviceKeyEntry   = "device-key"
	refreshEntry     = "refresh"
	legacyTokenEntry = "account-token"
)

type systemKeyring struct{}

var _ vault.Keyring = systemKeyring{}

func (systemKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (systemKeyring) Set(service, user, value string) error    { return keyring.Set(service, user, value) }
func (systemKeyring) Delete(service, user string) error        { return keyring.Delete(service, user) }

type credentials struct {
	mu       sync.Mutex
	ring     vault.Keyring
	fallback *sealedKeyring
	scope    string
}

func newCredentials(home string, ring vault.Keyring) (*credentials, error) {
	canonical, err := filepath.EvalSymlinks(home)
	if err != nil {
		return nil, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, err
	}
	c := &credentials{ring: ring, scope: fmt.Sprintf("%x", sha256.Sum256([]byte(canonical)))}
	if ring == nil {
		c.ring = systemKeyring{}
		if runtime.GOOS == "linux" {
			c.fallback = &sealedKeyring{dir: filepath.Join(canonical, "account")}
			if _, err := os.Lstat(filepath.Join(c.fallback.dir, "keys.enc")); err == nil {
				c.ring = c.fallback
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}
	return c, nil
}

func (c *credentials) entry(kind, id string) string { return c.scope + ":" + kind + ":" + id }

func (c *credentials) deviceKey(id string, create bool) (*ecdsa.PrivateKey, error) {
	raw, err := c.read(deviceKeyEntry, id)
	if err == nil {
		data, decodeErr := base64.RawStdEncoding.DecodeString(raw)
		if decodeErr != nil {
			return nil, errKeyring
		}
		key, parseErr := x509.ParseECPrivateKey(data)
		if parseErr != nil || key.Curve != elliptic.P256() {
			return nil, errKeyring
		}
		return key, nil
	}
	if !create || !errors.Is(err, keyring.ErrNotFound) {
		return nil, errKeyring
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errKeyring
	}
	data, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, errKeyring
	}
	if err := c.write(deviceKeyEntry, id, base64.RawStdEncoding.EncodeToString(data)); err != nil {
		return nil, err
	}
	return key, nil
}

func (c *credentials) read(kind, id string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entry(kind, id)
	value, err := c.ring.Get(keyringService, entry)
	if c.useFallback(err) {
		return c.ring.Get(keyringService, entry)
	}
	return value, err
}

func (c *credentials) write(kind, id, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entry(kind, id)
	err := c.ring.Set(keyringService, entry, value)
	if c.useFallback(err) {
		err = c.ring.Set(keyringService, entry, value)
	}
	if err != nil {
		return errKeyring
	}
	return nil
}

func (c *credentials) remove(kind, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entry(kind, id)
	err := c.ring.Delete(keyringService, entry)
	if c.useFallback(err) {
		err = c.ring.Delete(keyringService, entry)
	}
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return errKeyring
	}
	return nil
}

func (c *credentials) useFallback(err error) bool {
	if c.fallback == nil || err == nil || c.ring == c.fallback || !noSecretService(err) {
		return false
	}
	c.ring = c.fallback
	return true
}

func noSecretService(err error) bool {
	// Secret Service returns these errors when the session bus or service is absent.
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, os.ErrNotExist) ||
		strings.Contains(err.Error(), "The name org.freedesktop.secrets was not provided") ||
		strings.Contains(err.Error(), "Name \"org.freedesktop.secrets\" does not exist")
}
