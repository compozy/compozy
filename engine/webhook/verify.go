package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	StrategyNone   = "none"
	StrategyHMAC   = "hmac"
	StrategyStripe = "stripe"
	StrategyGitHub = "github"
)

const (
	headerStripeSignature = "Stripe-Signature"
	headerGitHubSignature = "X-Hub-Signature-256"
	prefixEnv             = "env://"
	prefixGitHub          = "sha256="
)

// Verifier validates an incoming webhook request using the given raw body.
type Verifier interface {
	Verify(ctx context.Context, r *http.Request, body []byte) error
}

// VerifyConfig defines verification strategy and options.
type VerifyConfig struct {
	Strategy string
	Secret   string
	Header   string
	Skew     time.Duration
}

// NewVerifier creates a Verifier based on the provided configuration.
const defaultStripeSkew = 5 * time.Minute

func NewVerifier(cfg VerifyConfig) (Verifier, error) {
	switch cfg.Strategy {
	case StrategyNone:
		return noneVerifier{}, nil
	case StrategyHMAC:
		sec, err := resolveSecret(cfg.Secret)
		if err != nil {
			return nil, err
		}
		if cfg.Header == "" {
			return nil, errors.New("missing signature header name for hmac strategy")
		}
		return hmacVerifier{secret: sec, header: cfg.Header}, nil
	case StrategyStripe:
		sec, err := resolveSecret(cfg.Secret)
		if err != nil {
			return nil, err
		}
		allowedSkew := cfg.Skew
		if allowedSkew == 0 {
			allowedSkew = defaultStripeSkew
		}
		return stripeVerifier{secret: sec, skew: allowedSkew, now: time.Now}, nil
	case StrategyGitHub:
		sec, err := resolveSecret(cfg.Secret)
		if err != nil {
			return nil, err
		}
		return githubVerifier{secret: sec}, nil
	default:
		return nil, errors.New("unknown verification strategy")
	}
}

func resolveSecret(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty secret")
	}
	if after, ok := strings.CutPrefix(s, prefixEnv); ok {
		key := after
		val := os.Getenv(key)
		if val == "" {
			return nil, fmt.Errorf("secret env %q not set", key)
		}
		return []byte(val), nil
	}
	return []byte(s), nil
}

type noneVerifier struct{}

func (noneVerifier) Verify(_ context.Context, _ *http.Request, _ []byte) error {
	return nil
}

type hmacVerifier struct {
	secret []byte
	header string
}

func (v hmacVerifier) Verify(_ context.Context, r *http.Request, body []byte) error {
	sig := r.Header.Get(v.header)
	if sig == "" {
		return errors.New("missing signature header")
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(strings.TrimSpace(sig))
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	if !hmac.Equal(expected, got) {
		return errors.New("signature mismatch")
	}
	return nil
}

type stripeVerifier struct {
	secret []byte
	skew   time.Duration
	now    func() time.Time
}

func (v stripeVerifier) Verify(_ context.Context, r *http.Request, body []byte) error {
	header := r.Header.Get(headerStripeSignature)
	if header == "" {
		return errors.New("missing Stripe-Signature")
	}
	tsStr, candidates, err := parseStripeSignatureHeader(header)
	if err != nil {
		return err
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return errors.New("invalid Stripe timestamp")
	}
	currentTime := v.now()
	tstamp := time.Unix(ts, 0)
	if currentTime.Sub(tstamp) > v.skew || tstamp.Sub(currentTime) > v.skew {
		return errors.New("timestamp skew too large")
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write([]byte(tsStr))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	for _, c := range candidates {
		got, err := hex.DecodeString(c)
		if err != nil {
			return fmt.Errorf("invalid Stripe signature: %w", err)
		}
		if hmac.Equal(expected, got) {
			return nil
		}
	}
	return errors.New("signature mismatch")
}

func parseStripeSignatureHeader(header string) (string, []string, error) {
	var tsStr string
	var v1Values []string
	parts := strings.Split(header, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "t" {
			tsStr = kv[1]
		}
		if kv[0] == "v1" {
			v1Values = append(v1Values, kv[1])
		}
	}
	if tsStr == "" || len(v1Values) == 0 {
		return "", nil, errors.New("invalid Stripe-Signature format")
	}
	var candidates []string
	for _, v1 := range v1Values {
		if strings.Contains(v1, ":") {
			candidates = append(candidates, strings.Split(v1, ":")...)
		} else {
			candidates = append(candidates, v1)
		}
	}
	for i := range candidates {
		candidates[i] = strings.TrimSpace(candidates[i])
	}
	return tsStr, candidates, nil
}

type githubVerifier struct {
	secret []byte
}

func (v githubVerifier) Verify(_ context.Context, r *http.Request, body []byte) error {
	sig := r.Header.Get(headerGitHubSignature)
	if !strings.HasPrefix(sig, prefixGitHub) {
		return errors.New("invalid GitHub signature header")
	}
	hexsig := strings.TrimSpace(sig[len(prefixGitHub):])
	if hexsig == "" {
		return errors.New("missing GitHub signature")
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(hexsig)
	if err != nil {
		return fmt.Errorf("invalid GitHub signature encoding: %w", err)
	}
	if !hmac.Equal(expected, got) {
		return errors.New("signature mismatch")
	}
	return nil
}
