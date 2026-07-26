package main

import (
	"context"

	"io"

	"net/http"

	"regexp"

	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerCreatedKey        = "created"
	providerGithubKey         = "github"
	providerInstallationIDKey = "installation_id"
	providerRepositoryKey     = "repository"
)

const (
	githubListenAddrEnv = "AGH_BRIDGE_GITHUB_LISTEN_ADDR"
	githubAPIBaseEnv    = "AGH_BRIDGE_GITHUB_API_BASE_URL"

	githubDefaultAPIBaseURL        = "https://api.github.com"
	githubWebhookReadHeaderTimeout = 10 * time.Second
	githubWebhookIdleTimeout       = 2 * time.Minute

	githubModePAT = "pat"
	githubModeApp = "app"

	githubThreadTypePR    = "pr"
	githubThreadTypeIssue = "issue"

	githubRemoteCommentKindIssue  = "issue"
	githubRemoteCommentKindReview = "review"
)

var (
	githubReviewThreadPattern = regexp.MustCompile(`^github:([^/]+)/([^:]+):([0-9]+):rc:([0-9]+)$`)
	githubIssueThreadPattern  = regexp.MustCompile(`^github:([^/]+)/([^:]+):issue:([0-9]+)$`)
	githubPRThreadPattern     = regexp.MustCompile(`^github:([^/]+)/([^:]+):([0-9]+)$`)
)

type githubProvider struct {
	sdk        *bridgesdk.Runtime
	lifecycle  *bridgesdk.ProviderLifecycle
	markers    *bridgesdk.AdapterMarkers
	http       *bridgesdk.ProviderHTTPServer
	stderr     io.Writer
	now        func() time.Time
	routes     *bridgesdk.RouteTable[resolvedInstanceConfig]
	deliveries *bridgesdk.DeliveryStateStore[deliveryState]

	mu                sync.RWMutex
	lastError         string
	listenAddr        string
	installationCache map[string]int64
	apiClients        map[string]githubAPI
	apiFactory        func(resolvedInstanceConfig) githubAPI
	rateLimiter       *bridgesdk.FixedWindowRateLimiter
	inFlightLimiter   *bridgesdk.InFlightLimiter
}

type deliveryState struct {
	LastSeq                int64
	RemoteMessageID        string
	ReplaceRemoteMessageID string
}

type githubProviderConfig struct {
	Mode           string `json:"mode,omitempty"`
	InstallationID int64  `json:"installation_id,omitempty"`
	BotLogin       string `json:"bot_login,omitempty"`
	Webhook        struct {
		ListenAddr string `json:"listen_addr,omitempty"`
		Path       string `json:"path,omitempty"`
	} `json:"webhook"`
	Repository struct {
		Owner    string `json:"owner,omitempty"`
		Name     string `json:"name,omitempty"`
		FullName string `json:"full_name,omitempty"`
	} `json:"repository"`
}

type resolvedInstanceConfig struct {
	managed            subprocess.InitializeBridgeManagedInstance
	instanceID         string
	listenAddr         string
	webhookPath        string
	apiBaseURL         string
	mode               string
	repoOwner          string
	repoName           string
	repoFullName       string
	installationID     int64
	webhookSecret      string
	token              string
	appID              string
	privateKey         string
	botLogin           string
	dedup              *bridgesdk.DedupCache
	configError        error
	initialDegradation *bridgepkg.BridgeDegradation
	initialStatus      bridgepkg.BridgeStatus
}

type githubRepository struct {
	ID       int64      `json:"id,omitempty"`
	Name     string     `json:"name,omitempty"`
	FullName string     `json:"full_name,omitempty"`
	Owner    githubUser `json:"owner"`
}

type githubUser struct {
	ID    int64  `json:"id,omitempty"`
	Login string `json:"login,omitempty"`
	Type  string `json:"type,omitempty"`
}

type githubIssueComment struct {
	ID        int64      `json:"id,omitempty"`
	Body      string     `json:"body,omitempty"`
	CreatedAt string     `json:"created_at,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
	HTMLURL   string     `json:"html_url,omitempty"`
	User      githubUser `json:"user"`
}

type githubReviewComment struct {
	ID          int64      `json:"id,omitempty"`
	Body        string     `json:"body,omitempty"`
	CreatedAt   string     `json:"created_at,omitempty"`
	UpdatedAt   string     `json:"updated_at,omitempty"`
	HTMLURL     string     `json:"html_url,omitempty"`
	Path        string     `json:"path,omitempty"`
	InReplyToID int64      `json:"in_reply_to_id,omitempty"`
	User        githubUser `json:"user"`
}

type githubIssuePayload struct {
	Action       string              `json:"action,omitempty"`
	Comment      githubIssueComment  `json:"comment"`
	Installation *githubInstallation `json:"installation,omitempty"`
	Issue        struct {
		Number      int64 `json:"number,omitempty"`
		PullRequest *struct {
			URL string `json:"url,omitempty"`
		} `json:"pull_request,omitempty"`
	} `json:"issue"`
	Repository githubRepository `json:"repository"`
	Sender     githubUser       `json:"sender"`
}

type githubReviewPayload struct {
	Action       string              `json:"action,omitempty"`
	Comment      githubReviewComment `json:"comment"`
	Installation *githubInstallation `json:"installation,omitempty"`
	PullRequest  struct {
		Number int64 `json:"number,omitempty"`
	} `json:"pull_request"`
	Repository githubRepository `json:"repository"`
	Sender     githubUser       `json:"sender"`
}

type githubInstallation struct {
	ID int64 `json:"id,omitempty"`
}

type githubMappedInbound struct {
	Envelope       bridgepkg.InboundMessageEnvelope
	InstallationID int64
}

type githubThreadRef struct {
	Owner           string
	Repo            string
	Number          int64
	Type            string
	ReviewCommentID int64
}

type githubRemoteCommentRef struct {
	Kind      string
	CommentID int64
}

//nolint:funlen // Construction keeps the provider's declarative runtime wiring visible in one place.
func newGitHubProvider(stderr io.Writer) (*githubProvider, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	provider := &githubProvider{
		stderr:  stderr,
		markers: bridgesdk.NewAdapterMarkers(providerGithubKey, stderr),
		now:     func() time.Time { return time.Now().UTC() },
		routes: bridgesdk.NewRouteTable(func(config resolvedInstanceConfig) []string {
			return []string{config.webhookPath}
		}),
		deliveries:        bridgesdk.NewDeliveryStateStore[deliveryState](),
		installationCache: make(map[string]int64),
		apiClients:        make(map[string]githubAPI),
		rateLimiter:       bridgesdk.NewFixedWindowRateLimiter(300, time.Minute),
		inFlightLimiter:   bridgesdk.NewInFlightLimiter(48),
	}
	provider.apiFactory = func(cfg resolvedInstanceConfig) githubAPI {
		provider.mu.Lock()
		defer provider.mu.Unlock()
		if client, ok := provider.apiClients[cfg.instanceID]; ok {
			return client
		}
		client := &githubClient{
			cfg: cfg,
			httpClient: &http.Client{
				Timeout: 10 * time.Second,
			},
			now: func() time.Time { return provider.now() },
		}
		provider.apiClients[cfg.instanceID] = client
		return client
	}

	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: providerGithubKey,
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
		ReadHeaderTimeout: githubWebhookReadHeaderTimeout,
		IdleTimeout:       githubWebhookIdleTimeout,
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
			Name:    providerGithubKey,
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

func (p *githubProvider) serve(stdin io.Reader, stdout io.Writer) error {
	return p.lifecycle.Serve(context.Background(), p.sdk, stdin, stdout)
}
