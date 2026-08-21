package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/resources"
)

const hookBindingManagedIDPrefix = "daemon.sync.hook_binding."

type hookBindingPublisher interface {
	Sync(context.Context) error
}

type hookBindingDeclarationProvider = hookspkg.DeclarationProvider

type hookBindingPublisherFunc func(context.Context) error

func (f hookBindingPublisherFunc) Sync(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

type hookBindingSourceSyncer struct {
	store     resources.Store[hookspkg.HookDecl]
	codec     resources.KindCodec[hookspkg.HookDecl]
	actor     resources.MutationActor
	logger    *slog.Logger
	trigger   func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	providers []hookBindingDeclarationProvider
}

func newHookBindingSourceSyncer(
	store resources.Store[hookspkg.HookDecl],
	codec resources.KindCodec[hookspkg.HookDecl],
	actor resources.MutationActor,
	logger *slog.Logger,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
	providers ...hookBindingDeclarationProvider,
) hookBindingPublisher {
	if store == nil || codec == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &hookBindingSourceSyncer{
		store:     store,
		codec:     codec,
		actor:     actor,
		logger:    logger,
		trigger:   trigger,
		providers: append([]hookBindingDeclarationProvider(nil), providers...),
	}
}

func hookBindingSyncActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "hook-binding-sync",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "hook-binding-sync",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}
}

func (s *hookBindingSourceSyncer) Sync(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: hook binding sync context is required")
	}

	desired, err := s.desiredBindings(ctx)
	if err != nil {
		return err
	}

	source := s.actor.Source
	current, err := s.store.List(ctx, s.actor, resources.ResourceFilter{
		Source: &source,
	})
	if err != nil {
		return fmt.Errorf("daemon: list managed hook bindings: %w", err)
	}

	currentByID := make(map[string]resources.Record[hookspkg.HookDecl], len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	changed := false
	for id, desiredBinding := range desired {
		existing, ok := currentByID[id]
		if ok && s.sameBinding(
			existing,
			desiredBinding.scope,
			desiredBinding.owner,
			desiredBinding.spec,
		) {
			delete(currentByID, id)
			continue
		}

		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := s.store.Put(ctx, s.actor, resources.Draft[hookspkg.HookDecl]{
			ID:              desiredBinding.id,
			Scope:           desiredBinding.scope,
			Owner:           desiredBinding.owner,
			ExpectedVersion: expectedVersion,
			Spec:            desiredBinding.spec,
		}); err != nil {
			return fmt.Errorf("daemon: sync hook binding %q: %w", desiredBinding.id, err)
		}
		changed = true
		delete(currentByID, id)
	}

	for _, stale := range currentByID {
		if err := s.store.Delete(ctx, s.actor, stale.ID, stale.Version); err != nil {
			return fmt.Errorf("daemon: delete stale hook binding %q: %w", stale.ID, err)
		}
		changed = true
	}

	if changed && s.trigger != nil {
		if err := s.trigger(ctx, hookBindingResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}
	return nil
}

type desiredHookBinding struct {
	id    string
	scope resources.ResourceScope
	owner *resources.ResourceOwner
	spec  hookspkg.HookDecl
}

func (s *hookBindingSourceSyncer) desiredBindings(ctx context.Context) (map[string]*desiredHookBinding, error) {
	bindings := make(map[string]*desiredHookBinding)
	for _, provider := range s.providers {
		if provider == nil {
			continue
		}
		decls, err := provider(ctx)
		if err != nil {
			return nil, err
		}
		for _, decl := range decls {
			scope := hookBindingScope(decl)
			spec, err := validateHookBindingSpec(ctx, scope, decl)
			if err != nil {
				return nil, err
			}
			owner := extensionHookOwner(spec)
			id, err := s.bindingID(scope, spec, owner)
			if err != nil {
				return nil, err
			}
			bindings[id] = &desiredHookBinding{
				id:    id,
				scope: scope,
				owner: owner,
				spec:  spec,
			}
		}
	}
	return bindings, nil
}

func (s *hookBindingSourceSyncer) bindingID(
	scope resources.ResourceScope,
	spec hookspkg.HookDecl,
	owner *resources.ResourceOwner,
) (string, error) {
	if owner != nil {
		return extensionHookBindingID(owner.ID, spec.Name), nil
	}
	encoded, err := s.codec.Encode(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(string(scope.Kind) + "\x00" + scope.ID + "\x00" + string(encoded)))
	return hookBindingManagedIDPrefix + strings.ToLower(spec.Source.String()) + "." + hex.EncodeToString(sum[:12]), nil
}

func hookBindingScope(decl hookspkg.HookDecl) resources.ResourceScope {
	workspaceID := strings.TrimSpace(decl.Matcher.WorkspaceID)
	if workspaceID != "" {
		return resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspace,
			ID:   workspaceID,
		}
	}
	return resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
}

func hookCloneDeclarations(decls []hookspkg.HookDecl) []hookspkg.HookDecl {
	if len(decls) == 0 {
		return nil
	}
	cloned := make([]hookspkg.HookDecl, 0, len(decls))
	for _, decl := range decls {
		cloned = append(cloned, cloneDaemonHookDecl(decl))
	}
	return cloned
}

func (s *hookBindingSourceSyncer) sameBinding(
	record resources.Record[hookspkg.HookDecl],
	scope resources.ResourceScope,
	owner *resources.ResourceOwner,
	spec hookspkg.HookDecl,
) bool {
	if record.Scope != scope {
		return false
	}
	if owner != nil && record.Owner.Normalize() != owner.Normalize() {
		return false
	}

	currentEncoded, err := s.codec.Encode(record.Spec)
	if err != nil {
		return false
	}
	desiredEncoded, err := s.codec.Encode(spec)
	if err != nil {
		return false
	}
	return bytes.Equal(currentEncoded, desiredEncoded)
}

func extensionHookBindingID(extensionName string, hookName string) string {
	return "extension/" + strings.TrimSpace(extensionName) + "/hook.binding/" + strings.TrimSpace(hookName)
}

func extensionHookOwner(spec hookspkg.HookDecl) *resources.ResourceOwner {
	if spec.Source != hookspkg.HookSourceExtension {
		return nil
	}
	name := strings.TrimSpace(spec.Metadata["extension"])
	if name == "" {
		return nil
	}
	return extensionOwner(name)
}
