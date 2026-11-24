package globaldb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/store"
)

func TestGlobalDBAppTokenLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("Should store only a hash and keep read-only authentication from updating last use", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		database := openTestGlobalDB(t)
		service := appTokenServiceForTest(t, database)
		issued, err := service.CreateAppToken(
			ctx,
			"  Dashboard  ",
			[]string{"http://A.test:80", "http://a.test", "https://b.test"},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^cpz_app_[A-Za-z0-9_-]{43}$`).MatchString(issued.Secret) ||
			!strings.HasPrefix(issued.Token.ID, "apt_") || issued.Token.Name != "Dashboard" ||
			!reflect.DeepEqual(issued.Token.Origins, []string{"http://a.test", "https://b.test"}) {
			t.Fatalf("unexpected issued token metadata: %#v", issued.Token)
		}
		digest := sha256.Sum256([]byte(issued.Secret))
		var storedHash string
		if err := database.db.QueryRowContext(ctx, "SELECT token_hash FROM app_tokens WHERE id = ?", issued.Token.ID).
			Scan(&storedHash); err != nil {
			t.Fatal(err)
		}
		if storedHash != hex.EncodeToString(digest[:]) || storedHash == issued.Secret {
			t.Fatal("app token secret was not stored exclusively as its hash")
		}
		token, err := service.AuthenticateAppToken(ctx, issued.Secret)
		if err != nil || !token.LastSeenAt.IsZero() {
			t.Fatalf("read-only authentication = (%#v, %v)", token, err)
		}

	})

	t.Run(
		"Should rebuild origins on create update revoke and startup while retaining revoked audit rows",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			database := openTestGlobalDB(t)
			service := appTokenServiceForTest(t, database)
			issued, err := service.CreateAppToken(ctx, "Dashboard", []string{"http://a.test"})
			if err != nil || !service.AppTokenOriginAllowed("http://a.test") {
				t.Fatalf("create origin union: %v", err)
			}
			restarted := appTokenServiceForTest(t, database)
			if !restarted.AppTokenOriginAllowed("http://a.test") {
				t.Fatal("startup did not load persisted origin union")
			}
			origins := []string{"https://b.test"}
			updated, err := service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Name: new("Production"), Origins: &origins},
			)
			if err != nil || updated.Name != "Production" || service.AppTokenOriginAllowed("http://a.test") ||
				!service.AppTokenOriginAllowed("https://b.test") {
				t.Fatalf("update origin union: (%#v, %v)", updated, err)
			}
			updated, err = service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Origins: new([]string{})},
			)
			if err != nil || len(updated.Origins) != 0 || updated.Name != "Production" ||
				service.AppTokenOriginAllowed("https://b.test") {
				t.Fatalf("clear origins = (%#v, %v)", updated, err)
			}
			updated, err = service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Origins: &origins},
			)
			if err != nil || updated.Name != "Production" {
				t.Fatalf("replace origins = (%#v, %v)", updated, err)
			}
			stream, cancel := context.WithCancel(ctx)
			defer cancel()
			service.Connections().RegisterWaitable(issued.Token.ID, cancel, stream.Done())
			revoked, err := service.RevokeAppToken(ctx, issued.Token.ID)
			if err != nil || !revoked.Changed || revoked.Canceled != 1 || stream.Err() == nil ||
				service.AppTokenOriginAllowed("https://b.test") {
				t.Fatalf("revocation = (%#v, %v)", revoked, err)
			}
			again, err := service.RevokeAppToken(ctx, issued.Token.ID)
			if err != nil || again.Changed || again.Canceled != 0 ||
				again.Token.RevokedAt != revoked.Token.RevokedAt {
				t.Fatalf("repeat revocation = (%#v, %v)", again, err)
			}
			if _, err := service.AuthenticateAppToken(ctx, issued.Secret); !errors.Is(err, apptoken.ErrRevoked) {
				t.Fatalf("revoked token authentication: %v", err)
			}
			if err := service.TouchAppTokenLastSeen(
				ctx,
				issued.Token.ID,
				0,
			); !errors.Is(
				err,
				apptoken.ErrRevoked,
			) {
				t.Fatalf("revoked token last use: %v", err)
			}
			if _, err := service.UpdateAppToken(
				ctx,
				issued.Token.ID,
				apptoken.Patch{Name: new("No"), Origins: new([]string{})},
			); !errors.Is(
				err,
				apptoken.ErrRevoked,
			) {
				t.Fatalf("revoked token update: %v", err)
			}
			for _, state := range []apptoken.State{apptoken.StateAll, apptoken.StateRevoked} {
				tokens, err := service.ListAppTokens(ctx, state)
				if err != nil || len(tokens) != 1 || !reflect.DeepEqual(tokens[0], again.Token) {
					t.Fatalf("retained revoked inventory(%s) = (%#v, %v)", state, tokens, err)
				}
			}
			active, err := service.ListAppTokens(ctx, apptoken.StateActive)
			if err != nil || len(active) != 0 {
				t.Fatalf("active inventory = (%#v, %v)", active, err)
			}
			if _, err := service.ListAppTokens(ctx, "expired"); !errors.Is(err, apptoken.ErrStateInvalid) {
				t.Fatalf("invalid state filter: %v", err)
			}
		},
	)

	t.Run("Should list active tokens before revoked tokens in newest-first order", func(t *testing.T) {
		t.Parallel()
		database := openTestGlobalDB(t)
		now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
		service, err := apptoken.NewService(
			database,
			remotehost.NewMutationGate(database.RevalidateAppTokenEpoch),
			remotehost.NewConnectionRegistry(),
			func() time.Time { return now },
		)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, 3)
		for index := range ids {
			issued, err := service.CreateAppToken(t.Context(), fmt.Sprintf("App %d", index), nil)
			if err != nil {
				t.Fatal(err)
			}
			ids[index] = issued.Token.ID
			now = now.Add(time.Minute)
		}
		if _, err := service.RevokeAppToken(t.Context(), ids[1]); err != nil {
			t.Fatal(err)
		}
		tokens, err := service.ListAppTokens(t.Context(), apptoken.StateAll)
		if err != nil || len(tokens) != 3 || tokens[0].ID != ids[2] || tokens[1].ID != ids[0] ||
			tokens[2].ID != ids[1] {
			t.Fatalf("ordered inventory = %#v, %v", tokens, err)
		}
		if _, err := service.RevokeAppToken(t.Context(), "apt_missing"); !errors.Is(err, apptoken.ErrNotFound) {
			t.Fatalf("unknown token revoke error = %v", err)
		}
	})

	t.Run("Should revoke an attributed local request without waiting on its own lease", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		service := appTokenServiceForTest(t, openTestGlobalDB(t))
		issued, err := service.CreateAppToken(ctx, "Self", nil)
		if err != nil {
			t.Fatal(err)
		}
		lease, err := service.AcquireMutation(ctx, issued.Token.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer remotehost.ReleaseMutation(lease)
		fenced := store.ContextWithMutationCommitFence(lease, func(commitCtx context.Context) error {
			return service.RevalidateForCommit(commitCtx, issued.Token.ID, 0)
		})
		result, err := service.RevokeAppToken(fenced, issued.Token.ID)
		if err != nil || !result.Changed {
			t.Fatalf("self revoke = %#v, %v", result, err)
		}
	})

	t.Run("Should drain app mutation leases before committing revocation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		service := appTokenServiceForTest(t, openTestGlobalDB(t))
		issued, err := service.CreateAppToken(ctx, "Leased", nil)
		if err != nil {
			t.Fatal(err)
		}
		lease, err := service.AcquireMutation(ctx, issued.Token.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer remotehost.ReleaseMutation(lease)
		done := make(chan error, 1)
		go func() {
			_, err := service.RevokeAppToken(ctx, issued.Token.ID)
			done <- err
		}()
		select {
		case <-lease.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("revocation did not cancel the active mutation")
		}
		select {
		case err := <-done:
			t.Fatalf("revocation finished before lease drain: %v", err)
		default:
		}
		updateDone := make(chan error, 1)
		go func() {
			_, err := service.UpdateAppToken(
				lease,
				issued.Token.ID,
				apptoken.Patch{Name: new("Canceled")},
			)
			updateDone <- err
		}()
		select {
		case err := <-updateDone:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled mutation error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("revocation held a lock needed by the draining mutation")
		}
		remotehost.ReleaseMutation(lease)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if _, err := service.AcquireMutation(
			ctx,
			issued.Token.ID,
			0,
		); !errors.Is(
			err,
			remotehost.ErrDeviceRevoked,
		) {
			t.Fatalf("revoked token acquired mutation lease: %v", err)
		}
	})
}

// Invariant: limits are enforced at validation and atomically at persistence; owner: gateway/SQLite.
func TestGlobalDBAppTokenLimits(t *testing.T) {
	t.Parallel()
	t.Run("Should validate names and count only distinct normalized origins", func(t *testing.T) {
		t.Parallel()
		service := appTokenServiceForTest(t, openTestGlobalDB(t))
		for _, name := range []string{"   ", strings.Repeat("a", 65)} {
			if _, err := service.CreateAppToken(
				t.Context(),
				name,
				nil,
			); !errors.Is(
				err,
				apptoken.ErrNameInvalid,
			) {
				t.Fatalf("invalid token name: %v", err)
			}
		}
		origins := make([]string, 16)
		for index := range origins {
			origins[index] = fmt.Sprintf("https://app-%d.test", index)
		}
		issued, err := service.CreateAppToken(
			t.Context(),
			strings.Repeat("a", 64),
			append(origins, "https://APP-0.test:443"),
		)
		if err != nil || len(issued.Token.Origins) != 16 {
			t.Fatalf("distinct-origin boundary = (%#v, %v)", issued.Token, err)
		}
		if _, err := service.CreateAppToken(
			t.Context(),
			"Too many",
			append(origins, "https://another.test"),
		); !errors.Is(
			err,
			apptoken.ErrOriginLimit,
		) {
			t.Fatalf("origin limit: %v", err)
		}
	})

	t.Run(
		"Should reserve the final active slot across concurrent creators and release it on revoke",
		func(t *testing.T) {
			t.Parallel()
			database := openTestGlobalDB(t)
			service := appTokenServiceForTest(t, database)
			var firstID string
			for index := range 60 {
				issued, err := service.CreateAppToken(t.Context(), fmt.Sprintf("App %d", index), nil)
				if err != nil {
					t.Fatal(err)
				}
				firstID = issued.Token.ID
			}
			start := make(chan struct{})
			results := make(chan error, 10)
			var workers sync.WaitGroup
			for range 10 {
				creator := appTokenServiceForTest(t, database)
				workers.Go(func() {
					<-start
					_, err := creator.CreateAppToken(t.Context(), "Concurrent", nil)
					results <- err
				})
			}
			close(start)
			workers.Wait()
			close(results)
			var success, limited int
			for err := range results {
				switch {
				case err == nil:
					success++
				case errors.Is(err, apptoken.ErrLimit):
					limited++
				default:
					t.Fatalf("concurrent create: %v", err)
				}
			}
			if success != 4 || limited != 6 {
				t.Fatalf("concurrent outcomes = (%d successes, %d limited)", success, limited)
			}
			count, err := database.AppTokenRepo.queries.CountActiveAppTokens(t.Context())
			if err != nil || count != 64 {
				t.Fatalf("active token count = %d, %v; want 64", count, err)
			}
			if _, err := service.RevokeAppToken(t.Context(), firstID); err != nil {
				t.Fatal(err)
			}
			if _, err := service.CreateAppToken(t.Context(), "Replacement", nil); err != nil {
				t.Fatalf("revoked token still occupied an active slot: %v", err)
			}
		},
	)
}

func appTokenServiceForTest(t *testing.T, database *GlobalDB) *apptoken.Service {
	t.Helper()
	service, err := apptoken.NewService(
		database,
		remotehost.NewMutationGate(database.RevalidateAppTokenEpoch),
		remotehost.NewConnectionRegistry(),
		func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.LoadAppTokenOrigins(t.Context()); err != nil {
		t.Fatal(err)
	}
	return service
}
