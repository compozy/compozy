package remoteproto

import (
	"context"
	"errors"
	"slices"
	"time"
)

var (
	ErrGrantSignatureInvalid = errors.New("grant_signature_invalid")
	ErrGrantBindingMissing   = errors.New("grant_binding_missing")
	ErrGrantExpired          = errors.New("grant_expired")
	ErrGrantInvalid          = errors.New("grant_invalid")
	ErrLeaseInvalid          = errors.New("lease_invalid")
	ErrLeaseExpired          = errors.New("lease_expired")
	ErrSnapshotInvalid       = errors.New("security_snapshot_invalid")
	ErrGatePolicyInvalid     = errors.New("gate_policy_invalid")
)

func (s *Signer) SignGrant(claims GrantClaims) (string, error) {
	return s.sign(grantType, claims)
}

func (v *JWSVerifier) VerifyGrant(ctx context.Context, token string, now time.Time) (GrantClaims, error) {
	var claims GrantClaims
	if err := v.verify(ctx, token, grantType, &claims); err != nil {
		return GrantClaims{}, ErrGrantSignatureInvalid
	}
	if claims.Cnf.JKT == "" {
		return GrantClaims{}, ErrGrantBindingMissing
	}
	if now.After(time.Unix(claims.ExpiresAt, 0).Add(GrantLeeway)) {
		return GrantClaims{}, ErrGrantExpired
	}
	if claims.Issuer != v.issuer || !validGrant(claims, now) {
		return GrantClaims{}, ErrGrantInvalid
	}
	return claims, nil
}

func validGrant(claims GrantClaims, now time.Time) bool {
	if _, err := decodeCoordinate(claims.Cnf.JKT); err != nil {
		return false
	}
	issuedAt := time.Unix(claims.IssuedAt, 0)
	expiresAt := time.Unix(claims.ExpiresAt, 0)
	return claims.Audience != "" && claims.Subject != "" && claims.AccountID != "" && claims.JTI != "" &&
		claims.SecurityEpoch > 0 && claims.Route.Grantable() && (!claims.Browser || claims.Route == RouteCloud) &&
		validScopes(claims.Scopes) && claims.IssuedAt > 0 && !issuedAt.After(now.Add(GrantLeeway)) &&
		expiresAt.After(issuedAt) && !expiresAt.After(issuedAt.Add(GrantTTL))
}

func validScopes(scopes []Scope) bool {
	return !slices.ContainsFunc(scopes, func(scope Scope) bool { return !scope.Valid() })
}

func (s *Signer) SignLease(lease EntitlementLease) (string, error) {
	return s.sign(leaseType, lease)
}

func (v *JWSVerifier) VerifyLease(ctx context.Context, token string, now time.Time) (EntitlementLease, error) {
	var claims EntitlementLease
	if err := v.verify(ctx, token, leaseType, &claims); err != nil {
		return EntitlementLease{}, ErrLeaseInvalid
	}
	if err := validateLease(claims, now); err != nil {
		return EntitlementLease{}, err
	}
	return claims, nil
}

func validateLease(lease EntitlementLease, now time.Time) error {
	if lease.ComputerID == "" || lease.AccountID == "" || lease.Sequence == 0 || lease.SecurityEpoch == 0 ||
		(lease.Plan != "free" && lease.Plan != "pro") {
		return ErrLeaseInvalid
	}
	for _, route := range lease.AllowedRoutes {
		if !route.Grantable() || (route == RouteCloud && lease.Plan != "pro") || lease.PlanLimited {
			return ErrLeaseInvalid
		}
	}
	if lease.ValidUntil <= now.Unix() {
		return ErrLeaseExpired
	}
	if lease.ValidUntil > now.Add(LeaseTTL+GrantLeeway).Unix() {
		return ErrLeaseInvalid
	}
	return nil
}

func (s *Signer) SignSnapshot(snapshot SecuritySnapshot) (string, error) {
	return s.sign(snapshotType, snapshot)
}

func (v *JWSVerifier) VerifySnapshot(ctx context.Context, token string) (SecuritySnapshot, error) {
	var snapshot SecuritySnapshot
	if err := v.verify(ctx, token, snapshotType, &snapshot); err != nil || !validSnapshot(snapshot) {
		return SecuritySnapshot{}, ErrSnapshotInvalid
	}
	return snapshot, nil
}

type GateMode string

const (
	GateEnforce GateMode = "enforce"
	GateGrace   GateMode = "grace"
)

type GatePolicy struct {
	Mode     GateMode   `json:"mode"`
	Until    *time.Time `json:"until"`
	Message  string     `json:"message"`
	IssuedAt int64      `json:"iat"`
}

func (p GatePolicy) GraceAt(now time.Time) bool {
	return p.Mode == GateGrace && p.Until != nil && now.Before(*p.Until)
}

func (s *Signer) SignGatePolicy(policy GatePolicy) (string, error) {
	return s.sign(gatePolicyType, policy)
}

func (v *JWSVerifier) VerifyGatePolicy(ctx context.Context, token string, now time.Time) (GatePolicy, error) {
	var policy GatePolicy
	if err := v.verify(ctx, token, gatePolicyType, &policy); err != nil {
		return GatePolicy{}, ErrGatePolicyInvalid
	}
	validMode := (policy.Mode == GateEnforce && policy.Until == nil) ||
		(policy.Mode == GateGrace && policy.Until != nil)
	if !validMode || policy.IssuedAt <= 0 || policy.IssuedAt > now.Add(GrantLeeway).Unix() {
		return GatePolicy{}, ErrGatePolicyInvalid
	}
	return policy, nil
}
