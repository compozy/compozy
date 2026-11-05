package providers

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

// Invariant: a stored sign-in verdict replaces only the declared native CLI fallback and is served
// only for the provider config and auth home it was checked against. Owner: AuthVerdicts.
func TestAuthVerdicts(t *testing.T) {
	t.Parallel()

	provider := compozyconfig.ProviderConfig{
		Command:       "acme acp",
		AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
		AuthStatusCmd: "acme auth status",
	}
	authenticated := Classification{State: ProviderAuthStateAuthenticated}
	fallback := Classification{
		State:      ProviderAuthStateUnknown,
		Code:       diagcontract.CodeProviderClassificationUnknown,
		Unverified: true,
	}

	t.Run("Should replace only the unverified fallback", func(t *testing.T) {
		t.Parallel()

		env := []string{"HOME=/daemon"}
		verdicts := NewAuthVerdicts()
		verdicts.Record("acme", provider, env, authenticated, time.Now())

		got, checkedAt := verdicts.Apply("acme", provider, env, fallback)
		if got.State != ProviderAuthStateAuthenticated || checkedAt.IsZero() {
			t.Fatalf("Apply(fallback) = %q at %v, want the stored verdict with its time", got.State, checkedAt)
		}
		resolverErr := unknownClassification(errors.New("permission denied"))
		got, checkedAt = verdicts.Apply("acme", provider, env, resolverErr)
		if got != resolverErr || !checkedAt.IsZero() {
			t.Fatalf("Apply(resolver error) = %#v at %v, want the declared error without a time", got, checkedAt)
		}
	})

	t.Run("Should serve a verdict only for the auth home it was checked against", func(t *testing.T) {
		t.Parallel()

		seatB := []string{"HOME=/seat-b"}
		verdicts := NewAuthVerdicts()
		verdicts.Record("acme", provider, seatB, authenticated, time.Now())
		if _, ok := verdicts.Lookup("acme", provider, seatB); !ok {
			t.Fatal("Lookup(same home) missed the recorded verdict")
		}
		if _, ok := verdicts.Lookup("acme", provider, []string{"HOME=/daemon"}); ok {
			t.Fatal("Lookup(another home) found the verdict")
		}
		if _, ok := verdicts.Lookup("acme", provider, seatB); ok {
			t.Fatal("Lookup(same home after mismatch) found the verdict, want it dropped")
		}

		verdicts.Record("acme", provider, seatB, authenticated, time.Now())
		entry := verdicts.entries["acme"]
		if entry.dropped {
			t.Fatal("Record after a dropped verdict did not store the newer verdict")
		}
		if bytes.Contains(entry.fingerprint[:], []byte("/seat-b")) {
			t.Fatal("stored fingerprint contains the raw auth home")
		}
	})

	t.Run("Should keep the probe order after a mismatch drops the verdict", func(t *testing.T) {
		t.Parallel()

		seatA := []string{"HOME=/seat-a"}
		newer := time.Now()
		verdicts := NewAuthVerdicts()
		verdicts.Record("acme", provider, seatA, Classification{State: ProviderAuthStateNeedsLogin}, newer)
		if _, ok := verdicts.Lookup("acme", provider, []string{"HOME=/daemon"}); ok {
			t.Fatal("Lookup(another home) found the verdict")
		}
		verdicts.Record("acme", provider, seatA, authenticated, newer.Add(-time.Second))
		if got, ok := verdicts.Lookup("acme", provider, seatA); ok {
			t.Fatalf("Lookup after an older probe finished = %q, want no verdict", got.Classification.State)
		}
	})

	t.Run("Should distinguish USERPROFILE homes with platform key semantics", func(t *testing.T) {
		t.Parallel()

		seatA := authVerdictHomeEnv([]string{`USERPROFILE=C:\seat-a`}, false)
		seatB := authVerdictHomeEnv([]string{`USERPROFILE=C:\seat-b`}, false)
		if slices.Equal(seatA, seatB) {
			t.Fatalf("auth home env %q equals %q, want distinct USERPROFILE homes", seatA, seatB)
		}
		windows := authVerdictHomeEnv([]string{`UserProfile=C:\seat-a`}, true)
		if !slices.Equal(windows, seatA) {
			t.Fatalf("Windows auth home env = %q, want %q", windows, seatA)
		}
		if unix := authVerdictHomeEnv([]string{`UserProfile=C:\seat-a`}, false); len(unix) != 0 {
			t.Fatalf("Unix auth home env = %q, want case-sensitive keys", unix)
		}
	})

	t.Run("Should ignore env outside the auth home keys", func(t *testing.T) {
		t.Parallel()

		left := authVerdictFingerprint(provider, []string{"HOME=/daemon", "PATH=/a", "TERM=xterm"})
		right := authVerdictFingerprint(provider, []string{"PATH=/b", "HOME=/daemon"})
		if left != right {
			t.Fatal("fingerprint changed with env outside the auth home keys")
		}
		if left == authVerdictFingerprint(provider, []string{"HOME=/daemon", "CODEX_HOME=/codex"}) {
			t.Fatal("fingerprint ignored CODEX_HOME")
		}
	})
}
