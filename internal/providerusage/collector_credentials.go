package providerusage

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type collectorCredential struct{ token, account, email, domain string }

func (c collectorCredential) identity() CredentialIdentity {
	return NewCredentialIdentity(SourceKindOAuthHTTP, []byte(c.token), c.account)
}
func routeEnvValue(t Target, key string) (string, bool) { return envValue(t.Route.Env, key) }
func envValue(env []string, key string) (string, bool) {
	for _, entry := range slices.Backward(env) {
		if value, ok := strings.CutPrefix(entry, key+"="); ok {
			return value, true
		}
	}
	return "", false
}
func routeEnv(t Target, key string) string { value, _ := routeEnvValue(t, key); return value }

// homeSelectorKey is the variable a home-based collector reads its directory from: override when it
// is set non-empty, otherwise HOME.
func homeSelectorKey(env []string, override string) string {
	if override != "" {
		if value, _ := envValue(env, override); value != "" {
			return override
		}
	}
	return "HOME"
}
func collectorHomePath(t Target, override string, parts ...string) (string, error) {
	if key := homeSelectorKey(t.Route.Env, override); key != "HOME" {
		if value := routeEnv(t, key); filepath.IsAbs(value) {
			return value, nil
		}
		return "", NewUsageError(ErrorCodeAuthMissing, t.ProviderID)
	}
	home := routeEnv(t, "HOME")
	if home == "" {
		home = routeEnv(t, "USERPROFILE")
	}
	if !filepath.IsAbs(home) {
		return "", NewUsageError(ErrorCodeAuthMissing, t.ProviderID)
	}
	return filepath.Join(append([]string{home}, parts...)...), nil
}
func readCredentialFile(path, provider string) ([]byte, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxResponseBytes+1))
	if err != nil || len(data) > MaxResponseBytes {
		clear(data)
		return nil, NewUsageError(ErrorCodeParseError, provider)
	}
	return data, nil
}
func readCollectorCredential(path, provider string, out any) error {
	data, err := readCredentialFile(path, provider)
	if err != nil {
		if usage, ok := errors.AsType[*UsageError](err); ok {
			return usage
		}
		return NewUsageError(ErrorCodeAuthMissing, provider)
	}
	defer clear(data)
	return decodeCollectorJSON(data, out, provider)
}

type collectorClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Domain  string `json:"hd"`
	Expiry  int64  `json:"exp"`
}

func readCollectorClaims(token string) (collectorClaims, error) {
	var claims collectorClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, NewUsageError(ErrorCodeParseError, "credential")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, NewUsageError(ErrorCodeParseError, "credential")
	}
	err = json.Unmarshal(data, &claims)
	return claims, err
}
func checkedCollectorCredential(value collectorCredential, cred CredentialIdentity, provider string) error {
	if !value.identity().Same(cred) {
		return NewUsageError(ErrorCodeAuthMissing, provider)
	}
	return nil
}
func expiredCollectorToken(expiry int64, now time.Time) bool {
	return expiry <= 0 || !time.Unix(expiry, 0).After(now.Add(time.Minute))
}
