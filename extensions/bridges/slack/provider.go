package main

import (
	"context"
	"encoding/json"
	"errors"

	"io"
	"net/http"

	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerAghBridgeDeliveryKey = "agh_bridge_delivery"
	providerBridgeInstanceIDKey  = "bridge_instance_id"
	providerChannelIDKey         = "channel_id"
	providerChannelNameKey       = "channel_name"
	providerDeliveryIDKey        = "delivery_id"
	providerMessageKey           = "message"
	providerReactionAddedKey     = "reaction_added"
	providerResponseURLKey       = "response_url"
	providerSlackKey             = "slack"
	providerTeamIDKey            = "team_id"
	providerTriggerIDKey         = "trigger_id"
	providerTypeKey              = "type"
)

const (
	slackListenAddrEnv = "AGH_BRIDGE_SLACK_LISTEN_ADDR"
	slackAPIBaseEnv    = "AGH_BRIDGE_SLACK_API_BASE_URL"

	slackDefaultAPIBaseURL        = "https://slack.com/api"
	slackSignatureVersion         = "v0"
	slackWebhookReadHeaderTimeout = 10 * time.Second
	slackWebhookIdleTimeout       = 2 * time.Minute
)

type slackProvider struct {
	sdk        *bridgesdk.Runtime
	lifecycle  *bridgesdk.ProviderLifecycle
	markers    *bridgesdk.AdapterMarkers
	http       *bridgesdk.ProviderHTTPServer
	stderr     io.Writer
	now        func() time.Time
	parents    *bridgesdk.ParentMessageCache
	routes     *bridgesdk.RouteTable[resolvedInstanceConfig]
	deliveries *bridgesdk.DeliveryStateStore[deliveryState]

	mu         sync.RWMutex
	lastError  string
	listenAddr string
	apiFactory func(*resolvedInstanceConfig) slackAPI
}

type slackProviderConfig struct {
	Webhook struct {
		ListenAddr string `json:"listen_addr,omitempty"`
		Path       string `json:"path,omitempty"`
	} `json:"webhook"`
	Batching struct {
		DelayMS        int `json:"delay_ms,omitempty"`
		SplitDelayMS   int `json:"split_delay_ms,omitempty"`
		SplitThreshold int `json:"split_threshold,omitempty"`
	} `json:"batching"`
	DM struct {
		AllowUserIDs    []string `json:"allow_user_ids,omitempty"`
		AllowUsernames  []string `json:"allow_usernames,omitempty"`
		PairedUserIDs   []string `json:"paired_user_ids,omitempty"`
		PairedUsernames []string `json:"paired_usernames,omitempty"`
	} `json:"dm"`
}

type resolvedInstanceConfig struct {
	managed            *subprocess.InitializeBridgeManagedInstance
	instanceID         string
	listenAddr         string
	webhookPath        string
	apiBaseURL         string
	botToken           string
	signingSecret      string
	dmPolicy           bridgepkg.BridgeDMPolicy
	allowUserIDs       map[string]struct{}
	allowUsernames     map[string]struct{}
	pairedUserIDs      map[string]struct{}
	pairedUsernames    map[string]struct{}
	dedup              *bridgesdk.DedupCache
	rateLimiter        *bridgesdk.FixedWindowRateLimiter
	inFlightLimiter    *bridgesdk.InFlightLimiter
	batcher            *bridgesdk.InboundBatcher
	configError        error
	initialDegradation *bridgepkg.BridgeDegradation
	initialStatus      bridgepkg.BridgeStatus
}

type slackWebhookEnvelope struct {
	Challenge string          `json:"challenge,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	EventID   string          `json:"event_id,omitempty"`
	EventTime int64           `json:"event_time,omitempty"`
	TeamID    string          `json:"team_id,omitempty"`
	Type      string          `json:"type"`
}

type slackEventTypePayload struct {
	Type string `json:"type"`
}

type slackMessageEvent struct {
	BotID           string                `json:"bot_id,omitempty"`
	Channel         string                `json:"channel,omitempty"`
	ChannelType     string                `json:"channel_type,omitempty"`
	DeletedTS       string                `json:"deleted_ts,omitempty"`
	Edited          *slackEdit            `json:"edited,omitempty"`
	Files           []slackFile           `json:"files,omitempty"`
	Message         *slackMessageSnapshot `json:"message,omitempty"`
	PreviousMessage *slackMessageSnapshot `json:"previous_message,omitempty"`
	Subtype         string                `json:"subtype,omitempty"`
	Team            string                `json:"team,omitempty"`
	TeamID          string                `json:"team_id,omitempty"`
	Text            string                `json:"text,omitempty"`
	ThreadTS        string                `json:"thread_ts,omitempty"`
	TS              string                `json:"ts,omitempty"`
	Type            string                `json:"type"`
	User            string                `json:"user,omitempty"`
	Username        string                `json:"username,omitempty"`
}

type slackMessageSnapshot struct {
	BotID    string      `json:"bot_id,omitempty"`
	Edited   *slackEdit  `json:"edited,omitempty"`
	Files    []slackFile `json:"files,omitempty"`
	Text     string      `json:"text,omitempty"`
	ThreadTS string      `json:"thread_ts,omitempty"`
	TS       string      `json:"ts,omitempty"`
	Type     string      `json:"type,omitempty"`
	User     string      `json:"user,omitempty"`
	Username string      `json:"username,omitempty"`
}

type slackEdit struct {
	TS string `json:"ts,omitempty"`
}

type slackFile struct {
	ID         string `json:"id,omitempty"`
	MIMEType   string `json:"mimetype,omitempty"`
	Name       string `json:"name,omitempty"`
	URLPrivate string `json:"url_private,omitempty"`
}

type slackReactionEvent struct {
	EventTS  string            `json:"event_ts,omitempty"`
	Item     slackReactionItem `json:"item"`
	ItemUser string            `json:"item_user,omitempty"`
	Reaction string            `json:"reaction,omitempty"`
	Type     string            `json:"type"`
	User     string            `json:"user,omitempty"`
}

type slackReactionItem struct {
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	Type    string `json:"type,omitempty"`
}

type slackBlockActionsPayload struct {
	Actions []slackBlockAction `json:"actions"`
	Channel struct {
		ID string `json:"id,omitempty"`
	} `json:"channel"`
	Container struct {
		Type        string `json:"type,omitempty"`
		MessageTS   string `json:"message_ts,omitempty"`
		ChannelID   string `json:"channel_id,omitempty"`
		IsEphemeral bool   `json:"is_ephemeral,omitempty"`
		ThreadTS    string `json:"thread_ts,omitempty"`
	} `json:"container"`
	Message struct {
		TS       string `json:"ts,omitempty"`
		ThreadTS string `json:"thread_ts,omitempty"`
	} `json:"message"`
	ResponseURL string `json:"response_url,omitempty"`
	TriggerID   string `json:"trigger_id,omitempty"`
	Type        string `json:"type"`
	User        struct {
		ID       string `json:"id,omitempty"`
		Name     string `json:"name,omitempty"`
		Username string `json:"username,omitempty"`
	} `json:"user"`
}

type slackBlockAction struct {
	ActionID       string `json:"action_id,omitempty"`
	ActionTS       string `json:"action_ts,omitempty"`
	BlockID        string `json:"block_id,omitempty"`
	Type           string `json:"type,omitempty"`
	Value          string `json:"value,omitempty"`
	SelectedOption *struct {
		Value string `json:"value,omitempty"`
	} `json:"selected_option,omitempty"`
}

type slackMappedInbound struct {
	Envelope bridgepkg.InboundMessageEnvelope
	Direct   bool
	User     slackUserIdentity
}

type slackUserIdentity struct {
	ID          string
	Username    string
	DisplayName string
}

type slackAPI interface {
	AuthTest(context.Context) (*slackAuthIdentity, error)
	PostMessage(context.Context, slackPostMessageRequest) (*slackPostedMessage, error)
	UpdateMessage(context.Context, slackUpdateMessageRequest) error
	DeleteMessage(context.Context, slackDeleteMessageRequest) error
	SetThreadStatus(context.Context, slackSetThreadStatusRequest) error
	AddReaction(context.Context, slackAddReactionRequest) error
}

type slackDeliveryReconciler interface {
	FindDeliveryMessage(context.Context, slackFindDeliveryMessageRequest) (*slackPostedMessage, error)
}

type slackAuthIdentity struct {
	BotID         string   `json:"bot_id,omitempty"`
	UserID        string   `json:"user_id,omitempty"`
	GrantedScopes []string `json:"-"`
}

type slackPostedMessage struct {
	TS string `json:"ts,omitempty"`
}

type slackPostMessageRequest struct {
	Channel  string                `json:"channel"`
	ThreadTS string                `json:"thread_ts,omitempty"`
	Text     string                `json:"text"`
	Mrkdwn   bool                  `json:"mrkdwn"`
	Metadata *slackMessageMetadata `json:"metadata,omitempty"`
}

type slackUpdateMessageRequest struct {
	Channel string `json:"channel"`
	TS      string `json:"ts"`
	Text    string `json:"text"`
}

type slackDeleteMessageRequest struct {
	Channel string `json:"channel"`
	TS      string `json:"ts"`
}

type slackFindDeliveryMessageRequest struct {
	Channel          string
	ThreadTS         string
	DeliveryID       string
	BridgeInstanceID string
}

func (r slackFindDeliveryMessageRequest) Validate() error {
	if strings.TrimSpace(r.Channel) == "" {
		return errors.New("slack: delivery reconciliation requires channel")
	}
	if strings.TrimSpace(r.DeliveryID) == "" {
		return errors.New("slack: delivery reconciliation requires delivery id")
	}
	if strings.TrimSpace(r.BridgeInstanceID) == "" {
		return errors.New("slack: delivery reconciliation requires bridge instance id")
	}
	return nil
}

type slackMessageMetadata struct {
	EventType    string                      `json:"event_type"`
	EventPayload slackMessageMetadataPayload `json:"event_payload"`
}

type slackMessageMetadataPayload struct {
	BridgeInstanceID string `json:"bridge_instance_id"`
	DeliveryID       string `json:"delivery_id"`
}

type slackConversationMessagesRequest struct {
	Channel   string `json:"channel"`
	Cursor    string `json:"cursor,omitempty"`
	Inclusive bool   `json:"inclusive,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	TS        string `json:"ts,omitempty"`
}

type slackConversationMessagesResponse struct {
	HasMore          bool                       `json:"has_more,omitempty"`
	Messages         []slackConversationMessage `json:"messages,omitempty"`
	ResponseMetadata *slackResponseMetadata     `json:"response_metadata,omitempty"`
}

type slackConversationMessage struct {
	TS       string                `json:"ts,omitempty"`
	Metadata *slackMessageMetadata `json:"metadata,omitempty"`
}

type slackResponseMetadata struct {
	NextCursor string `json:"next_cursor,omitempty"`
}

type slackAPIEnvelope struct {
	BotID  string `json:"bot_id,omitempty"`
	Error  string `json:"error,omitempty"`
	OK     bool   `json:"ok"`
	TS     string `json:"ts,omitempty"`
	UserID string `json:"user_id,omitempty"`
}

//nolint:funlen // Construction keeps the provider's declarative runtime wiring visible in one place.
func newSlackProvider(stderr io.Writer) (*slackProvider, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	provider := &slackProvider{
		stderr:  stderr,
		markers: bridgesdk.NewAdapterMarkers(providerSlackKey, stderr),
		now:     func() time.Time { return time.Now().UTC() },
		parents: bridgesdk.NewParentMessageCache(0),
		routes: bridgesdk.NewRouteTable(func(config resolvedInstanceConfig) []string {
			if config.configError != nil {
				return nil
			}
			return []string{config.webhookPath}
		}),
		deliveries: bridgesdk.NewDeliveryStateStore[deliveryState](),
	}
	provider.apiFactory = func(cfg *resolvedInstanceConfig) slackAPI {
		return &slackBotClient{
			baseURL:  cfg.apiBaseURL,
			botToken: cfg.botToken,
			httpClient: &http.Client{
				Timeout: 10 * time.Second,
			},
			reportResponseCleanup: func(err error) {
				provider.markers.ReportError("clean up Slack API response", err)
			},
		}
	}

	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: providerSlackKey,
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
		OnStop: provider.stopResources,
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
		ReadHeaderTimeout: slackWebhookReadHeaderTimeout,
		IdleTimeout:       slackWebhookIdleTimeout,
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
			Name:    providerSlackKey,
			Version: "0.1.0",
			SDKName: "bridgesdk",
		},
		Initialize:  lifecycle.Initialize,
		Deliver:     provider.handleBridgesDeliver,
		Progress:    provider.handleBridgesProgress,
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

func (p *slackProvider) serve(stdin io.Reader, stdout io.Writer) error {
	return p.lifecycle.Serve(context.Background(), p.sdk, stdin, stdout)
}
