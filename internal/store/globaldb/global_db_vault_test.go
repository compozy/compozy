package globaldb

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
)

func TestGlobalDBVaultSecretsCRUD(t *testing.T) {
	t.Parallel()

	t.Run("Should atomically clear only the matching profile credential requirement", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
		if _, err := globalDB.DB().ExecContext(ctx, `
			INSERT INTO profiles (id, name, color, icon, emoji, state, created_at)
			VALUES
				('00000000000000000000000001', 'marketing', '#5fbf85', 'chart-line', NULL, 'active', ?),
				('00000000000000000000000002', 'engineering', '#8e8eb5', 'code', NULL, 'active', ?)`,
			formatTimestamp(now),
			formatTimestamp(now),
		); err != nil {
			t.Fatalf("insert profiles error = %v", err)
		}
		if _, err := globalDB.DB().ExecContext(ctx, `
			INSERT INTO profile_credential_requirements
				(profile_id, provider, slot, source_extension, declaration_digest, created_at)
			VALUES ('00000000000000000000000002', 'openai', 'api_key', 'engineering-kit', 'digest', ?)`,
			formatTimestamp(now),
		); err != nil {
			t.Fatalf("insert foreign profile credential requirement error = %v", err)
		}
		for _, slot := range []string{"api_key", "organization"} {
			if _, err := globalDB.DB().ExecContext(ctx, `
				INSERT INTO profile_credential_requirements
				(profile_id, provider, slot, source_extension, declaration_digest, created_at)
				VALUES ('00000000000000000000000001', 'openai', ?, 'growth-kit', 'digest', ?)`,
				slot,
				formatTimestamp(now),
			); err != nil {
				t.Fatalf("insert credential requirement %q error = %v", slot, err)
			}
		}
		if err := globalDB.PutVaultSecret(ctx, vault.Record{
			Ref:            "vault:profiles/marketing/providers/openai/api_key",
			Kind:           "api_key",
			EncryptedValue: "aes-gcm-v1:profile-ciphertext",
			CreatedAt:      now,
			UpdatedAt:      now,
		}); err != nil {
			t.Fatalf("PutVaultSecret(profile credential) error = %v", err)
		}
		var remaining int
		if err := globalDB.DB().QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM profile_credential_requirements WHERE profile_id = '00000000000000000000000001'`,
		).Scan(&remaining); err != nil {
			t.Fatalf("count requirements error = %v", err)
		}
		if remaining != 1 {
			t.Fatalf("remaining requirements = %d, want only the unrelated slot", remaining)
		}
		if err := globalDB.DB().QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM profile_credential_requirements
			 WHERE profile_id = '00000000000000000000000002'
			 AND provider = 'openai' AND slot = 'api_key'`,
		).Scan(&remaining); err != nil {
			t.Fatalf("count foreign profile requirement error = %v", err)
		}
		if remaining != 1 {
			t.Fatalf("foreign profile requirements = %d, want matching slot preserved", remaining)
		}
	})

	t.Run("Should persist list and delete encrypted vault secret records", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		records := []vault.Record{
			{
				Ref:            "vault:providers/openrouter/api-key",
				Kind:           "api_key",
				EncryptedValue: "aes-gcm-v1:openrouter-ciphertext",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				Ref:            "vault:providers/zai/api-key",
				Kind:           "api_key",
				EncryptedValue: "aes-gcm-v1:zai-ciphertext",
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
		if got.EncryptedValue != "aes-gcm-v1:openrouter-ciphertext" || got.Kind != "api_key" {
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

func TestGlobalDBVaultCiphertextIdentityEnforcement(t *testing.T) {
	t.Parallel()

	const plaintext = "identity-bound-secret-value"
	tests := []struct {
		name       string
		resolveRef string
		tamper     func(vault.Record) vault.Record
	}{
		{
			name:       "Should reject ciphertext copied to another ref",
			resolveRef: "vault:providers/copied/api-key",
			tamper: func(record vault.Record) vault.Record {
				record.Ref = "vault:providers/copied/api-key"
				return record
			},
		},
		{
			name:       "Should reject ciphertext associated with another kind",
			resolveRef: "vault:providers/source/api-key",
			tamper: func(record vault.Record) vault.Record {
				record.Kind = "refresh_token"
				return record
			},
		},
		{
			name:       "Should reject obsolete ciphertext",
			resolveRef: "vault:providers/source/api-key",
			tamper: func(record vault.Record) vault.Record {
				record.EncryptedValue = "aes-gcm:obsolete"
				return record
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			globalDB := openTestGlobalDB(t)
			service, err := globalDB.vaultService()
			if err != nil {
				t.Fatalf("vaultService() error = %v", err)
			}
			sourceRef := "vault:providers/source/api-key"
			if _, err := service.PutSecret(ctx, sourceRef, "api_key", plaintext); err != nil {
				t.Fatalf("PutSecret() error = %v", err)
			}
			resolved, err := service.ResolveRef(ctx, sourceRef)
			if err != nil {
				t.Fatalf("ResolveRef(source) error = %v", err)
			}
			if resolved != plaintext {
				t.Fatalf("ResolveRef(source) = %q, want original plaintext", resolved)
			}

			record, err := globalDB.GetVaultSecret(ctx, sourceRef)
			if err != nil {
				t.Fatalf("GetVaultSecret(source) error = %v", err)
			}
			if err := globalDB.PutVaultSecret(ctx, tc.tamper(record)); err != nil {
				t.Fatalf("PutVaultSecret(tampered) error = %v", err)
			}

			value, err := service.ResolveRef(ctx, tc.resolveRef)
			if err == nil {
				t.Fatalf("ResolveRef(tampered) = %q, want authentication failure", value)
			}
			if strings.Contains(err.Error(), plaintext) {
				t.Fatalf("ResolveRef(tampered) error leaked plaintext: %v", err)
			}
		})
	}
}

func TestGlobalDBVaultSecretValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record vault.Record
	}{
		{name: "Should reject empty secret ref", record: vault.Record{EncryptedValue: "aes-gcm-v1:ciphertext"}},
		{
			name:   "Should reject non secret ref",
			record: vault.Record{Ref: "env:OPENROUTER_API_KEY", EncryptedValue: "aes-gcm-v1:ciphertext"},
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
			EncryptedValue: "aes-gcm-v1:first-ciphertext",
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
			EncryptedValue: "aes-gcm-v1:second-ciphertext",
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
				EncryptedValue: "aes-gcm-v1:sess-1",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				Ref:            "vault:sessions/sess-10/github-token",
				Kind:           "token",
				EncryptedValue: "aes-gcm-v1:sess-10",
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
