package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/extensionenv"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/vault"
)

var _ extensionenv.LifecycleStore = (*ExtensionEnvRepo)(nil)

// ListEnvBindings returns one extension instance's bindings in env-name order.
func (r *ExtensionEnvRepo) ListEnvBindings(
	ctx context.Context,
	extension string,
	profileID string,
	workspaceID string,
) ([]extensionenv.Binding, error) {
	if err := r.checkReady(ctx, "list extension env bindings"); err != nil {
		return nil, err
	}
	name, workspace, err := normalizeExtensionBindingInstance(extension, workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListExtensionEnvBindings(ctx, sqlcgen.ListExtensionEnvBindingsParams{
		ExtensionName: name,
		ProfileID:     strings.TrimSpace(profileID),
		WorkspaceID:   workspace,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"store: list extension env bindings for %s: %w",
			extensionBindingInstanceLabel(name, workspace),
			err,
		)
	}
	result := make([]extensionenv.Binding, 0, len(rows))
	for _, row := range rows {
		binding, err := extensionEnvBindingFromGenerated(row)
		if err != nil {
			return nil, err
		}
		result = append(result, binding)
	}
	return result, nil
}

// PutEnvBinding creates or replaces one extension instance binding.
func (r *ExtensionEnvRepo) PutEnvBinding(ctx context.Context, binding extensionenv.Binding) error {
	if err := r.checkReady(ctx, "put extension env binding"); err != nil {
		return err
	}
	normalized, err := r.normalizeExtensionEnvBinding(binding)
	if err != nil {
		return err
	}
	err = r.queries.UpsertExtensionEnvBinding(ctx, sqlcgen.UpsertExtensionEnvBindingParams{
		ExtensionName: normalized.ExtensionName,
		ProfileID:     normalized.ProfileID,
		WorkspaceID:   normalized.WorkspaceID,
		EnvName:       normalized.EnvName,
		SecretRef:     normalized.SecretRef,
		McpServer:     normalized.MCPServer,
		HeaderName:    normalized.HeaderName,
		Kind:          normalized.Kind,
		CreatedAt:     store.FormatTimestamp(normalized.CreatedAt),
		UpdatedAt:     store.FormatTimestamp(normalized.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf(
			"store: put extension env binding %s env %q: %w",
			extensionBindingInstanceLabel(normalized.ExtensionName, normalized.WorkspaceID),
			normalized.EnvName,
			err,
		)
	}
	return nil
}

// DeleteEnvBinding removes one extension instance binding.
func (r *ExtensionEnvRepo) DeleteEnvBinding(
	ctx context.Context,
	extension, profileID, workspaceID, envName string,
) error {
	if err := r.checkReady(ctx, "delete extension env binding"); err != nil {
		return err
	}
	name, workspace, err := normalizeExtensionBindingInstance(extension, workspaceID)
	if err != nil {
		return err
	}
	env := strings.TrimSpace(envName)
	if !vault.EnvNamePattern.MatchString(env) {
		return fmt.Errorf("store: invalid extension env name %q", env)
	}
	_, err = r.queries.DeleteExtensionEnvBinding(ctx, sqlcgen.DeleteExtensionEnvBindingParams{
		ExtensionName: name, ProfileID: strings.TrimSpace(profileID), WorkspaceID: workspace, EnvName: env,
	})
	if err != nil {
		return fmt.Errorf(
			"store: delete extension env binding %s env %q: %w",
			extensionBindingInstanceLabel(name, workspace),
			env,
			err,
		)
	}
	return nil
}

// DeleteEnvBindings removes all bindings for exactly one extension instance.
func (r *ExtensionEnvRepo) DeleteEnvBindings(
	ctx context.Context,
	extension, profileID, workspaceID string,
) error {
	if err := r.checkReady(ctx, "delete extension env bindings"); err != nil {
		return err
	}
	name, workspace, err := normalizeExtensionBindingInstance(extension, workspaceID)
	if err != nil {
		return err
	}
	_, err = r.queries.DeleteExtensionEnvBindings(ctx, sqlcgen.DeleteExtensionEnvBindingsParams{
		ExtensionName: name, ProfileID: strings.TrimSpace(profileID), WorkspaceID: workspace,
	})
	if err != nil {
		return fmt.Errorf(
			"store: delete extension env bindings for %s: %w",
			extensionBindingInstanceLabel(name, workspace),
			err,
		)
	}
	return nil
}

// CountEnvBindingsBySecretRef reports how many binding rows still reference one Vault ref.
func (r *ExtensionEnvRepo) CountEnvBindingsBySecretRef(ctx context.Context, secretRef string) (int64, error) {
	if err := r.checkReady(ctx, "count extension env bindings by secret ref"); err != nil {
		return 0, err
	}
	ref := vault.NormalizeRef(secretRef)
	if err := vault.ValidateSecretRef(ref); err != nil {
		return 0, err
	}
	count, err := r.queries.CountExtensionEnvBindingsBySecretRef(ctx, ref)
	if err != nil {
		return 0, fmt.Errorf("store: count extension env bindings by secret ref: %w", err)
	}
	return count, nil
}

func (r *ExtensionEnvRepo) normalizeExtensionEnvBinding(
	binding extensionenv.Binding,
) (extensionenv.Binding, error) {
	name, workspace, err := normalizeExtensionBindingInstance(binding.ExtensionName, binding.WorkspaceID)
	if err != nil {
		return extensionenv.Binding{}, err
	}
	binding.ExtensionName = name
	binding.ProfileID = strings.TrimSpace(binding.ProfileID)
	binding.WorkspaceID = workspace
	binding.EnvName = strings.TrimSpace(binding.EnvName)
	binding.SecretRef = vault.NormalizeRef(binding.SecretRef)
	binding.MCPServer = strings.TrimSpace(binding.MCPServer)
	binding.HeaderName = strings.TrimSpace(binding.HeaderName)
	binding.Kind = strings.TrimSpace(binding.Kind)
	if !vault.EnvNamePattern.MatchString(binding.EnvName) {
		return extensionenv.Binding{}, fmt.Errorf("store: invalid extension env name %q", binding.EnvName)
	}
	if err := vault.ValidateSecretRefNamespace(binding.SecretRef, "extensions"); err != nil {
		return extensionenv.Binding{}, err
	}
	if !strings.HasPrefix(binding.SecretRef, vault.ExtensionSecretOwnerPrefix(name, workspace)) {
		return extensionenv.Binding{}, errors.New(
			"store: extension env binding secret ref is outside its instance namespace",
		)
	}
	if binding.Kind != extensionenv.BindingKind {
		return extensionenv.Binding{}, fmt.Errorf(
			"store: extension env binding kind must be %q",
			extensionenv.BindingKind,
		)
	}
	if (binding.MCPServer == "") != (binding.HeaderName == "") {
		return extensionenv.Binding{}, errors.New(
			"store: extension env binding mcp_server and header_name must both be set or both be empty",
		)
	}
	now := r.now().UTC()
	if binding.CreatedAt.IsZero() {
		binding.CreatedAt = now
	}
	if binding.UpdatedAt.IsZero() {
		binding.UpdatedAt = now
	}
	return binding, nil
}

func normalizeExtensionBindingInstance(extension, workspaceID string) (string, string, error) {
	name := strings.TrimSpace(extension)
	if name == "" {
		return "", "", errors.New("store: extension name is required")
	}
	return name, strings.TrimSpace(workspaceID), nil
}

func extensionBindingInstanceLabel(extension, workspaceID string) string {
	if workspaceID == "" {
		return fmt.Sprintf("extension %q (global)", extension)
	}
	return fmt.Sprintf("extension %q (workspace %q)", extension, workspaceID)
}

func extensionEnvBindingFromGenerated(row sqlcgen.ExtensionEnvBinding) (extensionenv.Binding, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return extensionenv.Binding{}, fmt.Errorf(
			"store: parse extension env binding %s env %q created_at: %w",
			extensionBindingInstanceLabel(row.ExtensionName, row.WorkspaceID),
			row.EnvName,
			err,
		)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return extensionenv.Binding{}, fmt.Errorf(
			"store: parse extension env binding %s env %q updated_at: %w",
			extensionBindingInstanceLabel(row.ExtensionName, row.WorkspaceID),
			row.EnvName,
			err,
		)
	}
	return extensionenv.Binding{
		ExtensionName: row.ExtensionName, ProfileID: row.ProfileID,
		WorkspaceID: row.WorkspaceID, EnvName: row.EnvName,
		SecretRef: row.SecretRef, MCPServer: row.McpServer, HeaderName: row.HeaderName,
		Kind: row.Kind, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}
