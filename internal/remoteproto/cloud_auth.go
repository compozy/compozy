package remoteproto

import (
	"context"
	"errors"
	"time"
)

const (
	deviceAccessType = "compozy-device-access+jwt"
	DeviceAccessTTL  = 15 * time.Minute
)

type DeviceFlowRequest struct {
	Provider       string `json:"provider,omitempty"`
	ClientName     string `json:"client_name"`
	ClientPlatform string `json:"client_platform"`
	InstallationID string `json:"installation_id"`
}

type DeviceFlowResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int32  `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}

type DeviceFlowTokenRequest struct {
	DeviceCode string `json:"device_code"`
}

type DeviceFlowTokenResponse struct {
	EnrollmentToken string `json:"enrollment_token"`
	AccountID       string `json:"account_id"`
	Email           string `json:"email"`
	NewAccount      bool   `json:"new_account"`
}

type DeviceEnrollRequest struct {
	PublicJWK      JWK    `json:"public_jwk"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	InstallationID string `json:"installation_id"`
}

type DeviceSessionTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type DeviceEnrollResponse struct {
	DeviceID DeviceID `json:"device_id"`
	Admin    bool     `json:"admin"`
	DeviceSessionTokens
}

type DeviceAccessClaims struct {
	Issuer    string       `json:"iss"`
	AccountID string       `json:"account_id"`
	SessionID string       `json:"sid"`
	DeviceID  DeviceID     `json:"sub"`
	Cnf       Confirmation `json:"cnf"`
	JTI       string       `json:"jti"`
	IssuedAt  int64        `json:"iat"`
	ExpiresAt int64        `json:"exp"`
}

var ErrDeviceAccessInvalid = errors.New("invalid device access credential")

func (s *Signer) SignDeviceAccess(claims DeviceAccessClaims) (string, error) {
	return s.sign(deviceAccessType, claims)
}

func (v *JWSVerifier) VerifyDeviceAccess(ctx context.Context, token string, now time.Time) (DeviceAccessClaims, error) {
	var claims DeviceAccessClaims
	if err := v.verify(ctx, token, deviceAccessType, &claims); err != nil {
		return DeviceAccessClaims{}, ErrDeviceAccessInvalid
	}
	if _, err := decodeCoordinate(claims.Cnf.JKT); err != nil {
		return DeviceAccessClaims{}, ErrDeviceAccessInvalid
	}
	if claims.Issuer != v.issuer || claims.AccountID == "" || claims.SessionID == "" || claims.DeviceID == "" ||
		claims.JTI == "" ||
		claims.IssuedAt <= 0 || claims.IssuedAt > now.Add(DPoPLeeway).Unix() ||
		claims.ExpiresAt <= max(claims.IssuedAt, now.Unix()) ||
		claims.ExpiresAt-claims.IssuedAt > int64(DeviceAccessTTL/time.Second) {
		return DeviceAccessClaims{}, ErrDeviceAccessInvalid
	}
	return claims, nil
}
