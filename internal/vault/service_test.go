package vault

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type staticKeyProvider struct {
	key []byte
}

func (p staticKeyProvider) Key() ([]byte, error) {
	return append([]byte(nil), p.key...), nil
}

type memoryVaultStore struct {
	records map[string]Record
}

func newMemoryVaultStore() *memoryVaultStore {
	return &memoryVaultStore{records: make(map[string]Record)}
}

func (s *memoryVaultStore) PutVaultSecret(_ context.Context, record Record) error {
	if existing, ok := s.records[record.Ref]; ok {
		record.CreatedAt = existing.CreatedAt
	}
	s.records[record.Ref] = record
	return nil
}

func (s *memoryVaultStore) GetVaultSecret(_ context.Context, ref string) (Record, error) {
	record, ok := s.records[NormalizeRef(ref)]
	if !ok {
		return Record{}, ErrSecretNotFound
	}
	return record, nil
}

func (s *memoryVaultStore) ListVaultSecrets(_ context.Context, prefix string) ([]Record, error) {
	records := make([]Record, 0, len(s.records))
	for ref, record := range s.records {
		if RefMatchesPrefix(ref, prefix) {
			records = append(records, record)
		}
	}
	slices.SortFunc(records, func(a, b Record) int {
		return strings.Compare(a.Ref, b.Ref)
	})
	return records, nil
}

func TestServicePutSecretReturnsPersistedMetadataAndPreservesKindOnRotation(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve created timestamp and kind when rotating without kind", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newMemoryVaultStore()
		current := time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)
		service, err := NewService(
			store,
			staticKeyProvider{key: []byte("01234567890123456789012345678901")},
			WithNow(func() time.Time { return current }),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		ref := "vault:sessions/sess-1/github-token"
		created, err := service.PutSecret(ctx, ref, "token", "first-secret-value")
		if err != nil {
			t.Fatalf("PutSecret(first) error = %v", err)
		}

		current = current.Add(time.Hour)
		rotated, err := service.PutSecret(ctx, ref, "", "rotated-secret-value")
		if err != nil {
			t.Fatalf("PutSecret(rotated) error = %v", err)
		}

		if rotated.Kind != "token" {
			t.Fatalf("rotated.Kind = %q, want preserved token kind", rotated.Kind)
		}
		if !rotated.CreatedAt.Equal(created.CreatedAt) {
			t.Fatalf("rotated.CreatedAt = %v, want original %v", rotated.CreatedAt, created.CreatedAt)
		}
		if !rotated.UpdatedAt.Equal(current) {
			t.Fatalf("rotated.UpdatedAt = %v, want %v", rotated.UpdatedAt, current)
		}
	})
}

func (s *memoryVaultStore) DeleteVaultSecret(_ context.Context, ref string) error {
	normalized := NormalizeRef(ref)
	if _, ok := s.records[normalized]; !ok {
		return ErrSecretNotFound
	}
	delete(s.records, normalized)
	return nil
}

func TestServiceStoresEncryptedSecretsAndResolvesRefs(t *testing.T) {
	t.Parallel()

	t.Run("Should store encrypted vault value and resolve vault and env refs", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := newMemoryVaultStore()
		now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		service, err := NewService(
			store,
			staticKeyProvider{key: []byte("01234567890123456789012345678901")},
			WithNow(func() time.Time { return now }),
			WithLookupEnv(func(key string) (string, bool) {
				if key == "OPENROUTER_API_KEY" {
					return "env-secret-value", true
				}
				return "", false
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		metadata, err := service.PutSecret(
			ctx,
			"vault:providers/openrouter/api-key",
			"api_key",
			"vault-secret-value",
		)
		if err != nil {
			t.Fatalf("PutSecret() error = %v", err)
		}
		if !metadata.Present || metadata.Ref != "vault:providers/openrouter/api-key" || metadata.Kind != "api_key" {
			t.Fatalf("PutSecret() metadata = %#v, want present api_key metadata", metadata)
		}
		record := store.records["vault:providers/openrouter/api-key"]
		if !strings.HasPrefix(record.EncryptedValue, encryptedPrefix) {
			t.Fatalf("stored encrypted value = %q, want %q prefix", record.EncryptedValue, encryptedPrefix)
		}
		if strings.Contains(record.EncryptedValue, "vault-secret-value") {
			t.Fatalf("stored encrypted value leaked plaintext: %q", record.EncryptedValue)
		}

		resolvedVault, err := service.ResolveRef(ctx, "vault:providers/openrouter/api-key")
		if err != nil {
			t.Fatalf("ResolveRef(vault) error = %v", err)
		}
		if resolvedVault != "vault-secret-value" {
			t.Fatalf("ResolveRef(vault) = %q, want plaintext", resolvedVault)
		}
		resolvedEnv, err := service.ResolveRef(ctx, "env:OPENROUTER_API_KEY")
		if err != nil {
			t.Fatalf("ResolveRef(env) error = %v", err)
		}
		if resolvedEnv != "env-secret-value" {
			t.Fatalf("ResolveRef(env) = %q, want env value", resolvedEnv)
		}

		listed, err := service.ListMetadata(ctx, "vault:providers/openrouter/")
		if err != nil {
			t.Fatalf("ListMetadata() error = %v", err)
		}
		if len(listed) != 1 || listed[0].Ref != metadata.Ref || !listed[0].Present {
			t.Fatalf("ListMetadata() = %#v, want one redacted metadata row", listed)
		}
		if err := service.DeleteSecret(ctx, metadata.Ref); err != nil {
			t.Fatalf("DeleteSecret() error = %v", err)
		}
		if _, err := service.ResolveRef(ctx, metadata.Ref); !errors.Is(err, ErrSecretNotFound) {
			t.Fatalf("ResolveRef(deleted) error = %v, want ErrSecretNotFound", err)
		}
	})
}

func TestServiceRejectsUnsupportedAndMissingSecretRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Service) error
		want error
	}{
		{
			name: "Should reject storing env refs",
			run: func(service *Service) error {
				_, err := service.PutSecret(context.Background(), "env:OPENROUTER_API_KEY", "api_key", "secret")
				return err
			},
			want: ErrUnsupportedSecretRef,
		},
		{
			name: "Should reject unsupported refs",
			run: func(service *Service) error {
				_, err := service.ResolveRef(context.Background(), "file:/tmp/secret")
				return err
			},
			want: ErrUnsupportedSecretRef,
		},
		{
			name: "Should report missing env refs",
			run: func(service *Service) error {
				_, err := service.ResolveRef(context.Background(), "env:MISSING_API_KEY")
				return err
			},
			want: ErrMissingSecret,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service, err := NewService(
				newMemoryVaultStore(),
				staticKeyProvider{key: []byte("01234567890123456789012345678901")},
				WithLookupEnv(func(string) (string, bool) { return "", false }),
			)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			if err := tc.run(service); !errors.Is(err, tc.want) {
				t.Fatalf("service operation error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSecretRefValidationSupportsSessionNamespace(t *testing.T) {
	t.Parallel()

	t.Run("Should accept session vault refs and return sessions namespace", func(t *testing.T) {
		t.Parallel()

		ref := "vault:sessions/sess-1/github-token"
		if err := ValidateSecretRef(ref); err != nil {
			t.Fatalf("ValidateSecretRef() error = %v", err)
		}
		namespace, err := SecretRefNamespace(ref)
		if err != nil {
			t.Fatalf("SecretRefNamespace() error = %v", err)
		}
		if namespace != "sessions" {
			t.Fatalf("SecretRefNamespace() = %q, want sessions", namespace)
		}
	})

	t.Run("Should accept session vault prefixes and return sessions namespace", func(t *testing.T) {
		t.Parallel()

		prefix := "vault:sessions/sess-1/"
		if err := ValidateSecretRefPrefix(prefix); err != nil {
			t.Fatalf("ValidateSecretRefPrefix() error = %v", err)
		}
		namespace, err := SecretRefPrefixNamespace(prefix)
		if err != nil {
			t.Fatalf("SecretRefPrefixNamespace() error = %v", err)
		}
		if namespace != "sessions" {
			t.Fatalf("SecretRefPrefixNamespace() = %q, want sessions", namespace)
		}
	})

	t.Run("Should match exact refs or slash-delimited children for prefixes", func(t *testing.T) {
		t.Parallel()

		if !RefMatchesPrefix("vault:sessions/sess-1/github-token", "vault:sessions/sess-1") {
			t.Fatal("RefMatchesPrefix() should match slash-delimited session children")
		}
		if RefMatchesPrefix("vault:sessions/sess-10/github-token", "vault:sessions/sess-1") {
			t.Fatal("RefMatchesPrefix() matched sibling session prefix")
		}
	})

	t.Run("Should reject unsupported vault namespaces", func(t *testing.T) {
		t.Parallel()

		if err := ValidateSecretRef("vault:unknown/sess-1/github-token"); !errors.Is(err, ErrUnsupportedSecretRef) {
			t.Fatalf("ValidateSecretRef() error = %v, want ErrUnsupportedSecretRef", err)
		}
		if err := ValidateSecretRefPrefix("vault:unknown/sess-1/"); !errors.Is(err, ErrUnsupportedSecretRef) {
			t.Fatalf("ValidateSecretRefPrefix() error = %v, want ErrUnsupportedSecretRef", err)
		}
	})
}

func TestFileKeyProviderLoadsEnvAndCreatesKeyFile(t *testing.T) {
	t.Parallel()

	t.Run("Should load supported AGH_VAULT_KEY encodings", func(t *testing.T) {
		t.Parallel()

		rawKey := "01234567890123456789012345678901"
		tests := []struct {
			name  string
			value string
		}{
			{name: "Should load raw key", value: rawKey},
			{name: "Should load base64 key", value: base64.StdEncoding.EncodeToString([]byte(rawKey))},
			{name: "Should load hex key", value: hex.EncodeToString([]byte(rawKey))},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				provider := NewFileKeyProvider(t.TempDir(), func(key string) (string, bool) {
					if key == "AGH_VAULT_KEY" {
						return tc.value, true
					}
					return "", false
				})
				got, err := provider.Key()
				if err != nil {
					t.Fatalf("Key() error = %v", err)
				}
				if string(got) != rawKey {
					t.Fatalf("Key() = %q, want raw key bytes", string(got))
				}
			})
		}
	})

	t.Run("Should create and reuse daemon key file with restricted permissions", func(t *testing.T) {
		t.Parallel()

		homeDir := filepath.Join(t.TempDir(), "agh-home")
		provider := NewFileKeyProvider(homeDir, func(string) (string, bool) { return "", false })
		first, err := provider.Key()
		if err != nil {
			t.Fatalf("Key(first) error = %v", err)
		}
		if len(first) != keySizeBytes {
			t.Fatalf("Key(first) length = %d, want %d", len(first), keySizeBytes)
		}
		info, err := os.Stat(filepath.Join(homeDir, "vault.key"))
		if err != nil {
			t.Fatalf("Stat(vault.key) error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("vault.key permissions = %o, want 0600", got)
		}
		dirInfo, err := os.Stat(homeDir)
		if err != nil {
			t.Fatalf("Stat(homeDir) error = %v", err)
		}
		if got := dirInfo.Mode().Perm(); got != 0o700 {
			t.Fatalf("homeDir permissions = %o, want 0700", got)
		}

		second, err := provider.Key()
		if err != nil {
			t.Fatalf("Key(second) error = %v", err)
		}
		if !bytes.Equal(second, first) {
			t.Fatalf("Key(second) = %x, want reused key %x", second, first)
		}
	})

	t.Run("Should tighten preexisting key directory before creating daemon key file", func(t *testing.T) {
		t.Parallel()

		homeDir := filepath.Join(t.TempDir(), "agh-home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(homeDir) error = %v", err)
		}
		if err := os.Chmod(homeDir, 0o755); err != nil {
			t.Fatalf("Chmod(homeDir) error = %v", err)
		}

		provider := NewFileKeyProvider(homeDir, func(string) (string, bool) { return "", false })
		if _, err := provider.Key(); err != nil {
			t.Fatalf("Key() error = %v", err)
		}

		info, err := os.Stat(homeDir)
		if err != nil {
			t.Fatalf("Stat(homeDir) error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Fatalf("homeDir permissions = %#o, want %#o", got, os.FileMode(0o700))
		}
	})

	t.Run("Should reject preexisting key files with group or other permissions", func(t *testing.T) {
		t.Parallel()

		homeDir := filepath.Join(t.TempDir(), "agh-home")
		keyPath := filepath.Join(homeDir, "vault.key")
		if err := os.MkdirAll(homeDir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", homeDir, err)
		}
		rawKey := []byte("01234567890123456789012345678901")
		if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(rawKey)+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", keyPath, err)
		}

		provider := NewFileKeyProvider(homeDir, func(string) (string, bool) { return "", false })
		key, err := provider.Key()
		if err == nil {
			t.Fatalf("Key() error = nil with key %x, want permission rejection", key)
		}
		if !strings.Contains(err.Error(), "must not grant group or other permissions") {
			t.Fatalf("Key() error = %q, want permission guidance", err)
		}
	})

	t.Run("Should reject preexisting key symlinks", func(t *testing.T) {
		t.Parallel()

		homeDir := filepath.Join(t.TempDir(), "agh-home")
		if err := os.MkdirAll(homeDir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", homeDir, err)
		}
		targetPath := filepath.Join(homeDir, "target.key")
		rawKey := []byte("01234567890123456789012345678901")
		if err := os.WriteFile(targetPath, []byte(base64.StdEncoding.EncodeToString(rawKey)+"\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", targetPath, err)
		}
		keyPath := filepath.Join(homeDir, "vault.key")
		if err := os.Symlink(targetPath, keyPath); err != nil {
			t.Fatalf("Symlink(%q, %q) error = %v", targetPath, keyPath, err)
		}

		provider := NewFileKeyProvider(homeDir, func(string) (string, bool) { return "", false })
		key, err := provider.Key()
		if err == nil {
			t.Fatalf("Key() error = nil with key %x, want symlink rejection", key)
		}
		if !strings.Contains(err.Error(), "must not be a symlink") {
			t.Fatalf("Key() error = %q, want symlink guidance", err)
		}
	})

	t.Run("Should return the persisted key for concurrent first-use callers", func(t *testing.T) {
		t.Parallel()

		homeDir := filepath.Join(t.TempDir(), "agh-home")
		const workers = 128
		start := make(chan struct{})
		results := make([][]byte, workers)
		errs := make([]error, workers)
		var wait sync.WaitGroup
		wait.Add(workers)
		for idx := range workers {
			go func() {
				defer wait.Done()
				<-start
				provider := NewFileKeyProvider(homeDir, func(string) (string, bool) { return "", false })
				results[idx], errs[idx] = provider.Key()
			}()
		}
		close(start)
		wait.Wait()

		for idx, err := range errs {
			if err != nil {
				t.Fatalf("Key(worker %d) error = %v", idx, err)
			}
		}
		payload, err := os.ReadFile(filepath.Join(homeDir, "vault.key"))
		if err != nil {
			t.Fatalf("ReadFile(vault.key) error = %v", err)
		}
		persisted, err := decodeKey(strings.TrimSpace(string(payload)))
		if err != nil {
			t.Fatalf("decodeKey(vault.key) error = %v", err)
		}
		for idx, result := range results {
			if !bytes.Equal(result, persisted) {
				t.Fatalf("Key(worker %d) = %x, want persisted key %x", idx, result, persisted)
			}
		}
	})
}

func TestDecryptValueRejectsMalformedPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		encrypted string
	}{
		{name: "Should reject missing prefix", encrypted: "not-encrypted"},
		{name: "Should reject invalid base64", encrypted: encryptedPrefix + "%%%"},
		{
			name:      "Should reject truncated ciphertext",
			encrypted: encryptedPrefix + base64.StdEncoding.EncodeToString([]byte("short")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := decryptValue([]byte("01234567890123456789012345678901"), tc.encrypted); err == nil {
				t.Fatalf("decryptValue(%q) error = nil, want malformed payload failure", tc.encrypted)
			}
		})
	}
}
