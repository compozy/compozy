// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	Google = "google"
	GitHub = "github"

	providerField      = "provider"
	oauthCodeField     = "code"
	oauthStateField    = "state"
	pkceChallengeField = "code_challenge"
	accountActorPrefix = "account:"
	systemActor        = "system"
	deviceIDDetail     = "device_id"

	stateActive     = "active"
	stateSuspended  = "suspended"
	stateDeleting   = "deleting"
	statePending    = "pending"
	stateAuthorized = "authorized"
	stateCancelled  = "cancelled" //nolint:misspell // Published grant state.
	stateRedeemed   = "redeemed"
	stateExpired    = "expired"
)

type Config struct {
	Metrics          *observe.Metrics
	OAuth            *OAuth
	EncryptionKey    []byte
	Turnstile        TurnstileVerifier
	TurnstileSiteKey string
	TrustEdge        bool
	HostedDomain     string
	MonthlyPriceID   string
	AnnualPriceID    string
}

type TurnstileVerifier interface {
	Verify(ctx context.Context, token, clientIP, action string) error
}

type Service struct {
	db     *store.Store
	issuer string
	signer *remoteproto.Signer
	access *remoteproto.JWSVerifier
	proofs *remoteproto.DPoPVerifier
	seal   cipher.AEAD
	oauth  *OAuth
	cfg    Config
	limit  func(string) int
	now    func() time.Time
	codes  codeEntries
}

func New(
	db *store.Store, cfg Config, issuer string, signer *remoteproto.Signer, keys remoteproto.JWKS,
	limit func(string) int, now func() time.Time,
) (*Service, error) {
	origin, err := url.Parse(issuer)
	if err != nil || origin.Hostname() == "" || origin.User != nil || origin.Path != "" ||
		origin.RawQuery != "" || origin.Fragment != "" || (origin.Scheme != "https" && origin.Scheme != "http") {
		return nil, errors.New("invalid cloud auth origin")
	}
	if cfg.OAuth == nil || cfg.Turnstile == nil || cfg.TurnstileSiteKey == "" ||
		!validDomain(cfg.HostedDomain) || len(cfg.EncryptionKey) != 32 {
		return nil, errors.New("cloud auth requires OAuth, Turnstile, a hosted domain and a 32-byte encryption key")
	}
	block, err := aes.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	seal, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	access, err := remoteproto.NewJWSVerifier(issuer, keys, nil)
	if err != nil {
		return nil, err
	}
	cfg.EncryptionKey = nil
	return &Service{
		db: db, issuer: issuer, signer: signer, access: access, proofs: remoteproto.NewDPoPVerifier(),
		seal: seal, oauth: cfg.OAuth, cfg: cfg, limit: limit, now: now,
		codes: codeEntries{windows: make(map[string]codeWindow)},
	}, nil
}

func (s *Service) TurnstileSiteKey() string {
	return s.cfg.TurnstileSiteKey
}

func (s *Service) Sweep(ctx context.Context) error {
	return s.db.SweepAuth(ctx, s.now())
}

func SupportedProvider(provider string) bool {
	return provider == Google || provider == GitHub
}

func Secret() string {
	return base64.RawURLEncoding.EncodeToString(randBytes())
}

func randBytes() []byte {
	var value [32]byte
	rand.Read(value[:])
	return value[:]
}

func hashSecret(secret string) []byte {
	digest := sha256.Sum256([]byte(secret))
	return digest[:]
}

func challenge(verifier string) string {
	return base64.RawURLEncoding.EncodeToString(hashSecret(verifier))
}

func ValidSecret(value string) bool {
	if len(value) != base64.RawURLEncoding.EncodedLen(32) || strings.ContainsAny(value, "\r\n") {
		return false
	}
	_, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil
}

func (s *Service) writeLimit(accountID string) error {
	if retry := s.limit(accountID); retry > 0 {
		return &Failure{Status: http.StatusTooManyRequests, Code: "rate_limited",
			Message: "Too many requests. Try again shortly.", RetryAfter: retry}
	}
	return nil
}

func requireActive(account store.AuthAccount) error {
	switch account.State {
	case stateSuspended:
		return errAccountSuspended
	case stateDeleting:
		return errAccountDeleting
	case stateActive:
		return nil
	default:
		return errSessionInvalid
	}
}

type codeEntries struct {
	mu        sync.Mutex
	windows   map[string]codeWindow
	nextSweep time.Time
}

func (s *Service) ReplaceSigningKeys(keys remoteproto.JWKS) error {
	return s.access.ReplaceKeys(keys)
}
