package verify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoneVerifier_ShouldAcceptAll(t *testing.T) {
	v, err := New(Config{Strategy: "none"})
	require.NoError(t, err)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
	err = v.Verify(req.Context(), req, []byte("body"))
	require.NoError(t, err)
}

func TestHMACVerifier_ShouldValidateSignature(t *testing.T) {
	t.Run("Should verify valid signature", func(t *testing.T) {
		body := []byte("hello world")
		secret := "topsecret"
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		v, err := New(Config{Strategy: "hmac", Secret: secret, Header: "X-Sig"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Sig", sig)
		err = v.Verify(req.Context(), req, body)
		require.NoError(t, err)
	})
	t.Run("Should fail on missing header", func(t *testing.T) {
		v, err := New(Config{Strategy: "hmac", Secret: "s", Header: "X-Sig"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		err = v.Verify(req.Context(), req, []byte("abc"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing signature header")
	})
	t.Run("Should fail on invalid hex", func(t *testing.T) {
		v, err := New(Config{Strategy: "hmac", Secret: "s", Header: "X-Sig"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Sig", "not-hex")
		err = v.Verify(req.Context(), req, []byte("abc"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signature encoding")
	})
	t.Run("Should fail on signature mismatch", func(t *testing.T) {
		body := []byte("hello world")
		mac := hmac.New(sha256.New, []byte("wrongsecret"))
		_, _ = mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		v, err := New(Config{Strategy: "hmac", Secret: "topsecret", Header: "X-Sig"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Sig", sig)
		err = v.Verify(req.Context(), req, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature mismatch")
	})
	t.Run("Should resolve secret from env", func(t *testing.T) {
		os.Setenv("HMAC_SECRET", "abc")
		defer os.Unsetenv("HMAC_SECRET")
		v, err := New(Config{Strategy: "hmac", Secret: "env://HMAC_SECRET", Header: "X-Sig"})
		require.NoError(t, err)
		mac := hmac.New(sha256.New, []byte("abc"))
		_, _ = mac.Write([]byte("x"))
		sig := hex.EncodeToString(mac.Sum(nil))
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Sig", sig)
		err = v.Verify(req.Context(), req, []byte("x"))
		require.NoError(t, err)
	})
}

func TestStripeVerifier_ShouldValidateHeaderAndTimestamp(t *testing.T) {
	t.Run("Should verify valid stripe signature", func(t *testing.T) {
		body := []byte("{\"id\":\"evt_1\"}")
		ts := time.Now().Unix()
		secret := "whsec_123"
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10)))
		_, _ = mac.Write([]byte("."))
		_, _ = mac.Write(body)
		v1 := hex.EncodeToString(mac.Sum(nil))
		header := "t=" + strconv.FormatInt(ts, 10) + ", v1=" + v1
		v, err := New(Config{Strategy: "stripe", Secret: secret})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("Stripe-Signature", header)
		err = v.Verify(req.Context(), req, body)
		require.NoError(t, err)
	})
	t.Run("Should fail on skew too large", func(t *testing.T) {
		body := []byte("{}")
		ts := time.Now().Add(-10 * time.Minute).Unix()
		secret := "whsec_123"
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10)))
		_, _ = mac.Write([]byte("."))
		_, _ = mac.Write(body)
		v1 := hex.EncodeToString(mac.Sum(nil))
		header := "t=" + strconv.FormatInt(ts, 10) + ", v1=" + v1
		v, err := New(Config{Strategy: "stripe", Secret: secret})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("Stripe-Signature", header)
		err = v.Verify(req.Context(), req, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp skew too large")
	})
	t.Run("Should fail on missing parts", func(t *testing.T) {
		v, err := New(Config{Strategy: "stripe", Secret: "s"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("Stripe-Signature", "t=123")
		err = v.Verify(req.Context(), req, []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Stripe-Signature format")
	})
	t.Run("Should accept when any v1 matches among multiples", func(t *testing.T) {
		body := []byte("{\"id\":\"evt_1\"}")
		ts := time.Now().Unix()
		secret := "whsec_123"
		macGood := hmac.New(sha256.New, []byte(secret))
		_, _ = macGood.Write([]byte(strconv.FormatInt(ts, 10)))
		_, _ = macGood.Write([]byte("."))
		_, _ = macGood.Write(body)
		good := hex.EncodeToString(macGood.Sum(nil))
		bad := "deadbeef"
		header := "t=" + strconv.FormatInt(ts, 10) + ", v1=" + bad + ", v1=" + good
		v, err := New(Config{Strategy: "stripe", Secret: secret})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("Stripe-Signature", header)
		err = v.Verify(req.Context(), req, body)
		require.NoError(t, err)
	})
	t.Run("Should fail on signature mismatch", func(t *testing.T) {
		body := []byte("{}")
		ts := time.Now().Unix()
		secret := "whsec_123"
		wrong := "aaaaaaaa"
		header := "t=" + strconv.FormatInt(ts, 10) + ", v1=" + wrong
		v, err := New(Config{Strategy: "stripe", Secret: secret})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("Stripe-Signature", header)
		err = v.Verify(req.Context(), req, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature mismatch")
	})
}

func TestGitHubVerifier_ShouldValidateHeader(t *testing.T) {
	t.Run("Should verify valid signature", func(t *testing.T) {
		body := []byte("{\"a\":1}")
		secret := "ghs_abc"
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		v, err := New(Config{Strategy: "github", Secret: secret})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
		err = v.Verify(req.Context(), req, body)
		require.NoError(t, err)
	})
	t.Run("Should fail on malformed header", func(t *testing.T) {
		v, err := New(Config{Strategy: "github", Secret: "s"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Hub-Signature-256", "badprefix=")
		err = v.Verify(req.Context(), req, []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GitHub signature header")
	})
	t.Run("Should fail on empty signature value", func(t *testing.T) {
		v, err := New(Config{Strategy: "github", Secret: "s"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Hub-Signature-256", "sha256=")
		err = v.Verify(req.Context(), req, []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing GitHub signature")
	})
	t.Run("Should fail on invalid hex", func(t *testing.T) {
		v, err := New(Config{Strategy: "github", Secret: "s"})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Hub-Signature-256", "sha256=nothex")
		err = v.Verify(req.Context(), req, []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GitHub signature encoding")
	})
	t.Run("Should fail on signature mismatch", func(t *testing.T) {
		body := []byte("{\"a\":1}")
		secret := "ghs_abc"
		mac := hmac.New(sha256.New, []byte("wrong"))
		_, _ = mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		v, err := New(Config{Strategy: "github", Secret: secret})
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
		err = v.Verify(req.Context(), req, body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature mismatch")
	})
}

func TestFactory_ErrorPaths(t *testing.T) {
	t.Run("Should fail on unknown strategy", func(t *testing.T) {
		_, err := New(Config{Strategy: "unknown"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown verification strategy")
	})
	t.Run("Should fail when hmac missing header", func(t *testing.T) {
		_, err := New(Config{Strategy: "hmac", Secret: "s"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing signature header name")
	})
	t.Run("Should fail when secret empty", func(t *testing.T) {
		_, err := New(Config{Strategy: "stripe", Secret: ""})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty secret")
	})
	t.Run("Should fail when env secret not set", func(t *testing.T) {
		_, err := New(Config{Strategy: "github", Secret: "env://MISSING_ENV_VAR"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "secret env not set")
	})
}
