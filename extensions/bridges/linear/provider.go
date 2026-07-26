package main

import (
	"context"

	"io"

	"net/http"

	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerPromptedKey = "prompted"
)

const (
	providerAgentSessionEventValue = "AgentSessionEvent"
	providerCommentValue           = "Comment"
	providerActionKey              = "action"
	providerCommentKey             = "comment"
	providerCreateKey              = "create"
	providerCreatedKey             = "created"
	providerLinearKey              = "linear"
	providerModeKey                = "mode"
	providerOrganizationIDKey      = "organization_id"
	providerURLKey                 = "url"
)

const (
	linearListenAddrEnv = "AGH_BRIDGE_LINEAR_LISTEN_ADDR"
	linearAPIBaseEnv    = "AGH_BRIDGE_LINEAR_API_BASE_URL"

	linearDefaultAPIBaseURL  = "https://api.linear.app"
	linearDefaultWebhookPath = "/linear"
	linearOAuthPathSuffix    = "/oauth/token"

	linearModeComments      = "comments"
	linearModeAgentSessions = "agent_sessions"

	linearAuthModeAPIKey = "api_key"
	linearAuthModeOAuth  = "oauth"

	linearWebhookSkew              = time.Minute
	linearWebhookReadHeaderTimeout = 10 * time.Second
	linearWebhookIdleTimeout       = 30 * time.Second
	linearWebhookIngressTimeout    = 10 * time.Second
)

var (
	linearCommentSessionThreadPattern = regexp.MustCompile(`^linear:([^:]+):c:([^:]+):s:([^:]+)$`)
	linearIssueSessionThreadPattern   = regexp.MustCompile(`^linear:([^:]+):s:([^:]+)$`)
	linearCommentThreadPattern        = regexp.MustCompile(`^linear:([^:]+):c:([^:]+)$`)
	linearIssueThreadPattern          = regexp.MustCompile(`^linear:([^:]+)$`)
)

type linearProvider struct {
	sdk        *bridgesdk.Runtime
	lifecycle  *bridgesdk.ProviderLifecycle
	markers    *bridgesdk.AdapterMarkers
	http       *bridgesdk.ProviderHTTPServer
	stderr     io.Writer
	now        func() time.Time
	routes     *bridgesdk.RouteTable[resolvedInstanceConfig]
	deliveries *bridgesdk.DeliveryStateStore[deliveryState]

	mu                    sync.RWMutex
	lastError             string
	apiFactory            func(resolvedInstanceConfig) linearAPI
	rateLimiter           *bridgesdk.FixedWindowRateLimiter
	inFlight              *bridgesdk.InFlightLimiter
	webhookIngressTimeout time.Duration
}

type deliveryState struct {
	LastSeq                int64
	LastContent            string
	RemoteMessageID        string
	ReplaceRemoteMessageID string
}

type linearProviderConfig struct {
	OrganizationID string `json:"organization_id,omitempty"`
	Mode           string `json:"mode,omitempty"`
	AuthMode       string `json:"auth_mode,omitempty"`
	Webhook        struct {
		ListenAddr string `json:"listen_addr,omitempty"`
		Path       string `json:"path,omitempty"`
	} `json:"webhook"`
}

type resolvedInstanceConfig struct {
	managed            *subprocess.InitializeBridgeManagedInstance
	instanceID         string
	organizationID     string
	mode               string
	authMode           string
	listenAddr         string
	webhookPath        string
	apiBaseURL         string
	oauthTokenURL      string
	webhookSecret      string
	apiKey             string
	clientID           string
	clientSecret       string
	botUserID          string
	botDisplayName     string
	dedup              *bridgesdk.DedupCache
	configError        error
	initialDegradation *bridgepkg.BridgeDegradation
	initialStatus      bridgepkg.BridgeStatus
	oauthTokenCache    *linearOAuthTokenCache
}

type linearThreadRef struct {
	IssueID        string
	RootCommentID  string
	AgentSessionID string
}

type linearWebhookEnvelope struct {
	Type             string `json:"type,omitempty"`
	Action           string `json:"action,omitempty"`
	OrganizationID   string `json:"organizationId,omitempty"`
	WebhookID        string `json:"webhookId,omitempty"`
	WebhookTimestamp int64  `json:"webhookTimestamp,omitempty"`
}

type linearActor struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	URL       string `json:"url,omitempty"`
	Type      string `json:"type,omitempty"`
}

type linearCommentData struct {
	ID        string      `json:"id,omitempty"`
	Body      string      `json:"body,omitempty"`
	IssueID   string      `json:"issueId,omitempty"`
	UserID    string      `json:"userId,omitempty"`
	User      linearActor `json:"user"`
	CreatedAt string      `json:"createdAt,omitempty"`
	UpdatedAt string      `json:"updatedAt,omitempty"`
	ParentID  string      `json:"parentId,omitempty"`
}

type linearCommentWebhookPayload struct {
	Type             string            `json:"type,omitempty"`
	Action           string            `json:"action,omitempty"`
	CreatedAt        string            `json:"createdAt,omitempty"`
	OrganizationID   string            `json:"organizationId,omitempty"`
	URL              string            `json:"url,omitempty"`
	WebhookID        string            `json:"webhookId,omitempty"`
	WebhookTimestamp int64             `json:"webhookTimestamp,omitempty"`
	Data             linearCommentData `json:"data"`
	Actor            linearActor       `json:"actor"`
}

type linearAgentActivityPayload struct {
	ID        string `json:"id,omitempty"`
	Body      string `json:"body,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Content   struct {
		Type string `json:"type,omitempty"`
		Body string `json:"body,omitempty"`
	} `json:"content"`
}

type linearSessionComment struct {
	ID     string `json:"id,omitempty"`
	Body   string `json:"body,omitempty"`
	UserID string `json:"userId,omitempty"`
}

type linearAgentSession struct {
	ID              string                `json:"id,omitempty"`
	AppUserID       string                `json:"appUserId,omitempty"`
	IssueID         string                `json:"issueId,omitempty"`
	CommentID       string                `json:"commentId,omitempty"`
	SourceCommentID string                `json:"sourceCommentId,omitempty"`
	Comment         *linearSessionComment `json:"comment"`
	Creator         *linearActor          `json:"creator"`
}

type linearAgentSessionWebhookPayload struct {
	Type             string                      `json:"type,omitempty"`
	Action           string                      `json:"action,omitempty"`
	CreatedAt        string                      `json:"createdAt,omitempty"`
	AppUserID        string                      `json:"appUserId,omitempty"`
	OAuthClientID    string                      `json:"oauthClientId,omitempty"`
	OrganizationID   string                      `json:"organizationId,omitempty"`
	WebhookID        string                      `json:"webhookId,omitempty"`
	WebhookTimestamp int64                       `json:"webhookTimestamp,omitempty"`
	PromptContext    string                      `json:"promptContext,omitempty"`
	AgentSession     linearAgentSession          `json:"agentSession"`
	AgentActivity    *linearAgentActivityPayload `json:"agentActivity"`
	Actor            linearActor                 `json:"actor"`
}

//nolint:funlen // Construction keeps the provider's declarative runtime wiring visible in one place.
func newLinearProvider(stderr io.Writer) (*linearProvider, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	provider := &linearProvider{
		stderr:  stderr,
		markers: bridgesdk.NewAdapterMarkers(providerLinearKey, stderr),
		now:     func() time.Time { return time.Now().UTC() },
		routes: bridgesdk.NewRouteTable(func(config resolvedInstanceConfig) []string {
			return []string{config.webhookPath}
		}),
		deliveries:            bridgesdk.NewDeliveryStateStore[deliveryState](),
		rateLimiter:           bridgesdk.NewFixedWindowRateLimiter(300, time.Minute),
		inFlight:              bridgesdk.NewInFlightLimiter(48),
		webhookIngressTimeout: linearWebhookIngressTimeout,
	}
	provider.apiFactory = func(cfg resolvedInstanceConfig) linearAPI {
		return &linearClient{
			cfg: cfg,
			httpClient: &http.Client{
				Timeout: 10 * time.Second,
			},
			now: func() time.Time { return provider.now() },
		}
	}

	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: providerLinearKey,
		Markers:      provider.markers,
		Reconcile: func(
			ctx context.Context,
			managed []subprocess.InitializeBridgeManagedInstance,
		) ([]bridgesdk.ProviderInitialState, error) {
			configs := provider.reconcileInstanceConfigs(ctx, provider.lifecycle.Session(), managed)
			states := make([]bridgesdk.ProviderInitialState, 0, len(configs))
			for idx := range configs {
				states = append(states, bridgesdk.ProviderInitialState{
					BridgeInstanceID: configs[idx].instanceID,
					Status:           configs[idx].initialStatus,
					Degradation:      configs[idx].initialDegradation,
				})
			}
			return states, nil
		},
		FinalizeInitialize: func(err error) {
			if err != nil {
				provider.setLastError(err)
			}
		},
		ShutdownResources: func(ctx context.Context) error {
			if provider.http == nil {
				return nil
			}
			return provider.http.Shutdown(ctx)
		},
	})
	if err != nil {
		return nil, err
	}
	provider.lifecycle = lifecycle
	providerHTTP, err := bridgesdk.NewProviderHTTPServer(bridgesdk.ProviderHTTPConfig{
		ReadHeaderTimeout: linearWebhookReadHeaderTimeout,
		IdleTimeout:       linearWebhookIdleTimeout,
		Handler:           http.HandlerFunc(provider.serveWebhookHTTP),
		Go:                lifecycle.Go,
		OnError:           provider.setLastError,
	})
	if err != nil {
		return nil, err
	}
	provider.http = providerHTTP

	sdkRuntime, err := bridgesdk.NewRuntime(bridgesdk.RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    providerLinearKey,
			Version: "0.1.0",
			SDKName: "bridgesdk",
		},
		Initialize:  lifecycle.Initialize,
		Deliver:     provider.handleBridgesDeliver,
		Check:       provider.handleBridgeCheck,
		HealthCheck: func(context.Context, *bridgesdk.Session) error { return provider.healthCheck() },
		Shutdown:    lifecycle.Shutdown,
		Now:         func() time.Time { return provider.now() },
	})
	if err != nil {
		return nil, err
	}
	provider.sdk = sdkRuntime
	return provider, nil
}

func (p *linearProvider) serve(stdin io.Reader, stdout io.Writer) error {
	return p.lifecycle.Serve(context.Background(), p.sdk, stdin, stdout)
}

func (p *linearProvider) handleBridgesDeliver(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker := bridgesdk.DeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	}

	cfg, err := p.waitForInstanceConfig(ctx, strings.TrimSpace(request.Event.BridgeInstanceID), 500*time.Millisecond)
	if err != nil {
		marker.Error = err.Error()
		p.markers.RecordDelivery(marker)
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}
	if p.markers.ShouldCrashOnce() {
		p.markers.RecordDelivery(marker)
		p.markers.RecordCrash(map[string]any{
			"crashed":            true,
			"pid":                os.Getpid(),
			"delivery_id":        strings.TrimSpace(request.Event.DeliveryID),
			"bridge_instance_id": cfg.instanceID,
		})
		os.Exit(23)
	}

	api := p.apiFactory(cfg)
	ack, state, err := executeLinearDelivery(
		ctx,
		api,
		cfg,
		request,
		p.deliveryState(cfg.instanceID, request.Event.DeliveryID),
	)
	if err != nil {
		if bridgesdk.IsCommittedMutation(err) {
			p.deliveries.Delete(deliveryStateKey(cfg.instanceID, request.Event.DeliveryID))
		}
		marker.Error = err.Error()
		p.markers.RecordDelivery(marker)
		classified := bridgesdk.ClassifyError(err)
		_, _, reportErr := session.ReportClassifiedError(ctx, cfg.instanceID, classified)
		if reportErr != nil {
			p.setLastError(reportErr)
		} else {
			p.setLastError(err)
		}
		return bridgepkg.DeliveryAck{}, err
	}

	p.storeDeliveryState(cfg.instanceID, request.Event.DeliveryID, state)
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	} else {
		p.clearLastError()
	}

	marker.Ack = &ack
	p.markers.RecordDelivery(marker)
	return ack, nil
}
