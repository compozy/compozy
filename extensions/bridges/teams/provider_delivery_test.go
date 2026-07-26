package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

type teamsStatusSession struct{}

func (teamsStatusSession) SyncInstances(context.Context) ([]subprocess.InitializeBridgeManagedInstance, error) {
	return nil, nil
}

func (teamsStatusSession) CachedInstances() []subprocess.InitializeBridgeManagedInstance { return nil }

func (teamsStatusSession) GetBridgeInstance(context.Context, string) (*bridgepkg.BridgeInstance, error) {
	return nil, nil
}

func (teamsStatusSession) ReportBridgeInstanceState(
	_ context.Context,
	params bridgepkg.BridgesInstancesReportStateParams,
) (*bridgepkg.BridgeInstance, error) {
	return &bridgepkg.BridgeInstance{ID: params.BridgeInstanceID, Status: params.Status}, nil
}

func (teamsStatusSession) IngestBridgeMessage(
	context.Context,
	bridgepkg.InboundMessageEnvelope,
) (*bridgepkg.BridgesMessagesIngestResult, error) {
	return nil, nil
}

func TestTeamsContractHealthAggregation(t *testing.T) {
	t.Run("Should remain unhealthy while any reported instance is auth required", func(t *testing.T) {
		runtime, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		for instanceID, status := range map[string]bridgepkg.BridgeStatus{
			"brg-ready": bridgepkg.BridgeStatusReady,
			"brg-auth":  bridgepkg.BridgeStatusAuthRequired,
		} {
			if err := runtime.lifecycle.Host().ReportState(
				t.Context(),
				teamsStatusSession{},
				instanceID,
				status,
				nil,
			); err != nil {
				t.Fatalf("ReportState(%s) error = %v", instanceID, err)
			}
		}

		err = runtime.healthCheck()
		if err == nil || !strings.Contains(err.Error(), "brg-auth") {
			t.Fatalf("healthCheck() error = %v, want auth-required instance error", err)
		}

		runtime.clearLastError()
		err = runtime.healthCheck()
		if err == nil || !strings.Contains(err.Error(), "brg-auth") {
			t.Fatalf("healthCheck() after clearLastError() error = %v, want auth-required instance error", err)
		}
	})
}

func TestTeamsContractAuthorizationCache(t *testing.T) {
	// not parallel: loopback credentialed URL opt-in uses process environment.

	t.Run("Should reuse OpenID metadata and JWKS across authenticated webhooks", func(t *testing.T) {
		identity := newCountingTeamsIdentityServer(t)
		cfg := resolvedInstanceConfig{
			appID:             "app-id",
			serviceURL:        identity.ServiceURL(),
			openIDMetadataURL: identity.MetadataURL(),
		}
		body := []byte(teamsMessageWebhook(identity.ServiceURL(), "Need a summary"))
		for range 3 {
			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"http://127.0.0.1/teams/brg-1",
				strings.NewReader(string(body)),
			)
			if err != nil {
				t.Fatalf("http.NewRequestWithContext() error = %v", err)
			}
			req.Header.Set("Authorization", "Bearer "+identity.SignedToken(t, cfg.appID, cfg.serviceURL))
			if err := verifyTeamsAuthorization(context.Background(), req, body, cfg); err != nil {
				t.Fatalf("verifyTeamsAuthorization() error = %v", err)
			}
		}

		metadataCalls, jwksCalls := identity.Counts()
		if metadataCalls != 1 || jwksCalls != 1 {
			t.Fatalf("identity calls = metadata:%d jwks:%d, want 1/1", metadataCalls, jwksCalls)
		}
	})
}

func TestTeamsContractDeliveryReferences(t *testing.T) {
	t.Run("Should edit the explicitly referenced remote message before current state", func(t *testing.T) {
		api := &fakeTeamsAPI{nextActivityID: 900}
		targetRemote := testTeamsRemoteMessageID("conversation-a", "https://service.test", "activity-a")
		currentRemote := testTeamsRemoteMessageID("conversation-b", "https://service.test", "activity-b")
		request := bridgepkg.DeliveryRequest{
			Event: bridgepkg.DeliveryEvent{
				DeliveryID:       "delivery-edit",
				BridgeInstanceID: "brg-teams",
				RoutingKey:       testTeamsRoutingKey(),
				DeliveryTarget:   testTeamsDeliveryTarget(),
				Seq:              2,
				EventType:        bridgepkg.DeliveryEventTypeFinal,
				Final:            true,
				Operation:        bridgepkg.DeliveryOperationEdit,
				Reference: &bridgepkg.DeliveryMessageReference{
					RemoteMessageID: targetRemote,
				},
				Content: bridgepkg.MessageContent{Text: "updated"},
			},
		}

		_, _, err := executeTeamsDelivery(
			context.Background(),
			api,
			resolvedInstanceConfig{instanceID: "brg-teams"},
			request,
			deliveryState{LastSeq: 1, RemoteMessageID: currentRemote, LastContent: "updated"},
			nil,
			func(string, string) (teamsUserContext, bool) {
				return teamsUserContext{}, false
			},
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery(edit) error = %v", err)
		}
		if got, want := len(api.updateCalls), 1; got != want {
			t.Fatalf("len(api.updateCalls) = %d, want %d", got, want)
		}
		if got, want := api.updateCalls[0].ActivityID, "activity-a"; got != want {
			t.Fatalf("api.updateCalls[0].ActivityID = %q, want %q", got, want)
		}
	})

	t.Run("Should edit an explicit reference without delivery state", func(t *testing.T) {
		t.Parallel()

		api := &fakeTeamsAPI{}
		serviceURL := "https://reference.service.test"
		remoteID := testTeamsRemoteMessageID("conversation-reference", serviceURL, "activity-reference")
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-empty-state-edit",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       testTeamsRoutingKey(),
			DeliveryTarget:   testTeamsDeliveryTarget(),
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID},
			Content:          bridgepkg.MessageContent{Text: "referenced update"},
		}}

		ack, state, err := executeTeamsDelivery(
			context.Background(),
			api,
			resolvedInstanceConfig{instanceID: "brg-teams"},
			request,
			deliveryState{},
			nil,
			func(string, string) (teamsUserContext, bool) {
				return teamsUserContext{}, false
			},
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery() error = %v", err)
		}
		if got := len(api.createCalls); got != 0 {
			t.Fatalf("create calls = %d, want 0", got)
		}
		if got := len(api.sendCalls); got != 0 {
			t.Fatalf("send calls = %d, want 0", got)
		}
		if got, want := len(api.updateCalls), 1; got != want {
			t.Fatalf("update calls = %d, want %d", got, want)
		}
		if got := api.updateCalls[0].ServiceURL; got != serviceURL {
			t.Fatalf("update service URL = %q, want %q", got, serviceURL)
		}
		if got, want := api.updateCalls[0].ConversationID, "conversation-reference"; got != want {
			t.Fatalf("update conversation = %q, want %q", got, want)
		}
		if got, want := api.updateCalls[0].ActivityID, "activity-reference"; got != want {
			t.Fatalf("update activity = %q, want %q", got, want)
		}
		if got := ack.RemoteMessageID; got != remoteID {
			t.Fatalf("ack remote message id = %q, want %q", got, remoteID)
		}
		if got := state.RemoteMessageID; got != remoteID {
			t.Fatalf("state remote message id = %q, want %q", got, remoteID)
		}
	})

	t.Run("Should keep cross-delivery continuations with the referenced conversation", func(t *testing.T) {
		t.Parallel()

		api := &fakeTeamsAPI{nextActivityID: 950}
		serviceURL := "https://service.test"
		targetRemote := testTeamsRemoteMessageID("conversation-a", serviceURL, "activity-a")
		threadID := encodeTeamsThreadID(teamsThreadRef{
			ConversationID: "conversation-b;messageid=parent-b",
			ServiceURL:     serviceURL,
		})
		routing := testTeamsRoutingKey()
		routing.ThreadID = threadID
		target := testTeamsDeliveryTarget()
		target.ThreadID = threadID
		currentRemote := testTeamsRemoteMessageID("conversation-b", serviceURL, "activity-b")
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-cross-reference",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: targetRemote},
			Content:          bridgepkg.MessageContent{Text: strings.Repeat("teams cross reference ", 2_000)},
		}}

		_, _, err := executeTeamsDelivery(
			context.Background(),
			api,
			resolvedInstanceConfig{instanceID: "brg-teams", serviceURL: serviceURL},
			request,
			deliveryState{LastSeq: 1, RemoteMessageID: currentRemote},
			nil,
			func(string, string) (teamsUserContext, bool) {
				return teamsUserContext{}, false
			},
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery() error = %v", err)
		}
		if len(api.sendCalls) == 0 {
			t.Fatal("send calls = 0, want overflow continuations")
		}
		for index, call := range api.sendCalls {
			if got, want := call.ConversationID, "conversation-a"; got != want {
				t.Fatalf("continuation %d conversation = %q, want %q", index, got, want)
			}
			if got, want := call.ReplyToID, ""; got != want {
				t.Fatalf("continuation %d reply id = %q, want %q", index, got, want)
			}
		}
	})

	t.Run("Should delete a delivery-id reference from stored state", func(t *testing.T) {
		api := &fakeTeamsAPI{nextActivityID: 901}
		referencedRemote := testTeamsRemoteMessageID("conversation-ref", "https://service.test", "activity-ref")
		request := bridgepkg.DeliveryRequest{
			Event: bridgepkg.DeliveryEvent{
				DeliveryID:       "delivery-delete",
				BridgeInstanceID: "brg-teams",
				RoutingKey:       testTeamsRoutingKey(),
				DeliveryTarget:   testTeamsDeliveryTarget(),
				Seq:              2,
				Final:            true,
				EventType:        bridgepkg.DeliveryEventTypeDelete,
				Operation:        bridgepkg.DeliveryOperationDelete,
				Reference: &bridgepkg.DeliveryMessageReference{
					DeliveryID: "delivery-original",
				},
			},
		}

		_, _, err := executeTeamsDelivery(
			context.Background(),
			api,
			resolvedInstanceConfig{instanceID: "brg-teams"},
			request,
			deliveryState{LastSeq: 1},
			func(deliveryID string) (deliveryState, bool) {
				if deliveryID != "delivery-original" {
					return deliveryState{}, false
				}
				return deliveryState{RemoteMessageID: referencedRemote}, true
			},
			func(string, string) (teamsUserContext, bool) {
				return teamsUserContext{}, false
			},
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery(delete) error = %v", err)
		}
		if got, want := len(api.deleteCalls), 1; got != want {
			t.Fatalf("len(api.deleteCalls) = %d, want %d", got, want)
		}
		if got, want := api.deleteCalls[0].ActivityID, "activity-ref"; got != want {
			t.Fatalf("api.deleteCalls[0].ActivityID = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve delivery state when the referenced delete fails", func(t *testing.T) {
		t.Parallel()

		deleteErr := errors.New("delete unavailable")
		api := &fakeTeamsAPI{deleteErr: deleteErr}
		remoteID := testTeamsRemoteMessageID("conversation-delete", "https://service.test", "activity-delete")
		initialState := deliveryState{LastSeq: 1, RemoteMessageID: remoteID}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-delete-failure",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       testTeamsRoutingKey(),
			DeliveryTarget:   testTeamsDeliveryTarget(),
			Seq:              2,
			Final:            true,
			EventType:        bridgepkg.DeliveryEventTypeDelete,
			Operation:        bridgepkg.DeliveryOperationDelete,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID},
		}}

		ack, state, err := executeTeamsDelivery(
			context.Background(),
			api,
			resolvedInstanceConfig{instanceID: "brg-teams"},
			request,
			initialState,
			nil,
			func(string, string) (teamsUserContext, bool) {
				return teamsUserContext{}, false
			},
		)
		if !errors.Is(err, deleteErr) {
			t.Fatalf("executeTeamsDelivery() error = %v, want wrapped delete error", err)
		}
		if ack != (bridgepkg.DeliveryAck{}) || state != initialState {
			t.Fatalf("failed delete = ack:%#v state:%#v, want zero ack and unchanged state", ack, state)
		}
	})
}

func TestTeamsContractDeliveryRetry(t *testing.T) {
	t.Run("Should reuse a created conversation after the first activity send fails pre-commit", func(t *testing.T) {
		t.Parallel()

		sendErr := errors.New("Teams activity send failed before commit")
		api := &fakeTeamsAPI{
			createConversationID: "conversation-created-once",
			nextActivityID:       1_000,
			sendErrors:           []error{sendErr},
		}
		cfg := resolvedInstanceConfig{
			instanceID:  "brg-teams",
			serviceURL:  "https://service.test",
			appID:       "app-id",
			appTenantID: "tenant-id",
		}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-retry-conversation",
			BridgeInstanceID: cfg.instanceID,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-teams",
				BridgeInstanceID: cfg.instanceID,
				PeerID:           "29:user-retry",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: cfg.instanceID,
				PeerID:           "29:user-retry",
				Mode:             bridgepkg.DeliveryModeDirectSend,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeFinal,
			Final:     true,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		}}
		lookup := func(string, string) (teamsUserContext, bool) {
			return teamsUserContext{
				ServiceURL: cfg.serviceURL,
				TenantID:   cfg.appTenantID,
			}, true
		}

		_, retryState, err := executeTeamsDelivery(
			t.Context(),
			api,
			cfg,
			request,
			deliveryState{},
			nil,
			lookup,
		)
		if !errors.Is(err, sendErr) {
			t.Fatalf("first executeTeamsDelivery() error = %v, want send failure", err)
		}
		if bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("first executeTeamsDelivery() error = %v, want pre-commit failure", err)
		}

		ack, _, err := executeTeamsDelivery(
			t.Context(),
			api,
			cfg,
			request,
			retryState,
			nil,
			lookup,
		)
		if err != nil {
			t.Fatalf("retry executeTeamsDelivery() error = %v", err)
		}
		if got, want := len(api.createCalls), 1; got != want {
			t.Fatalf("conversation create calls = %d, want %d", got, want)
		}
		if got, want := len(api.sendAttempts), 2; got != want {
			t.Fatalf("activity send attempts = %d, want %d", got, want)
		}
		for index, call := range api.sendAttempts {
			if got, want := call.ConversationID, "conversation-created-once"; got != want {
				t.Fatalf("activity send %d conversation = %q, want %q", index, got, want)
			}
		}
		ref, err := decodeRemoteMessageID(ack.RemoteMessageID)
		if err != nil {
			t.Fatalf("decodeRemoteMessageID() error = %v", err)
		}
		if got, want := ref.ConversationID, "conversation-created-once"; got != want {
			t.Fatalf("ack conversation = %q, want %q", got, want)
		}
	})
}

type countingTeamsIdentityServer struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	keyID      string

	mu            sync.Mutex
	metadataCalls int
	jwksCalls     int
}

func newCountingTeamsIdentityServer(t *testing.T) *countingTeamsIdentityServer {
	t.Helper()
	enableTeamsLoopbackCredentialedURLsForTesting(t)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	identity := &countingTeamsIdentityServer{privateKey: privateKey, keyID: "teams-cache-key"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/openid/.well-known/openidconfiguration":
			identity.mu.Lock()
			identity.metadataCalls++
			identity.mu.Unlock()
			if err := json.NewEncoder(w).Encode(map[string]any{
				"issuer":   "https://api.botframework.com",
				"jwks_uri": identity.server.URL + "/openid/keys",
			}); err != nil {
				t.Errorf("encode Teams OpenID metadata error = %v", err)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/openid/keys":
			identity.mu.Lock()
			identity.jwksCalls++
			identity.mu.Unlock()
			pub := privateKey.PublicKey
			if err := json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{{
					"kty":          "RSA",
					"kid":          identity.keyID,
					"x5t":          identity.keyID,
					"n":            base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
					"e":            base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
					"endorsements": []string{"msteams"},
				}},
			}); err != nil {
				t.Errorf("encode Teams JWKS error = %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	identity.server = server
	t.Cleanup(server.Close)
	return identity
}

func (s *countingTeamsIdentityServer) ServiceURL() string {
	return s.server.URL
}

func (s *countingTeamsIdentityServer) MetadataURL() string {
	return s.server.URL + "/openid/.well-known/openidconfiguration"
}

func (s *countingTeamsIdentityServer) Counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metadataCalls, s.jwksCalls
}

func (s *countingTeamsIdentityServer) SignedToken(t *testing.T, appID string, serviceURL string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, teamsAuthClaims{
		ServiceURL: serviceURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://api.botframework.com",
			Audience:  jwt.ClaimStrings{appID},
			Subject:   "botframework",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	})
	token.Header["kid"] = s.keyID
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		t.Fatalf("token.SignedString() error = %v", err)
	}
	return signed
}

func testTeamsRemoteMessageID(conversationID string, serviceURL string, activityID string) string {
	return encodeRemoteMessageID(teamsRemoteMessageRef{
		ConversationID: conversationID,
		ServiceURL:     serviceURL,
		ActivityID:     activityID,
	})
}

func testTeamsRoutingKey() bridgepkg.RoutingKey {
	return bridgepkg.RoutingKey{
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-teams",
		BridgeInstanceID: "brg-teams",
		GroupID:          "19:channel@thread.tacv2",
	}
}

func testTeamsDeliveryTarget() bridgepkg.DeliveryTarget {
	return bridgepkg.DeliveryTarget{
		BridgeInstanceID: "brg-teams",
		GroupID:          "19:channel@thread.tacv2",
		Mode:             bridgepkg.DeliveryModeReply,
	}
}
