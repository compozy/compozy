package gateway

import (
	"context"
	"errors"
	"time"
)

const (
	ActorKindAppToken         ActorKind = "app_token"
	AppTokenLimit                       = 64
	AppTokenOriginLimit                 = 16
	AppTokenNameLimit                   = 64
	AppTokenAuthFailureLimit            = 10
	AppTokenAuthFailureWindow           = time.Minute
	// #nosec G101 -- this is a public protocol prefix, not credential material.
	appTokenPrefix = "cpz_app_"
)

var (
	errAppTokenStoreRequired   = errors.New("gateway: app token store is required")
	ErrAppTokenNameInvalid     = errors.New("name must be 1-64 characters")
	ErrAppTokenOriginInvalid   = errors.New("origin must use scheme://host[:port] with no path")
	ErrAppTokenOriginLimit     = errors.New("a token allows at most 16 origins")
	ErrAppTokenLimit           = errors.New("revoke an app token before creating another (limit 64)")
	ErrAppTokenNotFound        = errors.New("app token not found")
	ErrAppTokenRevoked         = errors.New("app token was revoked")
	ErrAppTokenStateInvalid    = errors.New("state must be active, revoked, or all")
	ErrAppTokenUnauthenticated = errors.New("app token is not valid")
	ErrAppTokenRequired        = errors.New("this origin needs an app token in the Authorization header")
	ErrAppTokenRateLimited     = errors.New("too many failed attempts; retry after 60s")
	ErrAppOriginNotAllowed     = errors.New("origin is not allowed; add it with compozy app-token update")
	ErrAppTokenRouteForbidden  = errors.New("this route is only available to local processes and the CompozyOS web app")
	ErrAppTokenLocalOnly       = errors.New("app tokens work on the local listener only")
)

type AppTokenState string

const (
	AppTokenStateActive  AppTokenState = "active"
	AppTokenStateRevoked AppTokenState = "revoked"
	AppTokenStateAll     AppTokenState = "all"
)

type AppToken struct {
	Session DeviceSession
	Origins []string
}

type AppTokenIssued struct {
	Token  AppToken
	Secret string
}

type AppTokenPatch struct {
	Name    *string
	Origins *[]string
}

type AppTokenRevokeResult struct {
	Token    AppToken
	Changed  bool
	Canceled int
}

type StoredAppToken struct {
	Token     AppToken
	TokenHash string
}

type AppTokenManager interface {
	CreateAppToken(context.Context, string, []string) (AppTokenIssued, error)
	ListAppTokens(context.Context, AppTokenState) ([]AppToken, error)
	UpdateAppToken(context.Context, string, AppTokenPatch) (AppToken, error)
	RevokeAppToken(context.Context, string) (AppTokenRevokeResult, error)
}

type AppTokenAuthenticator interface {
	AuthenticateAppToken(context.Context, string) (AppToken, error)
	TouchAppTokenLastSeen(context.Context, string, uint64) error
	AppTokenOriginAllowed(string) bool
	AcquireMutation(context.Context, string, uint64) (context.Context, error)
	RevalidateForCommit(context.Context, string, uint64) error
	Connections() ConnectionRegistry
}

// AppTokenStore returns the active origin union from the same transaction as each mutation.
type AppTokenStore interface {
	CreateAppToken(context.Context, StoredAppToken) ([]string, error)
	ListAppTokens(context.Context, AppTokenState) ([]AppToken, error)
	UpdateAppToken(context.Context, string, AppTokenPatch, time.Time) (AppToken, []string, error)
	RevokeAppToken(context.Context, string, time.Time) (AppToken, bool, []string, error)
	FindAppTokenByTokenHash(context.Context, string) (StoredAppToken, error)
	TouchAppTokenLastSeen(context.Context, string, uint64, time.Time) error
	ListActiveAppTokenOrigins(context.Context) ([]string, error)
}
