package remotehost

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const snapshotWait = 5 * time.Second

var ErrMissing = errors.New("remote host record not found")

type SessionOptions struct {
	Now             func() time.Time
	RequireApproval bool
	MaxScopes       []remoteproto.Scope
	Report          func(remoteproto.ControlType, uint64, any)
}

// Sessions serializes host writes and publishes authorization separately from SQLite writer admission.
type Sessions struct {
	store    Store
	identity Identity
	verifier remoteproto.Verifier
	proofs   *remoteproto.DPoPVerifier
	now      func() time.Time
	report   func(remoteproto.ControlType, uint64, any)

	serial             sync.Mutex
	security           *remoteproto.SecurityState
	leaseSequence      uint64
	snapshotBlocked    bool
	snapshotChanged    chan struct{}
	snapshotRequested  uint64
	policy             sessionPolicy
	locallyRevoked     map[remoteproto.DeviceID]bool
	retiredSigningKeys map[string]bool

	mu        sync.RWMutex
	admission admissionState
	live      map[*liveRequest]struct{}
}

type sessionPolicy struct {
	requireApproval bool
	maxScopes       []remoteproto.Scope
}

type admissionState struct {
	snapshot remoteproto.SecuritySnapshot
	lease    remoteproto.EntitlementLease
	policy   sessionPolicy
	blocked  bool
	enabled  bool
}

var _ SessionStore = (*Sessions)(nil)

func NewSessions(
	ctx context.Context,
	store Store,
	identity Identity,
	verifier remoteproto.Verifier,
	opts SessionOptions,
) (*Sessions, error) {
	sessions := &Sessions{
		store: store, identity: identity, verifier: verifier, proofs: remoteproto.NewDPoPVerifier(),
		now: opts.Now, report: opts.Report, snapshotChanged: make(chan struct{}),
		security:       remoteproto.NewSecurityState(identity.ComputerID, identity.AccountID),
		policy:         sessionPolicy{opts.RequireApproval, slices.Clone(opts.MaxScopes)},
		locallyRevoked: make(map[remoteproto.DeviceID]bool),
		live:           make(map[*liveRequest]struct{}),
	}
	if sessions.now == nil {
		sessions.now = time.Now
	}
	sequence, err := store.RemoteLeaseSequence(ctx)
	if err != nil {
		return nil, err
	}
	sessions.leaseSequence = sequence
	snapshot, err := store.RemoteSnapshot(ctx, identity)
	switch {
	case err == nil:
		if _, err := sessions.security.ApplySnapshot(
			snapshot,
			func(remoteproto.SecuritySnapshot) error { return nil },
		); err != nil {
			return nil, err
		}
	case !errors.Is(err, ErrMissing):
		return nil, err
	}
	sessions.admission.enabled = true
	sessions.publishLocked()
	return sessions, nil
}

func (s *Sessions) Mint(ctx context.Context, in MintInput) (MintResult, error) {
	now := s.now()
	grant, err := s.verifyMintGrant(ctx, in.Grant, now)
	if err != nil {
		return MintResult{}, err
	}
	_, err = s.proofs.VerifyDPoP(in.Proof.Token, remoteproto.DPoPBinding{
		Method: in.Proof.Method, URL: in.Proof.URL, JKT: grant.Cnf.JKT,
	}, now)
	if err != nil {
		return MintResult{}, err
	}
	if in.Proof.Listener.Route() != grant.Route || in.Proof.Browser != grant.Browser {
		return MintResult{}, ErrRouteMismatch
	}
	return s.mintWhenReady(ctx, grant)
}

func (s *Sessions) verifyMintGrant(ctx context.Context, token string, now time.Time) (remoteproto.GrantClaims, error) {
	grant, err := s.verifier.VerifyGrant(ctx, token, now)
	if err != nil {
		return remoteproto.GrantClaims{}, err
	}
	if grant.Audience != s.identity.ComputerID || grant.AccountID != s.identity.AccountID {
		return remoteproto.GrantClaims{}, remoteproto.ErrGrantInvalid
	}
	used, err := s.store.RemoteGrantUsed(ctx, grant.JTI)
	if err != nil {
		return remoteproto.GrantClaims{}, err
	}
	if used {
		return remoteproto.GrantClaims{}, ErrGrantReplay
	}
	return grant, nil
}

func (s *Sessions) mintWhenReady(ctx context.Context, grant remoteproto.GrantClaims) (MintResult, error) {
	wait, cancel := context.WithTimeout(ctx, snapshotWait)
	defer cancel()
	for {
		s.serial.Lock()
		s.mu.RLock()
		state := s.admission
		s.mu.RUnlock()
		if s.retiredSigningKeys[grant.SigningKeyID] {
			s.serial.Unlock()
			return MintResult{}, remoteproto.ErrGrantSignatureInvalid
		}
		if !state.enabled {
			s.serial.Unlock()
			return MintResult{}, ErrNotLinked
		}
		if state.snapshot.SecurityEpoch > grant.SecurityEpoch {
			s.serial.Unlock()
			return MintResult{}, ErrSnapshotPending
		}
		if !state.blocked && s.security.ReadyForMint(grant.SecurityEpoch) {
			result, err := s.commitMint(ctx, grant, state, s.now())
			s.serial.Unlock()
			return result, err
		}
		request := state.snapshot.SecurityEpoch < grant.SecurityEpoch && s.snapshotRequested < grant.SecurityEpoch
		if request {
			s.snapshotRequested = grant.SecurityEpoch
		}
		updated := s.snapshotChanged
		s.serial.Unlock()
		if request {
			s.emit(
				remoteproto.ControlSnapshotRequest,
				remoteproto.SnapshotRequest{AppliedEpoch: state.snapshot.SecurityEpoch},
			)
		}
		select {
		case <-updated:
		case <-wait.Done():
			s.serial.Lock()
			if s.snapshotRequested == grant.SecurityEpoch {
				s.snapshotRequested = 0
			}
			s.serial.Unlock()
			return MintResult{}, ErrSnapshotPending
		}
	}
}

func (s *Sessions) commitMint(
	ctx context.Context, grant remoteproto.GrantClaims, state admissionState, now time.Time,
) (MintResult, error) {
	if err := state.entitled(grant.Route, now); err != nil {
		return MintResult{}, err
	}
	if now.After(time.Unix(grant.ExpiresAt, 0).Add(remoteproto.GrantLeeway)) {
		return MintResult{}, remoteproto.ErrGrantExpired
	}
	device, ok := snapshotDevice(state.snapshot, grant.Subject, grant.Cnf.JKT)
	if !ok || s.locallyRevoked[grant.Subject] {
		return MintResult{}, ErrDeviceRevoked
	}
	credential, err := GenerateOpaqueSecret(rand.Reader, "rc_")
	if err != nil {
		return MintResult{}, err
	}
	session := Session{
		ID:             "rs_" + rand.Text(),
		CredentialHash: sha256.Sum256([]byte(credential)),
		DeviceID:       grant.Subject,
		JKT:            grant.Cnf.JKT,
		SigningKeyID:   grant.SigningKeyID,
		Scopes:         effectiveScopes(grant.Scopes, device, state.policy.maxScopes),
		Route:          grant.Route,
		Browser:        grant.Browser,
		CreatedAt:      now,
		ExpiresAt:      now.Add(SessionTTL),
	}
	var approval Approval
	var refusal error
	if err := s.store.WriteRemote(ctx, func(ctx context.Context, tx Tx) error {
		used, err := tx.UseGrant(ctx, grant.JTI, time.Unix(grant.ExpiresAt, 0).Add(remoteproto.DPoPReplayWindow))
		if err != nil {
			return err
		}
		if !used {
			return ErrGrantReplay
		}
		if state.policy.requireApproval {
			approval, refusal, err = admitApproval(ctx, tx, session, now)
			if refusal != nil || err != nil {
				return err
			}
		}
		return tx.InsertSession(ctx, session)
	}); err != nil {
		return MintResult{}, err
	}
	if refusal != nil {
		if approval.State == approvalPending {
			s.reportPending(approval)
		}
		return MintResult{}, refusal
	}
	s.renewTunnelRequests(ctx, session, now)
	s.emit(remoteproto.ControlSessionMinted, remoteproto.SessionMinted{
		DeviceID: session.DeviceID, JKT: session.JKT, At: now.Unix(),
	})
	return MintResult{
		SessionID: session.ID, Credential: credential, ExpiresAt: session.ExpiresAt.Unix(), Scopes: session.Scopes,
	}, nil
}

func (s *Sessions) Authenticate(
	ctx context.Context,
	credential string,
	proof DPoPProof,
	now time.Time,
) (Principal, error) {
	return s.authenticate(ctx, credential, proof, now, false)
}

func (s *Sessions) authenticate(
	ctx context.Context, credential string, proof DPoPProof, now time.Time, ticketVerified bool,
) (Principal, error) {
	if !ValidOpaqueSecret(credential, "rc_") {
		return Principal{}, ErrSessionInvalid
	}
	s.serial.Lock()
	defer s.serial.Unlock()
	session, err := s.store.RemoteSession(ctx, sha256.Sum256([]byte(credential)))
	if errors.Is(err, ErrMissing) {
		return Principal{}, ErrSessionInvalid
	}
	if err != nil {
		return Principal{}, err
	}
	if proof.Listener.Route() != session.Route || proof.Browser != session.Browser {
		return Principal{}, ErrRouteMismatch
	}
	if !ticketVerified {
		binding := remoteproto.DPoPBinding{Method: proof.Method, URL: proof.URL, JKT: session.JKT}
		if !proof.Browser {
			binding.Credential = credential
		}
		if _, err := s.proofs.VerifyDPoP(proof.Token, binding, now); err != nil {
			return Principal{}, err
		}
	}
	if err := s.checkDevice(ctx, session); err != nil {
		return Principal{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.admission.check(session, now); err != nil {
		return Principal{}, err
	}
	scopes := make(map[remoteproto.Scope]bool, len(session.Scopes))
	for _, scope := range session.Scopes {
		scopes[scope] = true
	}
	return Principal{DeviceID: session.DeviceID, SessionID: session.ID, Scopes: scopes, Route: session.Route,
		Browser: session.Browser, session: session, epoch: s.admission.snapshot.SecurityEpoch, proof: proof.Token}, nil
}

func (s *Sessions) emit(kind remoteproto.ControlType, payload any) {
	if s.report == nil {
		return
	}
	s.mu.RLock()
	epoch := s.admission.snapshot.SecurityEpoch
	s.mu.RUnlock()
	s.report(kind, epoch, payload)
}

func (s admissionState) entitled(route remoteproto.RouteKind, now time.Time) error {
	if !s.enabled || !s.snapshot.Linked || s.lease.Sequence == 0 || now.Unix() >= s.lease.ValidUntil {
		return ErrEntitlement
	}
	if !s.lease.Allows(route, now) {
		return ErrPlanRequired
	}
	return nil
}

func (s admissionState) check(session Session, now time.Time) error {
	if s.snapshot.Linked && !s.allows(session) {
		return ErrDeviceRevoked
	}
	if !now.Before(session.ExpiresAt) {
		return ErrSessionExpired
	}
	if s.blocked {
		return ErrSnapshotPending
	}
	if err := s.entitled(session.Route, now); err != nil {
		return err
	}
	if session.Revoked {
		return ErrDeviceRevoked
	}
	return nil
}

func (s admissionState) allows(session Session) bool {
	return s.snapshot.Allows(session.DeviceID, session.JKT, session.Scopes) &&
		(len(s.policy.maxScopes) == 0 || !slices.ContainsFunc(session.Scopes, func(scope remoteproto.Scope) bool {
			return !slices.Contains(s.policy.maxScopes, scope)
		}))
}

func snapshotDevice(
	snapshot remoteproto.SecuritySnapshot,
	id remoteproto.DeviceID,
	jkt string,
) (remoteproto.SnapshotDevice, bool) {
	for _, device := range snapshot.Devices {
		if device.DeviceID == id && slices.Contains(device.JKTs, jkt) {
			return device, true
		}
	}
	return remoteproto.SnapshotDevice{}, false
}

func effectiveScopes(
	grant []remoteproto.Scope,
	device remoteproto.SnapshotDevice,
	maximum []remoteproto.Scope,
) []remoteproto.Scope {
	effective := make(map[remoteproto.Scope]bool, len(grant))
	for _, scope := range grant {
		if (scope != remoteproto.ScopeAdmin || device.Admin) && slices.Contains(device.Scopes, scope) &&
			(len(maximum) == 0 || slices.Contains(maximum, scope)) {
			effective[scope] = true
		}
	}
	return slices.Sorted(maps.Keys(effective))
}

func pendingApprovalError(id string) error {
	return &Failure{http.StatusForbidden, "device_pending_approval", "Waiting for approval on this computer.",
		map[string]string{"approval_id": id}}
}
