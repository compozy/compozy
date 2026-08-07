package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
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

	t.Run("Should reject credential files whose permissions are not exactly 0600", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir() + "/credentials"
		store := gatewayCredentialStore{
			dir:     directory,
			keys:    &memoryCredentialKeyring{values: make(map[string]string)},
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x3a}, 64)),
		}
		path, err := store.write("laptop", "cpz_gwd_"+strings.Repeat("d", 43))
		if err != nil {
			t.Fatalf("write() error = %v", err)
		}
		if err := os.Chmod(path, 0o700); err != nil {
			t.Fatalf("Chmod() error = %v", err)
		}

		if _, err := store.read("laptop"); err == nil || !strings.Contains(err.Error(), "must be 0600") {
			t.Fatalf("read() error = %v, want strict permission failure", err)
		}
	})

	t.Run("Should reject a symbolic link before reading credential bytes", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir() + "/credentials"
		store := gatewayCredentialStore{
			dir:     directory,
			keys:    &memoryCredentialKeyring{values: make(map[string]string)},
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x4a}, 64)),
		}
		target, err := store.write("laptop", "cpz_gwd_"+strings.Repeat("e", 43))
		if err != nil {
			t.Fatalf("write() error = %v", err)
		}
		if err := os.Symlink(target, filepath.Join(directory, "linked.cred")); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}

		if _, err := store.read("linked"); !errors.Is(err, fileutil.ErrSymlink) {
			t.Fatalf("read() error = %v, want fileutil.ErrSymlink", err)
		}
	})

	t.Run("Should remove a newly-created key when credential publication fails", func(t *testing.T) {
		t.Parallel()

		blockedDirectory := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(blockedDirectory, []byte("occupied"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		keys := &memoryCredentialKeyring{values: make(map[string]string)}
		store := gatewayCredentialStore{
			dir:     blockedDirectory,
			keys:    keys,
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x5a}, 64)),
		}

		if _, err := store.write("laptop", "cpz_gwd_"+strings.Repeat("f", 43)); err == nil {
			t.Fatal("write() error = nil, want publication failure")
		}
		if len(keys.values) != 0 {
			t.Fatalf("write() retained an unused key: %#v", keys.values)
		}
	})

	t.Run("Should keep the credential pair intact when key removal fails", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir() + "/credentials"
		keys := &memoryCredentialKeyring{values: make(map[string]string)}
		store := gatewayCredentialStore{
			dir:     directory,
			keys:    keys,
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x6a}, 64)),
		}
		credential := "cpz_gwd_" + strings.Repeat("g", 43)
		path, err := store.write("laptop", credential)
		if err != nil {
			t.Fatalf("write() error = %v", err)
		}
		deleteErr := errors.New("keyring delete failed")
		keys.deleteErr = deleteErr

		if err := store.remove("laptop"); !errors.Is(err, deleteErr) {
			t.Fatalf("remove() error = %v, want keyring failure", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat() error = %v, want credential file preserved", err)
		}
		got, err := store.read("laptop")
		if err != nil {
			t.Fatalf("read() error = %v", err)
		}
		if got != credential {
			t.Fatalf("read() = %q, want %q", got, credential)
		}
	})

	t.Run("Should restore the key when credential file removal fails", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir() + "/credentials"
		keys := &memoryCredentialKeyring{values: make(map[string]string)}
		store := gatewayCredentialStore{
			dir:     directory,
			keys:    keys,
			entropy: bytes.NewReader(bytes.Repeat([]byte{0x7a}, 64)),
		}
		credential := "cpz_gwd_" + strings.Repeat("h", 43)
		if _, err := store.write("laptop", credential); err != nil {
			t.Fatalf("write() error = %v", err)
		}
		removeErr := errors.New("credential file removal failed")
		store.removeFile = func(string) error { return removeErr }

		if err := store.remove("laptop"); !errors.Is(err, removeErr) {
			t.Fatalf("remove() error = %v, want file removal failure", err)
		}
		got, err := store.read("laptop")
		if err != nil {
			t.Fatalf("read() after rollback error = %v", err)
		}
		if got != credential {
			t.Fatalf("read() after rollback = %q, want %q", got, credential)
		}
	})
}

func TestGatewayProfileBundle(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve one device identity in a portable encrypted profile", func(t *testing.T) {
		t.Parallel()
		bundle := gatewayProfileBundle{
			Profile: compozyconfig.GatewayConnectionConfig{
				Name: "laptop", Scheme: "https", Host: "gateway.example", Port: 443,
				CredentialFile: "laptop.cred", DefaultWorkspace: "workspace-1",
			},
			Credential: testGatewayCredential('p'),
		}
		passphrase := []byte("portable bundle passphrase")
		payload, err := encryptGatewayProfileBundle(
			bundle,
			passphrase,
			bytes.NewReader(bytes.Repeat([]byte{0x42}, 64)),
		)
		if err != nil {
			t.Fatalf("encryptGatewayProfileBundle() error = %v", err)
		}
		if bytes.Contains(payload, []byte(bundle.Credential)) || bytes.Contains(payload, passphrase) {
			t.Fatal("encrypted bundle contains raw credential or passphrase")
		}
		copied := append([]byte(nil), payload...)
		decoded, err := decryptGatewayProfileBundle(copied, passphrase)
		if err != nil {
			t.Fatalf("decryptGatewayProfileBundle(copy) error = %v", err)
		}
		if decoded.Profile != bundle.Profile || decoded.Credential != bundle.Credential {
			t.Fatalf("decoded bundle = %#v, want %#v", decoded, bundle)
		}
	})

	t.Run("Should fail closed for a wrong passphrase or tampered profile name", func(t *testing.T) {
		t.Parallel()
		bundle := gatewayProfileBundle{
			Profile: compozyconfig.GatewayConnectionConfig{
				Name: "laptop", Scheme: "https", Host: "gateway.example", Port: 443,
				CredentialFile: "laptop.cred",
			},
			Credential: testGatewayCredential('q'),
		}
		passphrase := []byte("correct bundle passphrase")
		payload, err := encryptGatewayProfileBundle(
			bundle,
			passphrase,
			bytes.NewReader(bytes.Repeat([]byte{0x24}, 64)),
		)
		if err != nil {
			t.Fatalf("encryptGatewayProfileBundle() error = %v", err)
		}
		if _, err := decryptGatewayProfileBundle(payload, []byte("incorrect passphrase value")); err == nil {
			t.Fatal("decryptGatewayProfileBundle(wrong passphrase) error = nil")
		}

		rawEnvelope, err := base64.RawURLEncoding.DecodeString(
			strings.TrimPrefix(strings.TrimSpace(string(payload)), gatewayProfileBundlePrefix),
		)
		if err != nil {
			t.Fatalf("decode envelope error = %v", err)
		}
		var envelope gatewayProfileBundleEnvelope
		if err := json.Unmarshal(rawEnvelope, &envelope); err != nil {
			t.Fatalf("json.Unmarshal(envelope) error = %v", err)
		}
		envelope.Profile = "other"
		tamperedEnvelope, err := json.Marshal(envelope)
		if err != nil {
			t.Fatalf("json.Marshal(envelope) error = %v", err)
		}
		tampered := []byte(
			gatewayProfileBundlePrefix + base64.RawURLEncoding.EncodeToString(tamperedEnvelope) + "\n",
		)
		if _, err := decryptGatewayProfileBundle(tampered, passphrase); err == nil {
			t.Fatal("decryptGatewayProfileBundle(tampered name) error = nil")
		}
	})

	t.Run("Should reject passphrase files visible to another user", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "passphrase")
		if err := os.WriteFile(path, []byte("private profile passphrase"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(passphrase) error = %v", err)
		}
		if _, err := readGatewayProfilePassphrase(path); err == nil {
			t.Fatal("readGatewayProfilePassphrase() error = nil, want permission refusal")
		}
	})
}
