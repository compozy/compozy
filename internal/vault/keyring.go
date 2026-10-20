package vault

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zalando/go-keyring"
)

const keyringService = "compozy-vault"

// Keyring is the OS credential-store boundary used by vault key operations.
type Keyring interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

type systemKeyring struct{}

var _ Keyring = systemKeyring{}

func (systemKeyring) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}

func (systemKeyring) Set(service, account, value string) error {
	return keyring.Set(service, account, value)
}

func (systemKeyring) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

type osKeyProvider struct {
	home   string
	ring   Keyring
	system bool
	cache  keyringKeyCache
}

type keyringKeyCache struct {
	once sync.Once
	key  []byte
	err  error
}

// All Vault consumers in one daemon share one OS read for the same home.
var systemKeyringKeys sync.Map

var _ KeyProvider = (*osKeyProvider)(nil)

// NewOSKeyProvider reads an explicitly provisioned key. It never imports a file,
// creates a replacement key, or falls back when the OS keyring is unavailable.
// A nil ring selects the system keyring; tests inject only this I/O boundary.
func NewOSKeyProvider(home string, ring Keyring) KeyProvider {
	system := ring == nil
	if ring == nil {
		ring = systemKeyring{}
	}
	return &osKeyProvider{home: home, ring: ring, system: system}
}

func (p *osKeyProvider) Key() ([]byte, error) {
	account, err := keyringAccount(p.home)
	if err != nil {
		return nil, err
	}
	cache := &p.cache
	if p.system {
		shared, _ := systemKeyringKeys.LoadOrStore(account, &keyringKeyCache{})
		var ok bool
		cache, ok = shared.(*keyringKeyCache)
		if !ok {
			return nil, errors.New("vault: invalid OS keyring cache")
		}
	}
	cache.once.Do(func() {
		cache.key, cache.err = p.readKey(account)
	})
	return bytes.Clone(cache.key), cache.err
}

func (p *osKeyProvider) readKey(account string) ([]byte, error) {
	encoded, err := p.ring.Get(keyringService, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, errors.New(
			"vault: OS keyring key is missing; explicitly migrate the existing vault key before selecting os-keyring",
		)
	}
	if err != nil {
		// Credential-store errors can contain implementation-specific data. Keep
		// the diagnostic actionable without echoing keyring payloads or accounts.
		return nil, errors.New(
			"vault: OS keyring is unavailable or locked; unlock the credential store and restart the daemon",
		)
	}
	key, err := decodeKey(encoded)
	if err != nil {
		return nil, errors.New("vault: OS keyring contains an invalid encryption key; restore the original key")
	}
	return key, nil
}

func keyringAccount(home string) (string, error) {
	if strings.TrimSpace(home) == "" {
		return "", errors.New("vault: home directory is required for OS keyring isolation")
	}
	canonical, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", fmt.Errorf("vault: resolve keyring home: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("vault: resolve absolute keyring home: %w", err)
	}
	return fmt.Sprintf("home-%x", sha256.Sum256([]byte(canonical))), nil
}
