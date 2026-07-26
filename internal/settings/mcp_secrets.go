package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/vault"
)

const (
	mcpEnvSecretKind         = "mcp_env"
	mcpOAuthMetadataKind     = "mcp_oauth_client_secret"
	mcpSecretRollbackTimeout = 5 * time.Second
)

type mcpSecretMutation struct {
	ref           string
	previousKind  string
	previousValue string
	created       bool
}

type ownedMCPSecretSnapshot struct {
	ref   string
	kind  string
	value string
}

type mcpSecretCleanupPlan struct {
	previousSource mcpSourceEntry
	superseded     []ownedMCPSecretSnapshot
}

type ownedMCPSecretDeleteOutcome struct {
	restorationComplete bool
}

func (s *service) prepareMCPSecretWrites(
	scope ScopeKind,
	workspaceID string,
	serverName string,
	server *aghconfig.MCPServer,
	secrets MCPSecretValues,
) ([]preparedSecretWrite, error) {
	if secrets.Empty() {
		return nil, nil
	}
	if s.providerSecrets == nil {
		return nil, validationError(errors.New("settings: secret store is not available"))
	}
	prefix, err := vault.MCPSecretOwnerPrefix(string(scope), workspaceID, serverName)
	if err != nil {
		return nil, validationError(err)
	}
	envWrites, err := prepareMCPSecretEnvValues(prefix, server, secrets.SecretEnv)
	if err != nil {
		return nil, err
	}
	writes := append([]preparedSecretWrite(nil), envWrites...)
	oauthWrite, ok, err := prepareMCPAuthClientSecretValue(prefix, server, secrets.OAuthClientSecret)
	if err != nil {
		return nil, err
	}
	if ok {
		writes = append(writes, oauthWrite)
	}
	return writes, nil
}

func prepareMCPSecretEnvValues(
	prefix string,
	server *aghconfig.MCPServer,
	values map[string]string,
) ([]preparedSecretWrite, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if server == nil {
		return nil, validationError(errors.New("settings: MCP server is required for secret_env values"))
	}
	if server.EffectiveTransport() != aghconfig.MCPServerTransportStdio {
		return nil, validationError(errors.New("settings: MCP secret_env values require stdio transport"))
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	writes := make([]preparedSecretWrite, 0, len(values))
	for _, key := range keys {
		envName := strings.TrimSpace(key)
		if !vault.EnvNamePattern.MatchString(envName) {
			return nil, validationError(fmt.Errorf("settings: MCP secret_env key %q is invalid", envName))
		}
		value := values[key]
		if strings.TrimSpace(value) == "" {
			return nil, validationError(fmt.Errorf("settings: MCP secret_env value %q is required", envName))
		}
		ref := prefix + "env/" + envName
		setMCPSecretEnvRef(server, envName, ref)
		writes = append(writes, preparedSecretWrite{
			description: fmt.Sprintf("MCP secret_env %q", envName),
			ref:         ref,
			kind:        mcpEnvSecretKind,
			value:       value,
		})
	}
	return writes, nil
}

func prepareMCPAuthClientSecretValue(
	prefix string,
	server *aghconfig.MCPServer,
	value *string,
) (preparedSecretWrite, bool, error) {
	if value == nil {
		return preparedSecretWrite{}, false, nil
	}
	if server == nil {
		return preparedSecretWrite{}, false, validationError(
			errors.New("settings: MCP server is required for OAuth client secret"),
		)
	}
	if strings.TrimSpace(*value) == "" {
		return preparedSecretWrite{}, false, validationError(
			errors.New("settings: MCP OAuth client secret value is required"),
		)
	}
	ref := prefix + "oauth/client-secret"
	server.Auth.ClientSecretRef = ref
	return preparedSecretWrite{
		description: "MCP OAuth client secret",
		ref:         ref,
		kind:        mcpOAuthMetadataKind,
		value:       *value,
	}, true, nil
}

func setMCPSecretEnvRef(server *aghconfig.MCPServer, envName string, ref string) {
	if server.SecretEnv == nil {
		server.SecretEnv = make(map[string]string)
	}
	for key := range server.SecretEnv {
		if strings.TrimSpace(key) == envName {
			delete(server.SecretEnv, key)
		}
	}
	server.SecretEnv[envName] = ref
}

func (s *service) storeMCPSecrets(
	ctx context.Context,
	writes []preparedSecretWrite,
) ([]mcpSecretMutation, error) {
	if len(writes) == 0 {
		return nil, nil
	}
	if s.providerSecrets == nil {
		return nil, validationError(errors.New("settings: secret store is not available"))
	}
	mutations := make([]mcpSecretMutation, 0, len(writes))
	for _, write := range writes {
		mutation, err := s.inspectMCPSecretMutation(ctx, write)
		if err != nil {
			rollbackErr := s.rollbackMCPSecretMutations(ctx, mutations)
			return nil, errors.Join(err, rollbackErr)
		}
		mutations = append(mutations, mutation)
		if _, err := s.providerSecrets.PutSecret(ctx, write.ref, write.kind, write.value); err != nil {
			rollbackErr := s.rollbackMCPSecretMutations(ctx, mutations)
			return nil, errors.Join(fmt.Errorf("settings: store %s: %w", write.description, err), rollbackErr)
		}
	}
	return mutations, nil
}

func (s *service) inspectMCPSecretMutation(
	ctx context.Context,
	write preparedSecretWrite,
) (mcpSecretMutation, error) {
	metadata, err := s.providerSecrets.GetMetadata(ctx, write.ref)
	if errors.Is(err, vault.ErrSecretNotFound) || (err == nil && !metadata.Present) {
		return mcpSecretMutation{ref: write.ref, created: true}, nil
	}
	if err != nil {
		return mcpSecretMutation{}, fmt.Errorf("settings: inspect %s: %w", write.description, err)
	}
	previousValue, err := s.providerSecrets.ResolveRef(ctx, write.ref)
	if err != nil {
		return mcpSecretMutation{}, fmt.Errorf("settings: snapshot %s: %w", write.description, err)
	}
	return mcpSecretMutation{
		ref:           write.ref,
		previousKind:  strings.TrimSpace(metadata.Kind),
		previousValue: previousValue,
	}, nil
}

func (s *service) rollbackMCPSecretMutations(ctx context.Context, mutations []mcpSecretMutation) error {
	if len(mutations) == 0 || s.providerSecrets == nil {
		return nil
	}
	rollbackCtx, cancel := mcpSecretRollbackContext(ctx)
	defer cancel()
	errList := make([]error, 0)
	for _, mutation := range slices.Backward(mutations) {
		if mutation.created {
			if err := s.providerSecrets.DeleteSecret(rollbackCtx, mutation.ref); err != nil &&
				!errors.Is(err, vault.ErrSecretNotFound) {
				errList = append(errList, fmt.Errorf("settings: delete new MCP secret ref %q: %w", mutation.ref, err))
			}
			continue
		}
		if _, err := s.providerSecrets.PutSecret(
			rollbackCtx,
			mutation.ref,
			mutation.previousKind,
			mutation.previousValue,
		); err != nil {
			errList = append(errList, fmt.Errorf("settings: restore MCP secret ref %q: %w", mutation.ref, err))
		}
	}
	return errors.Join(errList...)
}

func mcpSecretRollbackContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), mcpSecretRollbackTimeout)
}

func ownedMCPSecretRefs(prefix string, server aghconfig.MCPServer) map[string]string {
	refs := make(map[string]string)
	for key, rawRef := range server.SecretEnv {
		envName := strings.TrimSpace(key)
		ref := vault.NormalizeRef(rawRef)
		if ref == prefix+"env/"+envName {
			refs[ref] = mcpEnvSecretKind
		}
	}
	oauthRef := vault.NormalizeRef(server.Auth.ClientSecretRef)
	if oauthRef == prefix+"oauth/client-secret" {
		refs[oauthRef] = mcpOAuthMetadataKind
	}
	return refs
}

func referencedMCPSecretRefs(server aghconfig.MCPServer) map[string]struct{} {
	refs := make(map[string]struct{}, len(server.SecretEnv)+1)
	for _, rawRef := range server.SecretEnv {
		if ref := vault.NormalizeRef(rawRef); ref != "" {
			refs[ref] = struct{}{}
		}
	}
	if ref := vault.NormalizeRef(server.Auth.ClientSecretRef); ref != "" {
		refs[ref] = struct{}{}
	}
	return refs
}

func (s *service) prepareSupersededOwnedMCPSecretDeletes(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	previous aghconfig.MCPServer,
	replacement aghconfig.MCPServer,
) ([]ownedMCPSecretSnapshot, error) {
	snapshots, err := s.prepareOwnedMCPSecretDeletes(ctx, scope, workspaceID, previous)
	if err != nil {
		return nil, err
	}
	referenced := referencedMCPSecretRefs(replacement)
	superseded := make([]ownedMCPSecretSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if _, retained := referenced[snapshot.ref]; retained {
			continue
		}
		superseded = append(superseded, snapshot)
	}
	return superseded, nil
}

func (s *service) prepareMCPSecretCleanupPlan(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
	replacement aghconfig.MCPServer,
) (mcpSecretCleanupPlan, error) {
	previousSource, replacing := mcpSourceForTarget(name, target, sources)
	if !replacing {
		return mcpSecretCleanupPlan{}, nil
	}
	superseded, err := s.prepareSupersededOwnedMCPSecretDeletes(
		ctx,
		scope,
		workspaceID,
		previousSource.Server,
		replacement,
	)
	if err != nil {
		return mcpSecretCleanupPlan{}, fmt.Errorf(
			"settings: prepare MCP server %q superseded secret cleanup: %w",
			name,
			err,
		)
	}
	superseded = excludeMCPSecretsReferencedByOtherSources(superseded, name, target, sources)
	return mcpSecretCleanupPlan{previousSource: previousSource, superseded: superseded}, nil
}

func (s *service) executeMCPSecretCleanupPlan(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	root string,
	name string,
	target aghconfig.WriteTarget,
	plan mcpSecretCleanupPlan,
	secretMutations []mcpSecretMutation,
) ([]string, error) {
	if len(plan.superseded) == 0 {
		return nil, nil
	}
	cleanupCtx, cancel := mcpSecretRollbackContext(ctx)
	cleanupOutcome, cleanupErr := s.deleteOwnedMCPSecrets(cleanupCtx, plan.superseded)
	cancel()
	if cleanupErr == nil {
		return nil, nil
	}
	cleanupFailure := fmt.Errorf(
		"settings: garbage-collect MCP server %q superseded secrets: %w",
		name,
		cleanupErr,
	)
	if !cleanupOutcome.restorationComplete {
		return []string{
			"committed MCP replacement retained after partial secret cleanup rollback: " + cleanupFailure.Error(),
		}, nil
	}
	restoreErr := s.writeMCPServer(root, name, target, plan.previousSource.Server)
	if restoreErr != nil {
		restoreErr = fmt.Errorf(
			"settings: restore MCP server %q after superseded secret cleanup failure: %w",
			name,
			restoreErr,
		)
		return []string{
			"committed MCP replacement retained because its prior definition could not be restored: " +
				errors.Join(cleanupFailure, restoreErr).Error(),
		}, nil
	}
	invalidateErr := s.invalidateMCPAuthAfterDefinitionRestore(scope, workspaceID, name)
	rollbackErr := s.rollbackMCPSecretMutations(ctx, secretMutations)
	return nil, errors.Join(
		cleanupFailure,
		invalidateErr,
		rollbackErr,
	)
}

func excludeMCPSecretsReferencedByOtherSources(
	snapshots []ownedMCPSecretSnapshot,
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
) []ownedMCPSecretSnapshot {
	if len(snapshots) == 0 {
		return nil
	}
	referenced := make(map[string]struct{})
	for _, entry := range sources[strings.TrimSpace(name)] {
		if entry.Target == target {
			continue
		}
		for ref := range referencedMCPSecretRefs(entry.Server) {
			referenced[ref] = struct{}{}
		}
	}
	filtered := make([]ownedMCPSecretSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if _, retained := referenced[snapshot.ref]; retained {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	return filtered
}

func (s *service) prepareOwnedMCPSecretDeletes(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
	server aghconfig.MCPServer,
) ([]ownedMCPSecretSnapshot, error) {
	if s.providerSecrets == nil {
		return nil, nil
	}
	prefix, err := vault.MCPSecretOwnerPrefix(string(scope), workspaceID, server.Name)
	if err != nil {
		return nil, err
	}
	owned := ownedMCPSecretRefs(prefix, server)
	refs := make([]string, 0, len(owned))
	for ref := range owned {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	snapshots := make([]ownedMCPSecretSnapshot, 0, len(refs))
	for _, ref := range refs {
		metadata, metadataErr := s.providerSecrets.GetMetadata(ctx, ref)
		if errors.Is(metadataErr, vault.ErrSecretNotFound) {
			continue
		}
		if metadataErr != nil {
			return nil, fmt.Errorf("settings: inspect owned MCP secret %q: %w", ref, metadataErr)
		}
		if !metadata.Present || strings.TrimSpace(metadata.Kind) != owned[ref] {
			continue
		}
		value, resolveErr := s.providerSecrets.ResolveRef(ctx, ref)
		if resolveErr != nil {
			return nil, fmt.Errorf("settings: snapshot owned MCP secret %q: %w", ref, resolveErr)
		}
		snapshots = append(snapshots, ownedMCPSecretSnapshot{ref: ref, kind: owned[ref], value: value})
	}
	return snapshots, nil
}

func (s *service) deleteOwnedMCPSecrets(
	ctx context.Context,
	snapshots []ownedMCPSecretSnapshot,
) (ownedMCPSecretDeleteOutcome, error) {
	for idx, snapshot := range snapshots {
		deleteErr := s.providerSecrets.DeleteSecret(ctx, snapshot.ref)
		if deleteErr == nil || errors.Is(deleteErr, vault.ErrSecretNotFound) {
			continue
		}
		restoreErr := s.restoreOwnedMCPSecrets(ctx, snapshots[:idx+1])
		return ownedMCPSecretDeleteOutcome{restorationComplete: restoreErr == nil}, errors.Join(
			fmt.Errorf("settings: delete owned MCP secret %q: %w", snapshot.ref, deleteErr),
			restoreErr,
		)
	}
	return ownedMCPSecretDeleteOutcome{}, nil
}

func (s *service) restoreOwnedMCPSecrets(ctx context.Context, snapshots []ownedMCPSecretSnapshot) error {
	rollbackCtx, cancel := mcpSecretRollbackContext(ctx)
	defer cancel()
	errList := make([]error, 0)
	for _, snapshot := range snapshots {
		if _, err := s.providerSecrets.PutSecret(rollbackCtx, snapshot.ref, snapshot.kind, snapshot.value); err != nil {
			errList = append(errList, fmt.Errorf("settings: restore owned MCP secret %q: %w", snapshot.ref, err))
		}
	}
	return errors.Join(errList...)
}
