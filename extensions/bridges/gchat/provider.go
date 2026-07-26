package main

import (
	"context"
	"crypto/rsa"

	"encoding/json"

	"io"
	"net/http"

	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerAudKey        = "aud"
	providerExpKey        = "exp"
	providerGchatKey      = "gchat"
	providerIatKey        = "iat"
	providerIssKey        = "iss"
	providerNameKey       = "name"
	providerSourceKey     = "source"
	providerSpaceNameKey  = "space_name"
	providerSuccessKey    = "success"
	providerTextKey       = "text"
	providerThreadNameKey = "thread_name"
)

const (
	gchatListenAddrEnv  = "AGH_BRIDGE_GCHAT_LISTEN_ADDR"
	gchatAPIBaseEnv     = "AGH_BRIDGE_GCHAT_API_BASE_URL"
	gchatDirectCertsEnv = "AGH_BRIDGE_GCHAT_DIRECT_CERTS_URL"
	gchatPubSubCertsEnv = "AGH_BRIDGE_GCHAT_PUBSUB_CERTS_URL"

	gchatDefaultAPIBaseURL      = "https://chat.googleapis.com"
	gchatDefaultAuthEndpointURL = "https://oauth2.googleapis.com/token"
	gchatDefaultDirectCertsURL  = "https://www.googleapis.com/service_accounts/v1/metadata/x509/" +
		"chat@system.gserviceaccount.com"
	gchatDefaultPubSubCertsURL    = "https://www.googleapis.com/oauth2/v1/certs"
	gchatDefaultDirectIssuer      = "chat@system.gserviceaccount.com"
	gchatDefaultPubSubIssuerURL   = "https://accounts.google.com"
	gchatBotScope                 = "https://www.googleapis.com/auth/chat.bot"
	gchatWebhookReadHeaderTimeout = 10 * time.Second
	gchatWebhookIdleTimeout       = 2 * time.Minute
	gchatCertFetchTimeout         = 5 * time.Second
	gchatCertCacheFallbackTTL     = 5 * time.Minute

	gchatModeDirect = "direct"
	gchatModePubSub = "pubsub"
	gchatModeHybrid = "hybrid"

	gchatReplyMode = "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD"
)

var gchatTokenURLEnv = strings.Join([]string{"AGH", "BRIDGE", "GCHAT", "TOKEN", "URL"}, "_")

var reactionMessagePattern = regexp.MustCompile(`^(spaces/[^/]+/messages/[^/]+)/reactions/[^/]+$`)

var defaultGoogleX509KeyCache = newGoogleX509KeyCache(
	&http.Client{Timeout: gchatCertFetchTimeout},
	gchatCertCacheFallbackTTL,
	func() time.Time { return time.Now().UTC() },
)

type gchatProvider struct {
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
	apiFactory func(*resolvedInstanceConfig) gchatAPI
	apiClients map[string]cachedGChatAPIClient
}

type cachedGChatAPIClient struct {
	key    string
	client *gchatBotClient
}

type gchatProviderConfig struct {
	Mode    string `json:"mode,omitempty"`
	Webhook struct {
		ListenAddr string `json:"listen_addr,omitempty"`
		Path       string `json:"path,omitempty"`
	} `json:"webhook"`
	Verification struct {
		DirectCertsURL       string `json:"direct_certs_url,omitempty"`
		DirectIssuer         string `json:"direct_issuer,omitempty"`
		PubSubAudience       string `json:"pubsub_audience,omitempty"`
		PubSubCertsURL       string `json:"pubsub_certs_url,omitempty"`
		PubSubIssuer         string `json:"pubsub_issuer,omitempty"`
		PubSubServiceAccount string `json:"pubsub_service_account_email,omitempty"`
	} `json:"verification"`
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

type serviceAccountCredentials struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	ProjectID   string `json:"project_id,omitempty"`
	TokenURI    string `json:"token_uri,omitempty"`
}

type googleX509KeyCache struct {
	mu          sync.Mutex
	client      *http.Client
	fallbackTTL time.Duration
	now         func() time.Time
	entries     map[string]googleX509KeyCacheEntry
}

type googleX509KeyCacheEntry struct {
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

type resolvedInstanceConfig struct {
	managed                   subprocess.InitializeBridgeManagedInstance
	instanceID                string
	listenAddr                string
	webhookPath               string
	apiBaseURL                string
	tokenURL                  string
	mode                      string
	credentials               serviceAccountCredentials
	projectNumber             string
	directIssuer              string
	directCertsURL            string
	pubsubAudience            string
	pubsubIssuer              string
	pubsubCertsURL            string
	pubsubServiceAccountEmail string
	dmPolicy                  bridgepkg.BridgeDMPolicy
	allowUserIDs              map[string]struct{}
	allowUsernames            map[string]struct{}
	pairedUserIDs             map[string]struct{}
	pairedUsernames           map[string]struct{}
	dedup                     *bridgesdk.DedupCache
	rateLimiter               *bridgesdk.FixedWindowRateLimiter
	inFlightLimiter           *bridgesdk.InFlightLimiter
	batcher                   *bridgesdk.InboundBatcher
	configError               error
	initialDegradation        *bridgepkg.BridgeDegradation
	initialStatus             bridgepkg.BridgeStatus
}

type gchatWebhookProbe struct {
	Subscription string           `json:"subscription,omitempty"`
	Message      gchatPubSubInner `json:"message"`
	Chat         *json.RawMessage `json:"chat,omitempty"`
}

type gchatPubSubPushMessage struct {
	Message      gchatPubSubInner `json:"message"`
	Subscription string           `json:"subscription,omitempty"`
}

type gchatPubSubInner struct {
	Data        string            `json:"data,omitempty"`
	MessageID   string            `json:"messageId,omitempty"`
	PublishTime string            `json:"publishTime,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type gchatWorkspaceEventNotification struct {
	Subscription   string         `json:"subscription"`
	TargetResource string         `json:"target_resource"`
	EventType      string         `json:"event_type"`
	EventTime      string         `json:"event_time"`
	Message        *gchatMessage  `json:"message,omitempty"`
	Reaction       *gchatReaction `json:"reaction,omitempty"`
}

type gchatEvent struct {
	Chat *struct {
		User           *gchatUser `json:"user,omitempty"`
		EventTime      string     `json:"eventTime,omitempty"`
		MessagePayload *struct {
			Space   gchatSpace   `json:"space"`
			Message gchatMessage `json:"message"`
		} `json:"messagePayload,omitempty"`
		AddedToSpacePayload *struct {
			Space gchatSpace `json:"space"`
		} `json:"addedToSpacePayload,omitempty"`
		RemovedFromSpacePayload *struct {
			Space gchatSpace `json:"space"`
		} `json:"removedFromSpacePayload,omitempty"`
		ButtonClickedPayload *struct {
			Space   gchatSpace   `json:"space"`
			Message gchatMessage `json:"message"`
			User    gchatUser    `json:"user"`
		} `json:"buttonClickedPayload,omitempty"`
	} `json:"chat,omitempty"`
	CommonEventObject *struct {
		InvokedFunction string            `json:"invokedFunction,omitempty"`
		Parameters      map[string]string `json:"parameters,omitempty"`
	} `json:"commonEventObject,omitempty"`
}

type gchatMessage struct {
	Name                  string                      `json:"name"`
	Text                  string                      `json:"text,omitempty"`
	ArgumentText          string                      `json:"argumentText,omitempty"`
	FormattedText         string                      `json:"formattedText,omitempty"`
	CreateTime            string                      `json:"createTime,omitempty"`
	Sender                gchatUser                   `json:"sender"`
	Space                 *gchatSpace                 `json:"space,omitempty"`
	Thread                *gchatThread                `json:"thread,omitempty"`
	Attachment            []gchatAttachment           `json:"attachment,omitempty"`
	Annotations           []gchatAnnotation           `json:"annotations,omitempty"`
	QuotedMessageMetadata *gchatQuotedMessageMetadata `json:"quotedMessageMetadata,omitempty"`
}

type gchatQuotedMessageMetadata struct {
	Name                  string                      `json:"name,omitempty"`
	LastUpdateTime        string                      `json:"lastUpdateTime,omitempty"`
	QuoteType             string                      `json:"quoteType,omitempty"`
	QuotedMessageSnapshot *gchatQuotedMessageSnapshot `json:"quotedMessageSnapshot,omitempty"`
}

type gchatQuotedMessageSnapshot struct {
	Text          string `json:"text,omitempty"`
	FormattedText string `json:"formattedText,omitempty"`
	Sender        string `json:"sender,omitempty"`
}

type gchatSpace struct {
	Name                string `json:"name"`
	Type                string `json:"type,omitempty"`
	SpaceType           string `json:"spaceType,omitempty"`
	DisplayName         string `json:"displayName,omitempty"`
	SingleUserBotDM     bool   `json:"singleUserBotDm,omitempty"`
	SpaceThreadingState string `json:"spaceThreadingState,omitempty"`
}

type gchatThread struct {
	Name string `json:"name,omitempty"`
}

type gchatUser struct {
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Type        string `json:"type,omitempty"`
	Email       string `json:"email,omitempty"`
}

type gchatAttachment struct {
	Name        string `json:"name,omitempty"`
	ContentName string `json:"contentName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	DownloadURI string `json:"downloadUri,omitempty"`
}

type gchatAnnotation struct {
	Type        string `json:"type,omitempty"`
	StartIndex  int    `json:"startIndex,omitempty"`
	Length      int    `json:"length,omitempty"`
	UserMention *struct {
		User gchatUser `json:"user"`
		Type string    `json:"type,omitempty"`
	} `json:"userMention,omitempty"`
}

type gchatReaction struct {
	Name  string `json:"name,omitempty"`
	Emoji *struct {
		Unicode string `json:"unicode,omitempty"`
	} `json:"emoji,omitempty"`
	User *gchatUser `json:"user,omitempty"`
}

type gchatUserIdentity struct {
	ID          string
	Username    string
	DisplayName string
}

type gchatMappedInbound struct {
	Envelope bridgepkg.InboundMessageEnvelope
	Direct   bool
	User     gchatUserIdentity
}

type gchatAPI interface {
	ValidateAuth(context.Context) error
	CreateMessage(context.Context, gchatCreateMessageRequest) (*gchatSentMessage, error)
	UpdateMessage(context.Context, gchatUpdateMessageRequest) (*gchatSentMessage, error)
	DeleteMessage(context.Context, string) error
	GetMessage(context.Context, string) (*gchatMessage, error)
}

type gchatHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type validatedGChatURL string

type gchatBotClient struct {
	cfg                   resolvedInstanceConfig
	httpClient            gchatHTTPDoer
	reportResponseCleanup func(error)

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

type gchatCreateMessageRequest struct {
	SpaceName  string
	ThreadName string
	Text       string
}

type gchatUpdateMessageRequest struct {
	MessageName string
	Text        string
}

type gchatSentMessage struct {
	Name   string       `json:"name,omitempty"`
	Thread *gchatThread `json:"thread,omitempty"`
	Space  *gchatSpace  `json:"space,omitempty"`
}

type gchatTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
}

type gchatGoogleErrorEnvelope struct {
	Error struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
		Status  string `json:"status,omitempty"`
	} `json:"error"`
}

type googleDirectClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email,omitempty"`
}

type googleOIDCClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
}

type gchatThreadRef struct {
	SpaceName  string
	ThreadName string
	IsDM       bool
}
