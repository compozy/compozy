package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

const (
	SessionCreationCodeInvalid          = "session_creation_invalid"
	SessionCreationCodeIdentityMismatch = "session_creation_identity_mismatch"
	SessionCreationCodeEffectUnknown    = "session_creation_effect_unknown"
	SessionCreationCodeStoreUnavailable = "session_creation_store_unavailable"
)

// EnsureCreatedOutcome is the exhaustive idempotent creation result.
type EnsureCreatedOutcome string

const (
	EnsureCreatedOutcomeCreated       EnsureCreatedOutcome = "created"
	EnsureCreatedOutcomeExistingMatch EnsureCreatedOutcome = "existing-match"
)

// EnsureCreatedResult identifies the created or identity-matched session.
type EnsureCreatedResult struct {
	SessionID string
	Outcome   EnsureCreatedOutcome
}

// CreationError carries typed session-creation effect certainty through errors.As.
type CreationError struct {
	Effect EffectCertainty
	Code   string
	Err    error
}

func (e *CreationError) Error() string {
	if e == nil {
		return "session: creation failed"
	}
	if e.Err == nil {
		return fmt.Sprintf("session: creation failed (%s, %s)", e.Code, e.Effect)
	}
	return fmt.Sprintf("session: creation failed (%s, %s): %v", e.Code, e.Effect, e.Err)
}

func (e *CreationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IdempotentSessionCreator creates one preallocated session or reuses its exact identity.
type IdempotentSessionCreator interface {
	EnsureCreated(context.Context, CreateOpts) (EnsureCreatedResult, error)
}

var _ IdempotentSessionCreator = (*Manager)(nil)

// EnsureCreated creates one preallocated session exactly once by immutable creation identity.
func (m *Manager) EnsureCreated(ctx context.Context, opts CreateOpts) (EnsureCreatedResult, error) {
	if ctx == nil {
		return EnsureCreatedResult{}, creationError(
			EffectKnownFalse,
			SessionCreationCodeInvalid,
			errors.New("session: ensure created context is required"),
		)
	}
	if m == nil || m.creationStore == nil {
		return EnsureCreatedResult{}, creationError(
			EffectKnownFalse,
			SessionCreationCodeStoreUnavailable,
			errors.New("session: creation store is unavailable"),
		)
	}
	desiredID := strings.TrimSpace(opts.DesiredSessionID)
	if desiredID == "" || opts.CreationProfile == nil || opts.CreationIdentity == nil {
		return EnsureCreatedResult{}, creationError(
			EffectKnownFalse,
			SessionCreationCodeInvalid,
			errors.New("session: desired id, creation profile, and creation identity are required"),
		)
	}
	if err := validateRequestedCreationIdentity(opts); err != nil {
		return EnsureCreatedResult{}, creationError(
			EffectKnownFalse,
			SessionCreationCodeIdentityMismatch,
			err,
		)
	}

	matched, err := m.matchExistingCreation(ctx, desiredID, *opts.CreationIdentity)
	if err != nil {
		return EnsureCreatedResult{}, err
	}
	if matched {
		return EnsureCreatedResult{SessionID: desiredID, Outcome: EnsureCreatedOutcomeExistingMatch}, nil
	}

	spec, err := m.prepareCreateStart(ctx, opts)
	if err != nil {
		return EnsureCreatedResult{}, creationError(
			EffectKnownFalse,
			SessionCreationCodeInvalid,
			err,
		)
	}
	created, err := m.startSession(ctx, &spec)
	if err != nil {
		return EnsureCreatedResult{}, creationError(
			EffectUnknown,
			SessionCreationCodeEffectUnknown,
			err,
		)
	}
	if created == nil || created.Info() == nil || strings.TrimSpace(created.Info().ID) != desiredID {
		return EnsureCreatedResult{}, creationError(
			EffectUnknown,
			SessionCreationCodeEffectUnknown,
			errors.New("session: created session identity is unavailable"),
		)
	}
	return EnsureCreatedResult{SessionID: desiredID, Outcome: EnsureCreatedOutcomeCreated}, nil
}

func (m *Manager) matchExistingCreation(
	ctx context.Context,
	sessionID string,
	expected store.SessionCreationIdentity,
) (bool, error) {
	stored, err := m.creationStore.GetSessionCreationIdentity(ctx, sessionID)
	if err == nil {
		if stored != expected {
			return false, creationError(
				EffectKnownFalse,
				SessionCreationCodeIdentityMismatch,
				store.ErrSessionCreationIdentityMismatch,
			)
		}
		return m.ensureExistingSessionActive(ctx, sessionID)
	}
	if !errors.Is(err, store.ErrSessionNotFound) {
		code := SessionCreationCodeEffectUnknown
		effect := EffectUnknown
		if errors.Is(err, store.ErrSessionCreationIdentityMismatch) {
			code = SessionCreationCodeIdentityMismatch
			effect = EffectKnownFalse
		}
		return false, creationError(effect, code, err)
	}

	meta, metaErr := m.readMetaWithContext(ctx, sessionID)
	if metaErr != nil {
		if errors.Is(metaErr, os.ErrNotExist) || errors.Is(metaErr, ErrSessionNotFound) {
			return false, nil
		}
		return false, creationError(EffectUnknown, SessionCreationCodeEffectUnknown, metaErr)
	}
	if creationIdentityFromMeta(meta) == nil {
		return false, creationError(
			EffectKnownFalse,
			SessionCreationCodeIdentityMismatch,
			errors.New("session: existing metadata has no creation identity"),
		)
	}
	if *creationIdentityFromMeta(meta) != expected {
		return false, creationError(
			EffectKnownFalse,
			SessionCreationCodeIdentityMismatch,
			store.ErrSessionCreationIdentityMismatch,
		)
	}
	if err := m.persistSessionCatalogFromMeta(ctx, meta); err != nil {
		return false, creationError(EffectUnknown, SessionCreationCodeEffectUnknown, err)
	}
	return m.ensureExistingSessionActive(ctx, sessionID)
}

func (m *Manager) ensureExistingSessionActive(ctx context.Context, sessionID string) (bool, error) {
	if _, ok := m.Get(sessionID); ok {
		return true, nil
	}
	if _, err := m.Resume(ctx, sessionID); err != nil {
		return false, creationError(EffectUnknown, SessionCreationCodeEffectUnknown, err)
	}
	return true, nil
}

func prepareStartCreationIdentity(spec *sessionStartSpec, resolved aghconfig.ResolvedAgent) error {
	if spec == nil {
		return errors.New("session: start spec is required for creation identity")
	}
	actual := creationProfileFromStart(spec, resolved)
	if err := actual.Validate(); err != nil {
		return err
	}
	identity, err := identityForCreationProfile(actual, spec)
	if err != nil {
		return err
	}
	if spec.creationIdentityPinned {
		if spec.creationProfile == nil || spec.creationIdentity == nil {
			return errors.New("session: pinned creation profile and identity must be supplied together")
		}
		actualJSON, err := actual.CanonicalJSON()
		if err != nil {
			return err
		}
		expectedJSON, err := spec.creationProfile.CanonicalJSON()
		if err != nil {
			return err
		}
		if !bytes.Equal(actualJSON, expectedJSON) || identity != *spec.creationIdentity {
			return fmt.Errorf(
				"%w: effective session creation profile changed",
				store.ErrSessionCreationIdentityMismatch,
			)
		}
	}
	spec.creationProfile = cloneCreationProfile(&actual)
	spec.creationOptions = creationOptionsForSpec(spec)
	spec.creationIdentity = cloneCreationIdentity(&identity)
	return nil
}

func prepareStartCreationIdentityIfEnabled(
	spec *sessionStartSpec,
	resolved aghconfig.ResolvedAgent,
) error {
	if spec.startAction != sessionStartActionCreate || !spec.creationIdentityEnabled {
		return nil
	}
	return prepareStartCreationIdentity(spec, resolved)
}

func finalizeStartCreationIdentity(spec *sessionStartSpec, session *Session) error {
	if spec == nil || session == nil || spec.creationProfile == nil || spec.creationIdentity == nil {
		return errors.New("session: creation identity is unavailable before start")
	}
	profile := store.NormalizeSessionCreationProfile(*spec.creationProfile)
	effectivePermissions := session.effectivePermissions()
	if spec.creationIdentityPinned && profile.Permissions != effectivePermissions {
		return fmt.Errorf("%w: effective session permissions changed", store.ErrSessionCreationIdentityMismatch)
	}
	profile.Permissions = effectivePermissions
	identity, err := identityForCreationProfile(profile, spec)
	if err != nil {
		return err
	}
	if spec.creationIdentityPinned && identity != *spec.creationIdentity {
		return store.ErrSessionCreationIdentityMismatch
	}
	spec.creationProfile = cloneCreationProfile(&profile)
	if spec.creationOptions == nil {
		spec.creationOptions = creationOptionsForSpec(spec)
	}
	spec.creationIdentity = cloneCreationIdentity(&identity)
	session.mu.Lock()
	session.creationProfile = cloneCreationProfile(&profile)
	session.creationOptions = cloneCreationOptions(spec.creationOptions)
	session.creationIdentity = cloneCreationIdentity(&identity)
	session.mu.Unlock()
	return nil
}

func finalizeStartCreationIdentityIfEnabled(spec *sessionStartSpec, session *Session) error {
	if spec.startAction != sessionStartActionCreate || !spec.creationIdentityEnabled {
		return nil
	}
	return finalizeStartCreationIdentity(spec, session)
}

func creationProfileFromStart(
	spec *sessionStartSpec,
	resolved aghconfig.ResolvedAgent,
) store.SessionCreationProfile {
	sandboxMode := store.SessionCreationSandboxRef
	sandboxRef := strings.TrimSpace(spec.workspace.SandboxRef)
	if sandboxRef == "" {
		sandboxRef = strings.TrimSpace(spec.workspace.Config.Defaults.Sandbox)
	}
	if spec.sandboxDisabled || sandboxRef == "" {
		sandboxMode = store.SessionCreationSandboxNone
		sandboxRef = ""
	}
	return BuildCreationProfile(CreationProfileInput{
		AgentName:       resolved.Name,
		Provider:        resolved.Provider,
		Model:           resolved.Model,
		ReasoningEffort: spec.reasoningEffort,
		WorkspaceID:     spec.workspace.ID,
		CWD:             spec.cwd,
		SandboxMode:     sandboxMode,
		SandboxRef:      sandboxRef,
		Permissions:     startSpecPermissions(spec, resolved.Permissions),
		AllowedTools:    spec.allowedToolsOverride,
		AgentTools:      resolved.Tools,
		AgentToolsets:   resolved.Toolsets,
		DeniedTools:     resolved.DenyTools,
		RuntimeMode:     spec.runtimeMode,
		PromptOverlay:   spec.promptOverlay,
		ContractOverlay: spec.contractOverlay,
	})
}

func identityForCreationProfile(
	profile store.SessionCreationProfile,
	spec *sessionStartSpec,
) (store.SessionCreationIdentity, error) {
	profileRef, err := profile.Ref()
	if err != nil {
		return store.SessionCreationIdentity{}, err
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return store.SessionCreationIdentity{}, err
	}
	creationOptions := creationOptionsForSpec(spec)
	if creationOptions == nil {
		return store.SessionCreationIdentity{}, errors.New("session: creation options are required")
	}
	creationDigest, err := profile.CreationDigest(*creationOptions)
	if err != nil {
		return store.SessionCreationIdentity{}, err
	}
	return store.SessionCreationIdentity{
		CreationProfileRef: profileRef,
		PolicySpecDigest:   policyDigest,
		CreationDigest:     creationDigest,
	}, nil
}

func creationOptionsForSpec(spec *sessionStartSpec) *store.SessionCreationOptions {
	if spec == nil {
		return nil
	}
	if spec.creationOptions != nil {
		return cloneCreationOptions(spec.creationOptions)
	}
	return &store.SessionCreationOptions{
		SessionID:            strings.TrimSpace(spec.sessionID),
		Name:                 strings.TrimSpace(spec.sessionName),
		NetworkOwnerKey:      strings.TrimSpace(spec.networkOwnerKey),
		NetworkParticipation: spec.networkParticipation,
		SessionType:          string(normalizeSessionType(spec.sessionType)),
	}
}

func validateRequestedCreationIdentity(opts CreateOpts) error {
	profile := cloneCreationProfile(opts.CreationProfile)
	identity := cloneCreationIdentity(opts.CreationIdentity)
	if profile == nil || identity == nil {
		return errors.New("session: creation profile and identity are required")
	}
	networkParticipation := participation.LocalSpec()
	if opts.ResolvedNetworkParticipation != nil {
		networkParticipation = *opts.ResolvedNetworkParticipation
	}
	spec := &sessionStartSpec{
		sessionID:            strings.TrimSpace(opts.DesiredSessionID),
		sessionName:          strings.TrimSpace(opts.Name),
		networkOwnerKey:      strings.TrimSpace(opts.NetworkOwnerKey),
		networkParticipation: networkParticipation,
		sessionType:          normalizeSessionType(opts.Type),
	}
	if spec.networkOwnerKey == "" {
		spec.networkOwnerKey = participation.OwnerKey(participation.OwnerRef{
			Kind: participation.OwnerKindSession,
			ID:   spec.sessionID,
		})
	}
	computed, err := identityForCreationProfile(*profile, spec)
	if err != nil {
		return err
	}
	if computed != *identity {
		return store.ErrSessionCreationIdentityMismatch
	}
	return nil
}

func creationIdentityFromMeta(meta store.SessionMeta) *store.SessionCreationIdentity {
	identity := &store.SessionCreationIdentity{
		CreationProfileRef: strings.TrimSpace(meta.CreationProfileRef),
		PolicySpecDigest:   strings.TrimSpace(meta.PolicySpecDigest),
		CreationDigest:     strings.TrimSpace(meta.CreationDigest),
	}
	if identity.Validate() != nil {
		return nil
	}
	return identity
}

func cloneCreationProfile(profile *store.SessionCreationProfile) *store.SessionCreationProfile {
	if profile == nil {
		return nil
	}
	cloned := store.NormalizeSessionCreationProfile(*profile)
	return &cloned
}

func cloneCreationIdentity(identity *store.SessionCreationIdentity) *store.SessionCreationIdentity {
	if identity == nil {
		return nil
	}
	cloned := *identity
	return &cloned
}

func cloneCreationOptions(options *store.SessionCreationOptions) *store.SessionCreationOptions {
	if options == nil {
		return nil
	}
	cloned := *options
	return &cloned
}

func creationError(effect EffectCertainty, code string, err error) error {
	return &CreationError{Effect: effect, Code: strings.TrimSpace(code), Err: err}
}
