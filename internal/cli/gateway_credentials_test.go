package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

type memoryCredentialKeyring struct {
	values    map[string]string
	getErr    error
	setErr    error
	deleteErr error
}

func (k *memoryCredentialKeyring) Get(service, user string) (string, error) {
	if k.getErr != nil {
		return "", k.getErr
	}
	value, ok := k.values[service+"\x00"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (k *memoryCredentialKeyring) Set(service, user, password string) error {
	if k.setErr != nil {
		return k.setErr
	}
	k.values[service+"\x00"+user] = password
	return nil
}

func (k *memoryCredentialKeyring) Delete(service, user string) error {
	if k.deleteErr != nil {
		return k.deleteErr
	}
	key := service + "\x00" + user
	if _, ok := k.values[key]; !ok {
		return keyring.ErrNotFound
	}
	delete(k.values, key)
	return nil
}

func TestGatewayCredentialStore(t *testing.T) {
	t.Parallel()

	t.Run("Should encrypt credentials at rest and preserve strict permissions [UT-049]", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir() + "/credentials"
		keys := &memoryCredentialKeyring{values: make(map[string]string)}
		store := gatewayCredentialStore{
			dir:     directory,
			keys:    keys,
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x5a}, 64)),
		}
		credential := "cpz_gwd_" + strings.Repeat("a", 43)

		path, err := store.write("laptop", credential)
		if err != nil {
			t.Fatalf("write() error = %v", err)
		}
		stored, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if bytes.Contains(stored, []byte(credential)) {
			t.Fatalf("credential file contains plaintext credential %q", credential)
		}
		if !bytes.HasPrefix(stored, []byte(gatewayCredentialCipherPrefix)) {
			t.Fatalf("credential file = %q, want versioned ciphertext", stored)
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(file) error = %v", err)
		}
		if got := fileInfo.Mode().Perm(); got != 0o600 {
			t.Fatalf("file permissions = %o, want 600", got)
		}
		dirInfo, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("Stat(directory) error = %v", err)
		}
		if got := dirInfo.Mode().Perm(); got != 0o700 {
			t.Fatalf("directory permissions = %o, want 700", got)
		}

		got, err := store.read("laptop")
		if err != nil {
			t.Fatalf("read() error = %v", err)
		}
		if got != credential {
			t.Fatalf("read() = %q, want %q", got, credential)
		}
		if err := store.remove("laptop"); err != nil {
			t.Fatalf("remove() error = %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(removed file) error = %v, want os.ErrNotExist", err)
		}
		if _, ok := keys.values[gatewayCredentialKeyringService+"\x00laptop"]; ok {
			t.Fatal("remove() retained the OS-keyring encryption key")
		}
	})

	t.Run("Should fail closed when the OS keyring is unavailable", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir() + "/credentials"
		keyringErr := errors.New("keyring unavailable")
		store := gatewayCredentialStore{
			dir: directory,
			keys: &memoryCredentialKeyring{
				values: make(map[string]string),
				getErr: keyringErr,
			},
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x2a}, 64)),
		}

		_, err := store.write("laptop", "cpz_gwd_"+strings.Repeat("b", 43))
		if !errors.Is(err, keyringErr) {
			t.Fatalf("write() error = %v, want keyring failure", err)
		}
		if _, statErr := os.Stat(directory); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(directory) error = %v, want no plaintext fallback", statErr)
		}
	})

	t.Run("Should reject profile names that can escape the credential directory", func(t *testing.T) {
		t.Parallel()

		store := gatewayCredentialStore{
			dir:     t.TempDir(),
			keys:    &memoryCredentialKeyring{values: make(map[string]string)},
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x1a}, 64)),
		}
		if _, err := store.write("../outside", "cpz_gwd_"+strings.Repeat("c", 43)); err == nil {
			t.Fatal("write() error = nil, want unsafe profile rejection")
		}
	})

	t.Run("Should reject a malformed credential before touching the keyring or disk", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir() + "/credentials"
		keys := &memoryCredentialKeyring{values: make(map[string]string)}
		store := gatewayCredentialStore{
			dir: directory, keys: keys,
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x1b}, 64)),
		}
		for _, credential := range []string{
			"cpz_gwd_",
			"cpz_gwd_short",
			"cpz_gwd_" + strings.Repeat("!", 43),
			"other_" + strings.Repeat("a", 43),
		} {
			if _, err := store.write("laptop", credential); err == nil {
				t.Fatalf("write(%q) error = nil, want invalid credential", credential)
			}
		}
		if len(keys.values) != 0 {
			t.Fatalf("invalid credentials wrote keyring values: %#v", keys.values)
		}
		if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(directory) error = %v, want no credential directory", err)
		}
	})
}
