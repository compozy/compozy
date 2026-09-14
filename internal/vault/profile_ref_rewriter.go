package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	//nolint:gosec // These values are SQL table identifiers, not credentials.
	mcpAuthTokensTable         = "mcp_auth_tokens"
	mcpOAuthRegistrationsTable = "mcp_oauth_registrations"
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

type profileRefPrefix struct {
	old string
	new string
}

var profileRefLocations = []profileRefLocation{
	{name: "vault_secrets.ref", table: "vault_secrets", column: "ref"},
	{name: "extension_env_bindings.secret_ref", table: "extension_env_bindings", column: "secret_ref"},
	{name: "bridge_secret_bindings.secret_ref", table: "bridge_secret_bindings", column: "secret_ref"},
	{name: "automation_triggers.webhook_secret_ref", table: "automation_triggers", column: "webhook_secret_ref"},
	{name: "mcp_auth_tokens.access_token_ref", table: mcpAuthTokensTable, column: "access_token_ref"},
	{name: "mcp_auth_tokens.refresh_token_ref", table: mcpAuthTokensTable, column: "refresh_token_ref"},
	{name: "mcp_oauth_registrations.client_secret_ref", table: mcpOAuthRegistrationsTable, column: "client_secret_ref"},
	{
		name:   "mcp_oauth_registrations.registration_access_token_ref",
		table:  mcpOAuthRegistrationsTable,
		column: "registration_access_token_ref",
	},
}

var profileMCPRowLocations = []profileRefLocation{
	{name: "mcp_auth_tokens.workspace_id", table: mcpAuthTokensTable, column: "workspace_id"},
	{name: "mcp_oauth_registrations.workspace_id", table: mcpOAuthRegistrationsTable, column: "workspace_id"},
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
	prefixes, err := profileRenameRefPrefixes(ctx, q, oldName, newName)
	if err != nil {
		return nil, err
	}
	rewrites := make([]ProfileRefRewrite, 0)
	locationRewrites, err := listProfileRefLocationRewrites(ctx, q, prefixes)
	if err != nil {
		return nil, err
	}
	rewrites = append(rewrites, locationRewrites...)
	ownerRewrites, err := listProfileMCPOwnerRewrites(ctx, q, oldName, newName)
	if err != nil {
		return nil, err
	}
	return append(rewrites, ownerRewrites...), nil
}

func listProfileRefLocationRewrites(
	ctx context.Context,
	q profileRefSQLQueryer,
	prefixes []profileRefPrefix,
) ([]ProfileRefRewrite, error) {
	rewrites := make([]ProfileRefRewrite, 0)
	for _, location := range profileRefLocations {
		for _, prefix := range prefixes {
			query := "SELECT " + location.column + " FROM " + location.table +
				" WHERE SUBSTR(" + location.column + ", 1, LENGTH(?)) = ? ORDER BY " + location.column
			rows, queryErr := q.QueryContext(ctx, query, prefix.old, prefix.old)
			if queryErr != nil {
				return nil, fmt.Errorf("vault: list profile ref rewrites from %s: %w", location.name, queryErr)
			}
			for rows.Next() {
				var oldRef string
				if scanErr := rows.Scan(&oldRef); scanErr != nil {
					return nil, errors.Join(
						fmt.Errorf("vault: scan profile ref rewrite from %s: %w", location.name, scanErr),
						rows.Close(),
					)
				}
				rewrites = append(rewrites, ProfileRefRewrite{
					Location: location.name,
					OldRef:   oldRef,
					NewRef:   strings.Replace(oldRef, prefix.old, prefix.new, 1),
				})
			}
			if rowsErr := rows.Err(); rowsErr != nil {
				return nil, errors.Join(
					fmt.Errorf("vault: iterate profile ref rewrites from %s: %w", location.name, rowsErr),
					rows.Close(),
				)
			}
			if closeErr := rows.Close(); closeErr != nil {
				return nil, fmt.Errorf("vault: close profile ref rewrites from %s: %w", location.name, closeErr)
			}
		}
	}
	return rewrites, nil
}

func listProfileMCPOwnerRewrites(
	ctx context.Context,
	q profileRefSQLQueryer,
	oldName string,
	newName string,
) ([]ProfileRefRewrite, error) {
	rewrites := make([]ProfileRefRewrite, 0)
	for _, location := range profileMCPRowLocations {
		query := "SELECT scope, workspace_id FROM " + location.table +
			" WHERE (scope = 'profile' AND workspace_id = ?) OR" +
			" (scope = 'workspace_profile' AND SUBSTR(workspace_id, -LENGTH(?)) = ?) ORDER BY scope, workspace_id"
		suffix := "@pf:" + strings.TrimSpace(oldName)
		rows, queryErr := q.QueryContext(ctx, query, strings.TrimSpace(oldName), suffix, suffix)
		if queryErr != nil {
			return nil, fmt.Errorf("vault: list profile MCP owner rewrites from %s: %w", location.name, queryErr)
		}
		for rows.Next() {
			var scope, owner string
			if scanErr := rows.Scan(&scope, &owner); scanErr != nil {
				closeErr := rows.Close()
				return nil, errors.Join(
					fmt.Errorf("vault: scan profile MCP owner rewrite from %s: %w", location.name, scanErr),
					closeErr,
				)
			}
			newOwner := strings.TrimSpace(newName)
			if scope == MCPWorkspaceProfileScope {
				newOwner = strings.TrimSuffix(owner, suffix) + "@pf:" + newOwner
			}
			rewrites = append(rewrites, ProfileRefRewrite{Location: location.name, OldRef: owner, NewRef: newOwner})
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			closeErr := rows.Close()
			return nil, errors.Join(
				fmt.Errorf("vault: iterate profile MCP owner rewrites from %s: %w", location.name, rowsErr),
				closeErr,
			)
		}
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("vault: close profile MCP owner rewrites from %s: %w", location.name, closeErr)
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
	prefixes, err := profileRenameRefPrefixes(ctx, exec, oldName, newName)
	if err != nil {
		return err
	}
	for _, prefix := range prefixes {
		if err := r.rewriteVaultRows(ctx, exec, prefix.old, prefix.new, updatedAt); err != nil {
			return err
		}
		for _, location := range profileRefLocations[1:] {
			query := "UPDATE " + location.table + " SET " + location.column +
				" = ? || SUBSTR(" + location.column + ", LENGTH(?) + 1), updated_at = ?" +
				" WHERE SUBSTR(" + location.column + ", 1, LENGTH(?)) = ?"
			if _, execErr := exec.ExecContext(
				ctx,
				query,
				prefix.new,
				prefix.old,
				updatedAt,
				prefix.old,
				prefix.old,
			); execErr != nil {
				return fmt.Errorf("vault: rewrite profile refs in %s: %w", location.name, execErr)
			}
		}
	}
	for _, location := range profileMCPRowLocations {
		query := "UPDATE " + location.table +
			" SET workspace_id = CASE WHEN scope = 'profile' THEN ? ELSE" +
			" SUBSTR(workspace_id, 1, LENGTH(workspace_id) - LENGTH(?)) || ? END, updated_at = ?" +
			" WHERE (scope = 'profile' AND workspace_id = ?) OR" +
			" (scope = 'workspace_profile' AND SUBSTR(workspace_id, -LENGTH(?)) = ?)"
		oldSuffix := "@pf:" + strings.TrimSpace(oldName)
		if _, err := exec.ExecContext(
			ctx, query, strings.TrimSpace(newName), oldSuffix, "@pf:"+strings.TrimSpace(newName), updatedAt,
			strings.TrimSpace(oldName), oldSuffix, oldSuffix,
		); err != nil {
			return fmt.Errorf("vault: rewrite profile MCP owner in %s: %w", location.name, err)
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
		`SELECT ref, kind, encrypted_value FROM vault_secrets WHERE SUBSTR(ref, 1, LENGTH(?)) = ? ORDER BY ref`,
		oldPrefix, oldPrefix,
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

func mcpProfileRenamePrefixes(oldName, newName string) (string, string, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if !profileSecretNamePattern.MatchString(oldName) || !profileSecretNamePattern.MatchString(newName) {
		return "", "", fmt.Errorf("%w: invalid profile name for MCP secret ref rewrite", ErrUnsupportedSecretRef)
	}
	oldSegment, err := MCPOwnerSegment(oldName)
	if err != nil {
		return "", "", fmt.Errorf("vault: encode old MCP profile owner: %w", err)
	}
	newSegment, err := MCPOwnerSegment(newName)
	if err != nil {
		return "", "", fmt.Errorf("vault: encode new MCP profile owner: %w", err)
	}
	return "vault:mcp/profile/" + oldSegment + "/", "vault:mcp/profile/" + newSegment + "/", nil
}
