package providerusage

import (
	"encoding/base64"
	"encoding/json"
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
func collectorEnv(t Target, key string) string {
	for _, entry := range slices.Backward(t.Route.Env) {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == key {
			return value
		}
	}
	return ""
}
func collectorHomePath(t Target, override string, parts ...string) (string, error) {
	if override != "" {
		if value := collectorEnv(t, override); value != "" {
			if filepath.IsAbs(value) {
				return value, nil
			}
			return "", NewUsageError(ErrorCodeAuthMissing, t.ProviderID)
		}
	}
	home := collectorEnv(t, "HOME")
	if home == "" {
		home = collectorEnv(t, "USERPROFILE")
	}
	if !filepath.IsAbs(home) {
		return "", NewUsageError(ErrorCodeAuthMissing, t.ProviderID)
	}
	return filepath.Join(append([]string{home}, parts...)...), nil
}
func readCollectorCredential(path, provider string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return NewUsageError(ErrorCodeAuthMissing, provider)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxResponseBytes+1))
	if err != nil || len(data) > MaxResponseBytes {
		return NewUsageError(ErrorCodeParseError, provider)
	}
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
