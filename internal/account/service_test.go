package account

import (
	"context"
	"crypto"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/jonboulle/clockwork"
	"github.com/zalando/go-keyring"
)

const testAccountID = "acc_test"

func TestAccountSignIn(t *testing.T) {
	t.Parallel()

	t.Run("Should persist an enrolled account and emit one change (UT-001)", func(t *testing.T) {
		t.Parallel()
		cloud := accountCloudStub{start: testGrant, poll: testAuthorized, enroll: testEnroll, profile: testProfile}
		s, store, clock := newAccountTest(t, &cloud, Installation{State: SignedOut})
		events, stop := s.SubscribeSignIn()
		defer stop()
		attempt := startTestLogin(t, s, "google")
		advanceSignIn(t, clock)
		expectSignIn(t, events, SignInEvent{AttemptID: attempt.AttemptID, State: "signed_in"})
		requireSignedIn(t, s.Status())
		requireSignedIn(t, store.current())
		s.mu.Lock()
		g := s.generation
		changes := g.replay("4294967296") // Beginning of the first signed-in generation.
		s.mu.Unlock()
		if len(changes) != 1 || changes[0].Event != "account.changed" || changes[0].Data.Kind != "sign_in" {
			t.Fatalf("account changes = %#v, want one sign-in event", changes)
		}
	})

	t.Run("Should expire at the deadline and discard a late authorization (UT-002)", func(t *testing.T) {
		t.Parallel()
		polling, release := make(chan struct{}, 1), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		cloud := accountCloudStub{start: testGrant, poll: func(string) (remoteproto.DeviceFlowTokenResponse, error) {
			polling <- struct{}{}
			<-release
			return testAuthorized("")
		}}
		s, store, clock := newAccountTest(t, &cloud, Installation{State: SignedOut})
		defer unblock()
		events, stop := s.SubscribeSignIn()
		defer stop()
		attempt := startTestLogin(t, s, "google")
		advanceSignIn(t, clock)
		awaitAccountTest(t, polling)
		clock.Advance(2 * time.Second)
		expectSignIn(
			t,
			events,
			SignInEvent{AttemptID: attempt.AttemptID, State: "expired", Code: "account_login_expired"},
		)
		unblock()
		s.Close()
		if status := s.Status(); status.State != SignedOut || store.current().AuthGeneration != 0 {
			t.Fatalf("expired sign-in changed installation = %#v", status)
		}
		for event := range events {
			t.Errorf("unexpected event after expiry: %#v", event)
		}
	})

	t.Run("Should keep local state signed out when the cloud refuses a connection (UT-003)", func(t *testing.T) {
		t.Parallel()
		cloud := accountCloudStub{start: func(string) (remoteproto.DeviceFlowResponse, error) {
			return remoteproto.DeviceFlowResponse{}, &cloudclient.Failure{
				Status: http.StatusServiceUnavailable, Code: "account_cloud_unreachable",
			}
		}}
		s, store, _ := newAccountTest(t, &cloud, Installation{State: SignedOut})
		_, err := s.StartLogin(t.Context(), "")
		if cloudclient.FailureCode(err) != "account_cloud_unreachable" ||
			s.Status().State != SignedOut || store.current().State != SignedOut {
			t.Fatalf("offline sign-in left state %s, error %v", s.Status().State, err)
		}
		// The service owns account admission only. IT-012 exercises local session creation.
	})

	t.Run("Should cancel the replaced cloud grant and ignore its late result (UT-124)", func(t *testing.T) {
		t.Parallel()
		polling, release, canceled := make(chan struct{}, 1), make(chan struct{}), make(chan string, 1)
		unblock := sync.OnceFunc(func() { close(release) })
		cloud := accountCloudStub{start: testGrant, cancelGrant: func(code string) { canceled <- code },
			poll: func(code string) (remoteproto.DeviceFlowTokenResponse, error) {
				if code == "google" {
					polling <- struct{}{}
					<-release
				}
				return testAuthorized(code)
			}}
		s, store, clock := newAccountTest(t, &cloud, Installation{State: SignedOut})
		defer unblock()
		events, stop := s.SubscribeSignIn()
		defer stop()
		first := startTestLogin(t, s, "google")
		advanceSignIn(t, clock)
		awaitAccountTest(t, polling)
		second := startTestLogin(t, s, "github")
		if code := awaitAccountTest(t, canceled); code != "google" {
			t.Fatalf("canceled grant = %q, want the first grant", code)
		}
		expectSignIn(t, events, SignInEvent{
			AttemptID: first.AttemptID,
			State:     "cancelled", //nolint:misspell // Account sign-in wire state.
		})
		expectSignIn(t, events, SignInEvent{AttemptID: second.AttemptID, State: "waiting"})
		unblock()
		s.Close()
		if status := store.current(); status.State != SignedOut || status.AuthGeneration != 0 {
			t.Fatalf("superseded grant changed persisted state: %#v", status)
		}
		for event := range events {
			t.Errorf("unexpected event after provider switch: %#v", event)
		}
	})

	t.Run(
		"Should replace the key of a revoked device and keep the cloud's admin decision (UT-126)",
		func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name  string
				admin bool
			}{
				{name: "Should preserve ordinary device trust"},
				{name: "Should honor admin recovery after fresh sign-in", admin: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					attempted := make(chan remoteproto.JWK, 2)
					cloud := accountCloudStub{start: testGrant, poll: testAuthorized,
						enroll: func(input remoteproto.DeviceEnrollRequest) (remoteproto.DeviceEnrollResponse, error) {
							attempted <- input.PublicJWK
							if len(attempted) == 1 {
								return remoteproto.DeviceEnrollResponse{}, &cloudclient.Failure{
									Status: http.StatusConflict, Code: "device_key_revoked",
								}
							}
							device, err := testEnroll(input)
							device.Admin = tc.admin
							return device, err
						},
						profile: func() (remoteproto.AccountSummary, error) {
							profile, err := testProfile()
							profile.Device.Admin = tc.admin
							return profile, err
						},
					}
					s, _, clock := newAccountTest(t, &cloud, Installation{State: SignedOut})
					oldKey, err := s.credentials.deviceKey(testAccountID, true)
					if err != nil {
						t.Fatal(err)
					}
					oldJWK, err := remoteproto.NewPublicJWK(oldKey.Public())
					if err != nil {
						t.Fatal(err)
					}
					events, stop := s.SubscribeSignIn()
					defer stop()
					attempt := startTestLogin(t, s, "google")
					advanceSignIn(t, clock)
					expectSignIn(t, events, SignInEvent{AttemptID: attempt.AttemptID, State: "signed_in"})
					first, second := awaitAccountTest(t, attempted), awaitAccountTest(t, attempted)
					updated, err := s.credentials.deviceKey(testAccountID, false)
					if err != nil {
						t.Fatal(err)
					}
					expected, err := remoteproto.NewPublicJWK(updated.Public())
					if err != nil {
						t.Fatal(err)
					}
					if first != oldJWK || second == first || second != expected ||
						s.Status().Profile.Device.Admin != tc.admin {
						t.Fatal("revoked key was not replaced or cloud admin decision was lost")
					}
				})
			}
		},
	)
}

func TestAccountRefresh(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a cold signed-in state and respect transient retry deadlines (UT-116)", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name, code string
			status     int
			delay      time.Duration
		}{
			{name: "Should retain sign-in on transport failure", code: "account_cloud_unreachable", status: 503, delay: time.Minute},
			{name: "Should retain sign-in on server failure", code: "account_cloud_unreachable", status: 500, delay: time.Minute},
			{name: "Should honor rate limit backoff", code: "rate_limited", status: 429, delay: 2 * time.Minute},
			{name: "Should honor cloud unavailability backoff", code: "account_cloud_unreachable", status: 503, delay: 2 * time.Minute},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				var calls atomic.Int32
				failure := &cloudclient.Failure{Code: tc.code, Status: tc.status}
				if tc.delay > time.Minute {
					failure.RetryAt = accountTestTime().Add(tc.delay)
				}
				cloud := accountCloudStub{refresh: func() (remoteproto.DeviceSessionTokens, error) {
					if calls.Add(1) == 1 {
						return remoteproto.DeviceSessionTokens{}, failure
					}
					return testTokens(), nil
				}}
				s, store, clock := newAccountTest(t, &cloud, signedInInstallation())
				if _, err := s.refresh(s.generation, true); err != failure || s.Status().State != SignedIn {
					t.Fatalf("cold offline refresh = %v, state %s", err, s.Status().State)
				}
				clock.Advance(tc.delay - time.Second)
				if _, err := s.refresh(s.generation, true); err != failure || calls.Load() != 1 {
					t.Fatalf("refresh before retry deadline = %v, calls %d", err, calls.Load())
				}
				clock.Advance(time.Second)
				if _, err := s.refresh(s.generation, true); err != nil || calls.Load() != 2 ||
					!store.current().LastValidatedAt.Equal(clock.Now()) {
					t.Fatalf(
						"refresh at retry deadline = %v, calls %d, persisted %#v",
						err,
						calls.Load(),
						store.current(),
					)
				}
			})
		}
	})

	t.Run("Should refresh exactly once after 24 hours while idle (UT-116)", func(t *testing.T) {
		t.Parallel()
		calls := make(chan struct{}, 2)
		cloud := accountCloudStub{refresh: func() (remoteproto.DeviceSessionTokens, error) {
			calls <- struct{}{}
			return testTokens(), nil
		}}
		s, store, clock := newAccountTest(t, &cloud, signedInInstallation())
		blockAccountClock(t, clock, 1)
		clock.Advance(24*time.Hour - time.Second)
		select {
		case <-calls:
			t.Fatal("idle refresh ran before 24 hours")
		default:
		}
		clock.Advance(time.Second)
		awaitAccountTest(t, calls)
		s.Close()
		if !store.current().LastValidatedAt.Equal(clock.Now()) || len(calls) != 0 {
			t.Fatalf("daily refresh did not persist exactly once: %#v", store.current())
		}
	})

	t.Run("Should persist explicit revocation or suspension and close the session (UT-116)", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ name, reason string }{
			{name: "Should require sign-in after device revocation", reason: "device_revoked"},
			{name: "Should require sign-in after account suspension", reason: "account_suspended"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				cloud := accountCloudStub{refresh: func() (remoteproto.DeviceSessionTokens, error) {
					return remoteproto.DeviceSessionTokens{}, &cloudclient.Failure{
						Status: http.StatusUnauthorized, Code: "reauth_required", Reason: tc.reason,
					}
				}}
				s, store, _ := newAccountTest(t, &cloud, signedInInstallation())
				events, stop := s.SubscribeSignIn()
				defer stop()
				g := s.generation
				previous := s.Status().AuthGeneration
				_, err := s.refresh(g, true)
				if reauthReason(err) != tc.reason || g.ctx.Err() == nil {
					t.Fatalf("reauth refresh = %v, previous lifetime error %v", err, g.ctx.Err())
				}
				state := store.current()
				if state.State != ReauthRequired || state.SignedOutReason != tc.reason ||
					state.AuthGeneration != previous+1 {
					t.Fatalf("persisted reauth state = %#v", state)
				}
				expectSignIn(t, events, SignInEvent{State: "reauth_required", SignedOutReason: tc.reason})
				s.Close()
				for event := range events {
					t.Errorf("duplicate account state event: %#v", event)
				}
			})
		}
	})

	t.Run("Should stay signed out on restart after logout (UT-116)", func(t *testing.T) {
		t.Parallel()
		cloud := accountCloudStub{}
		s, store, clock := newAccountTest(t, &cloud, signedInInstallation())
		// UT-093. Safety Invariant 11. An old account stream has one final eviction notice.
		sub, err := s.Subscribe("")
		if err != nil {
			t.Fatal(err)
		}
		defer sub.Close()
		if err := s.Logout(t.Context()); err != nil {
			t.Fatal(err)
		}
		select {
		case <-sub.Done:
			if event := sub.Eviction(); event == nil || event.Event != "account.changed" || !event.Data.EvictRemote {
				t.Fatalf("account eviction = %+v", event)
			}
		default:
			t.Fatal("the old account stream remained open after sign-out")
		}
		s.Close()
		reopened, err := New(
			t.Context(),
			store,
			&cloud,
			t.TempDir(),
			WithClock(clock),
			WithKeyring(&accountKeyringStub{}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(reopened.Close)
		if reopened.Status().State != SignedOut {
			t.Fatalf("reopened state = %s", reopened.Status().State)
		}
	})
}

type accountStoreStub struct {
	mu    sync.Mutex
	state Installation
}

func (s *accountStoreStub) current() Installation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *accountStoreStub) ReadInstallationAccount(context.Context) (Installation, error) {
	return s.current(), nil
}

func (s *accountStoreStub) SaveInstallationAccount(_ context.Context, _ int64, state Installation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	return nil
}

type accountKeyringStub struct{ entries sync.Map }

func (s *accountKeyringStub) Get(service, user string) (string, error) {
	value, ok := s.entries.Load([2]string{service, user})
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value.(string), nil
}

func (s *accountKeyringStub) Set(service, user, value string) error {
	s.entries.Store([2]string{service, user}, value)
	return nil
}

func (s *accountKeyringStub) Delete(service, user string) error {
	s.entries.Delete([2]string{service, user})
	return nil
}

type accountCloudStub struct {
	Cloud
	start       func(string) (remoteproto.DeviceFlowResponse, error)
	poll        func(string) (remoteproto.DeviceFlowTokenResponse, error)
	cancelGrant func(string)
	enroll      func(remoteproto.DeviceEnrollRequest) (remoteproto.DeviceEnrollResponse, error)
	profile     func() (remoteproto.AccountSummary, error)
	refresh     func() (remoteproto.DeviceSessionTokens, error)
}

func (s *accountCloudStub) StartDeviceFlow(
	_ context.Context,
	input remoteproto.DeviceFlowRequest,
) (remoteproto.DeviceFlowResponse, error) {
	return s.start(input.Provider)
}

func (s *accountCloudStub) PollDeviceFlow(_ context.Context, code string) (remoteproto.DeviceFlowTokenResponse, error) {
	return s.poll(code)
}

func (s *accountCloudStub) CancelDeviceFlow(_ context.Context, code string) error {
	s.cancelGrant(code)
	return nil
}

func (s *accountCloudStub) Enroll(
	_ context.Context,
	_ string,
	_ crypto.Signer,
	input remoteproto.DeviceEnrollRequest,
) (remoteproto.DeviceEnrollResponse, error) {
	return s.enroll(input)
}

func (s *accountCloudStub) Account(context.Context, cloudclient.Credentials) (remoteproto.AccountSummary, error) {
	return s.profile()
}

func (s *accountCloudStub) Refresh(context.Context, string, crypto.Signer) (remoteproto.DeviceSessionTokens, error) {
	return s.refresh()
}

func (*accountCloudStub) Logout(context.Context, string, crypto.Signer) error { return nil }

func accountTestTime() time.Time { return time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC) }

func signedInInstallation() Installation {
	profile, _ := testProfile()
	return Installation{
		State:           SignedIn,
		Source:          "session",
		AccountID:       testAccountID,
		Profile:         profile,
		LastValidatedAt: accountTestTime(),
	}
}

func newAccountTest(
	t *testing.T,
	cloud Cloud,
	initial Installation,
) (*Service, *accountStoreStub, *clockwork.FakeClock) {
	t.Helper()
	initial.InstallationID = "ins_test"
	store := &accountStoreStub{state: initial}
	clock := clockwork.NewFakeClockAt(accountTestTime())
	s, err := New(t.Context(), store, cloud, t.TempDir(), WithClock(clock), WithKeyring(&accountKeyringStub{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if initial.State == SignedIn {
		if _, err := s.credentials.deviceKey(initial.AccountID, true); err != nil {
			t.Fatal(err)
		}
		if err := s.credentials.write(refreshEntry, initial.AccountID, "refresh"); err != nil {
			t.Fatal(err)
		}
	}
	return s, store, clock
}

func testGrant(provider string) (remoteproto.DeviceFlowResponse, error) {
	return remoteproto.DeviceFlowResponse{DeviceCode: provider, Interval: 1, ExpiresIn: 3}, nil
}

func testAuthorized(string) (remoteproto.DeviceFlowTokenResponse, error) {
	return remoteproto.DeviceFlowTokenResponse{
		AccountID:       testAccountID,
		EnrollmentToken: "enroll",
		Email:           "pedro@example.com",
	}, nil
}

func testProfile() (remoteproto.AccountSummary, error) {
	return remoteproto.AccountSummary{Email: "pedro@example.com", Plan: "free",
		Device: remoteproto.AccountDevice{ID: "dev_test", Admin: true}}, nil
}

func testEnroll(remoteproto.DeviceEnrollRequest) (remoteproto.DeviceEnrollResponse, error) {
	return remoteproto.DeviceEnrollResponse{DeviceID: "dev_test", Admin: true, DeviceSessionTokens: testTokens()}, nil
}

func testTokens() remoteproto.DeviceSessionTokens {
	return remoteproto.DeviceSessionTokens{AccessToken: "access", RefreshToken: "refresh"}
}

func requireSignedIn(t *testing.T, installation Installation) {
	t.Helper()
	if installation.State != SignedIn || installation.Profile.Email != "pedro@example.com" ||
		installation.Profile.Plan != "free" {
		t.Fatalf("installation = %#v, want signed-in Free account", installation)
	}
}

func startTestLogin(t *testing.T, s *Service, provider string) Login {
	t.Helper()
	attempt, err := s.StartLogin(t.Context(), provider)
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func advanceSignIn(t *testing.T, clock *clockwork.FakeClock) {
	t.Helper()
	blockAccountClock(t, clock, 2)
	clock.Advance(time.Second)
}

func blockAccountClock(t *testing.T, clock *clockwork.FakeClock, timers int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := clock.BlockUntilContext(ctx, timers); err != nil {
		t.Fatalf("wait for %d account timers: %v", timers, err)
	}
}

func awaitAccountTest[T any](t *testing.T, source <-chan T) T {
	t.Helper()
	select {
	case value, open := <-source:
		if !open {
			t.Fatal("account event stream closed")
		}
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for account operation")
		var zero T
		return zero
	}
}

func expectSignIn(t *testing.T, events <-chan SignInEvent, want SignInEvent) {
	t.Helper()
	for {
		event := awaitAccountTest(t, events)
		if event.State == want.State {
			if event != want {
				t.Fatalf("sign-in event = %#v, want %#v", event, want)
			}
			return
		}
	}
}
