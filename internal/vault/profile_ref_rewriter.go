package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ProfileRefRewrite identifies one persisted ref mutation included in a profile rename.
type ProfileRefRewrite struct {
	Location string
	OldRef   string
	NewRef   string
}

type profileRefSQLQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type profileRefSQLExecutor interface {
	profileRefSQLQueryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type profileRefLocation struct {
	name   string
	table  string
	column string
}

var profileRefLocations = []profileRefLocation{
	{name: "vault_secrets.ref", table: "vault_secrets", column: "ref"},
	{name: "extension_env_bindings.secret_ref", table: "extension_env_bindings", column: "secret_ref"},
	{name: "bridge_secret_bindings.secret_ref", table: "bridge_secret_bindings", column: "secret_ref"},
	{name: "automation_triggers.webhook_secret_ref", table: "automation_triggers", column: "webhook_secret_ref"},
	{name: "mcp_auth_tokens.access_token_ref", table: "mcp_auth_tokens", column: "access_token_ref"},
	{name: "mcp_auth_tokens.refresh_token_ref", table: "mcp_auth_tokens", column: "refresh_token_ref"},
	{name: "mcp_oauth_registrations.client_secret_ref", table: "mcp_oauth_registrations", column: "client_secret_ref"},
	{name: "mcp_oauth_registrations.registration_access_token_ref", table: "mcp_oauth_registrations", column: "registration_access_token_ref"},
}

// ProfileRefRewriter rebinds profile-owned ciphertext and every daemon-owned ref atomically in a caller transaction.
type ProfileRefRewriter struct {
	keys KeyProvider
}

// NewProfileRefRewriter constructs the profile lifecycle ref rewriter.
func NewProfileRefRewriter(keys KeyProvider) (*ProfileRefRewriter, error) {
	if keys == nil {
		return nil, errors.New("vault: profile ref rewriter key provider is required")
	}
	return &ProfileRefRewriter{keys: keys}, nil
}

// ListProfileRefRewrites enumerates every persisted ref occurrence touched by a rename.
func ListProfileRefRewrites(
	ctx context.Context,
	q profileRefSQLQueryer,
	oldName string,
	newName string,
) ([]ProfileRefRewrite, error) {
	oldPrefix, newPrefix, err := profileRenamePrefixes(oldName, newName)
	if err != nil {
		return nil, err
	}
	rewrites := make([]ProfileRefRewrite, 0)
	for _, location := range profileRefLocations {
		query := "SELECT " + location.column + " FROM " + location.table +
			" WHERE " + location.column + " LIKE ? ORDER BY " + location.column
		rows, err := q.QueryContext(ctx, query, oldPrefix+"%")
		if err != nil {
			return nil, fmt.Errorf("vault: list profile ref rewrites from %s: %w", location.name, err)
		}
		for rows.Next() {
			var oldRef string
			if err := rows.Scan(&oldRef); err != nil {
				closeErr := rows.Close()
				return nil, errors.Join(
					fmt.Errorf("vault: scan profile ref rewrite from %s: %w", location.name, err),
					closeErr,
				)
			}
			rewrites = append(rewrites, ProfileRefRewrite{
				Location: location.name,
				OldRef:   oldRef,
				NewRef:   strings.Replace(oldRef, oldPrefix, newPrefix, 1),
			})
		}
		if err := rows.Err(); err != nil {
			closeErr := rows.Close()
			return nil, errors.Join(
				fmt.Errorf("vault: iterate profile ref rewrites from %s: %w", location.name, err),
				closeErr,
			)
		}
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("vault: close profile ref rewrites from %s: %w", location.name, err)
		}
	}
	return rewrites, nil
}

// RewriteProfileRefs re-encrypts vault rows under their new AAD identity and updates every stored ref occurrence.
func (r *ProfileRefRewriter) RewriteProfileRefs(
	ctx context.Context,
	exec profileRefSQLExecutor,
	oldName string,
	newName string,
	updatedAt string,
) error {
	if r == nil || r.keys == nil {
		return errors.New("vault: profile ref rewriter is not configured")
	}
	oldPrefix, newPrefix, err := profileRenamePrefixes(oldName, newName)
	if err != nil {
		return err
	}
	if err := r.rewriteVaultRows(ctx, exec, oldPrefix, newPrefix, updatedAt); err != nil {
		return err
	}
	for _, location := range profileRefLocations[1:] {
		query := "UPDATE " + location.table + " SET " + location.column +
			" = REPLACE(" + location.column + ", ?, ?), updated_at = ? WHERE " + location.column + " LIKE ?"
		if _, err := exec.ExecContext(ctx, query, oldPrefix, newPrefix, updatedAt, oldPrefix+"%"); err != nil {
			return fmt.Errorf("vault: rewrite profile refs in %s: %w", location.name, err)
		}
	}
	return nil
}

func (r *ProfileRefRewriter) rewriteVaultRows(
	ctx context.Context,
	exec profileRefSQLExecutor,
	oldPrefix string,
	newPrefix string,
	updatedAt string,
) error {
	rows, err := exec.QueryContext(
		ctx,
		`SELECT ref, kind, encrypted_value FROM vault_secrets WHERE ref LIKE ? ORDER BY ref`,
		oldPrefix+"%",
	)
	if err != nil {
		return fmt.Errorf("vault: list encrypted profile refs: %w", err)
	}
	type encryptedRewrite struct{ oldRef, newRef, kind, value string }
	values := make([]encryptedRewrite, 0)
	for rows.Next() {
		var value encryptedRewrite
		if err := rows.Scan(&value.oldRef, &value.kind, &value.value); err != nil {
			closeErr := rows.Close()
			return errors.Join(fmt.Errorf("vault: scan encrypted profile ref: %w", err), closeErr)
		}
		value.newRef = strings.Replace(value.oldRef, oldPrefix, newPrefix, 1)
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		closeErr := rows.Close()
		return errors.Join(fmt.Errorf("vault: iterate encrypted profile refs: %w", err), closeErr)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("vault: close encrypted profile refs: %w", err)
	}
	if len(values) == 0 {
		return nil
	}
	key, err := r.keys.Key()
	if err != nil {
		return fmt.Errorf("vault: load key for profile ref rewrite: %w", err)
	}
	for _, value := range values {
		oldIdentity, err := newCiphertextIdentity(value.oldRef, value.kind)
		if err != nil {
			return err
		}
		plaintext, err := decryptValue(key, value.value, oldIdentity)
		if err != nil {
			return fmt.Errorf("vault: decrypt profile ref %s for rename: %w", value.oldRef, err)
		}
		newIdentity, err := newCiphertextIdentity(value.newRef, value.kind)
		if err != nil {
			return err
		}
		encrypted, err := encryptValue(key, plaintext, newIdentity)
		if err != nil {
			return fmt.Errorf("vault: encrypt profile ref %s for rename: %w", value.newRef, err)
		}
		if _, err := exec.ExecContext(
			ctx,
			`UPDATE vault_secrets SET ref = ?, encrypted_value = ?, updated_at = ? WHERE ref = ?`,
			value.newRef,
			encrypted,
			updatedAt,
			value.oldRef,
		); err != nil {
			return fmt.Errorf("vault: persist renamed profile ref %s: %w", value.newRef, err)
		}
	}
	return nil
}

func profileRenamePrefixes(oldName, newName string) (string, string, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if !profileSecretNamePattern.MatchString(oldName) || !profileSecretNamePattern.MatchString(newName) {
		return "", "", fmt.Errorf("%w: invalid profile name for secret ref rewrite", ErrUnsupportedSecretRef)
	}
	return ProfileSecretRefPrefix + oldName + "/", ProfileSecretRefPrefix + newName + "/", nil
}
