package globaldb

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/vault"
)

func TestGlobalDBVaultSecretsCRUD(t *testing.T) {
	t.Parallel()

	t.Run("Should persist list and delete encrypted vault secret records", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		records := []vault.Record{
			{
				Ref:            "vault:providers/openrouter/api-key",
				Kind:           "api_key",
				EncryptedValue: "aes-gcm:openrouter-ciphertext",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				Ref:            "vault:providers/zai/api-key",
				Kind:           "api_key",
				EncryptedValue: "aes-gcm:zai-ciphertext",
				CreatedAt:      now.Add(time.Minute),
				UpdatedAt:      now.Add(time.Minute),
			},
		}

		for _, record := range records {
			if err := globalDB.PutVaultSecret(ctx, record); err != nil {
				t.Fatalf("PutVaultSecret(%q) error = %v", record.Ref, err)
			}
		}

		got, err := globalDB.GetVaultSecret(ctx, "vault:providers/openrouter/api-key")
		if err != nil {
			t.Fatalf("GetVaultSecret(openrouter) error = %v", err)
		}
		if got.EncryptedValue != "aes-gcm:openrouter-ciphertext" || got.Kind != "api_key" {
			t.Fatalf("GetVaultSecret(openrouter) = %#v, want encrypted record", got)
		}

		listed, err := globalDB.ListVaultSecrets(ctx, "vault:providers/")
		if err != nil {
			t.Fatalf("ListVaultSecrets() error = %v", err)
		}
		if len(listed) != 2 || listed[0].Ref != records[0].Ref || listed[1].Ref != records[1].Ref {
			t.Fatalf("ListVaultSecrets() = %#v, want both refs sorted by ref", listed)
		}

		if err := globalDB.DeleteVaultSecret(ctx, records[0].Ref); err != nil {
			t.Fatalf("DeleteVaultSecret(openrouter) error = %v", err)
		}
		if _, err := globalDB.GetVaultSecret(ctx, records[0].Ref); !errors.Is(err, vault.ErrSecretNotFound) {
			t.Fatalf("GetVaultSecret(deleted) error = %v, want ErrSecretNotFound", err)
		}
	})
}

func TestGlobalDBVaultSecretValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record vault.Record
	}{
		{name: "Should reject empty secret ref", record: vault.Record{EncryptedValue: "aes-gcm:ciphertext"}},
		{
			name:   "Should reject non secret ref",
			record: vault.Record{Ref: "env:OPENROUTER_API_KEY", EncryptedValue: "aes-gcm:ciphertext"},
		},
		{name: "Should reject empty encrypted value", record: vault.Record{Ref: "vault:providers/openrouter/api-key"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)
			if err := globalDB.PutVaultSecret(testutil.Context(t), tc.record); err == nil {
				t.Fatalf("PutVaultSecret(%#v) error = nil, want validation failure", tc.record)
			}
		})
	}
}

func TestGlobalDBVaultSecretUpsert(t *testing.T) {
	t.Parallel()

	t.Run("Should update encrypted value and timestamp on upsert", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		ref := "vault:providers/openrouter/api-key"
		createdAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		first := vault.Record{
			Ref:            ref,
			Kind:           "api_key",
			EncryptedValue: "aes-gcm:first-ciphertext",
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt,
		}
		if err := globalDB.PutVaultSecret(ctx, first); err != nil {
			t.Fatalf("PutVaultSecret(first) error = %v", err)
		}
		updatedAt := createdAt.Add(time.Hour)
		second := vault.Record{
			Ref:            ref,
			Kind:           "api_key",
			EncryptedValue: "aes-gcm:second-ciphertext",
			CreatedAt:      createdAt.Add(-time.Hour),
			UpdatedAt:      updatedAt,
		}
		if err := globalDB.PutVaultSecret(ctx, second); err != nil {
			t.Fatalf("PutVaultSecret(second) error = %v", err)
		}

		got, err := globalDB.GetVaultSecret(ctx, ref)
		if err != nil {
			t.Fatalf("GetVaultSecret(%q) error = %v", ref, err)
		}
		if got.EncryptedValue != second.EncryptedValue || !got.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("GetVaultSecret(%q) = %#v, want latest encrypted value and updated timestamp", ref, got)
		}
	})
}

func TestGlobalDBVaultSecretPrefixFiltering(t *testing.T) {
	t.Parallel()

	t.Run("Should match exact refs and children without matching sibling prefixes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		records := []vault.Record{
			{
				Ref:            "vault:sessions/sess-1/github-token",
				Kind:           "token",
				EncryptedValue: "aes-gcm:sess-1",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				Ref:            "vault:sessions/sess-10/github-token",
				Kind:           "token",
				EncryptedValue: "aes-gcm:sess-10",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}
		for _, record := range records {
			if err := globalDB.PutVaultSecret(ctx, record); err != nil {
				t.Fatalf("PutVaultSecret(%q) error = %v", record.Ref, err)
			}
		}

		listed, err := globalDB.ListVaultSecrets(ctx, "vault:sessions/sess-1")
		if err != nil {
			t.Fatalf("ListVaultSecrets() error = %v", err)
		}
		if len(listed) != 1 || listed[0].Ref != "vault:sessions/sess-1/github-token" {
			t.Fatalf("ListVaultSecrets() = %#v, want only sess-1 child", listed)
		}
	})
}
