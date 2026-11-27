package apptoken

import (
	"context"
	"errors"
	"time"
)

const (
	ActorKind         = "app_token"
	Limit             = 64
	OriginLimit       = 16
	nameLimit         = 64
	AuthFailureLimit  = 10
	AuthFailureWindow = time.Minute
	// #nosec G101 -- this is a public protocol prefix, not credential material.
	appTokenPrefix = "cpz_app_"
)

var (
	errAppTokenStoreRequired = errors.New("app token: app token store is required")
	ErrNameInvalid           = errors.New("name must be 1-64 characters")
	ErrOriginInvalid         = errors.New("origin must use scheme://host[:port] with no path")
	ErrOriginLimit           = errors.New("a token allows at most 16 origins")
	ErrLimit                 = errors.New("revoke an app token before creating another (limit 64)")
	ErrNotFound              = errors.New("app token not found")
	ErrRevoked               = errors.New("app token was revoked")
	ErrStateInvalid          = errors.New("state must be active, revoked, or all")
	ErrUnauthenticated       = errors.New("app token is not valid")
	ErrRequired              = errors.New("this origin needs an app token in the Authorization header")
	ErrRateLimited           = errors.New("too many failed attempts; retry after 60s")
	ErrOriginNotAllowed      = errors.New("origin is not allowed; add it with compozy app-token update")
	ErrRouteForbidden        = errors.New("this route is only available to local processes and the CompozyOS web app")
	ErrLocalOnly             = errors.New("app tokens work on the local listener only")
)

type State string

const (
	StateActive  State = "active"
	StateRevoked State = "revoked"
	StateAll     State = "all"
)

type Token struct {
	ID          string
	Name        string
	ActorKind   string
	RevokeEpoch uint64
	CreatedAt   time.Time
	LastSeenAt  time.Time
	RevokedAt   time.Time
	Origins     []string
}

type Issued struct {
	Token  Token
	Secret string
}

type Patch struct {
	Name    *string
	Origins *[]string
}

type RevokeResult struct {
	Token    Token
	Changed  bool
	Canceled int
}

type StoredToken struct {
	Token     Token
	TokenHash string
}

// Store returns the active origin union from the same transaction as each mutation.
type Store interface {
	CreateAppToken(context.Context, StoredToken) ([]string, error)
	ListAppTokens(context.Context, State) ([]Token, error)
	UpdateAppToken(context.Context, string, Patch, time.Time) (Token, []string, error)
	RevokeAppToken(context.Context, string, time.Time) (Token, bool, []string, error)
	FindAppTokenByTokenHash(context.Context, string) (StoredToken, error)
	TouchAppTokenLastSeen(context.Context, string, uint64, time.Time) error
	ListActiveAppTokenOrigins(context.Context) ([]string, error)
}
