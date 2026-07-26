package main

import (
	"context"
	"encoding/json"

	"io"
	"net/http"
	"regexp"

	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerActivityIDKey         = "activity_id"
	providerBaseConversationIDKey = "base_conversation_id"
	providerConversationIDKey     = "conversation_id"
	providerMessageKey            = "message"
	providerServiceURLKey         = "service_url"
	providerTeamsKey              = "teams"
	providerTenantIDKey           = "tenant_id"
)

const (
	teamsListenAddrEnv            = "AGH_BRIDGE_TEAMS_LISTEN_ADDR"
	teamsServiceURLEnv            = "AGH_BRIDGE_TEAMS_SERVICE_URL"
	teamsOpenIDMetadataURLEnv     = "AGH_BRIDGE_TEAMS_OPENID_METADATA_URL"
	teamsTestLoopbackAuthEnv      = "AGH_BRIDGE_TEAMS_ALLOW_LOOPBACK_AUTH_FOR_TESTING"
	teamsDefaultOpenIDMetadata    = "https://login.botframework.com/v1/.well-known/openidconfiguration"
	teamsDefaultServiceURL        = "https://smba.trafficmanager.net/teams/"
	teamsDefaultScope             = "https://api.botframework.com/.default"
	teamsWebhookReadHeaderTimeout = 10 * time.Second
	teamsWebhookIdleTimeout       = 30 * time.Second
	teamsAuthCacheTTL             = 5 * time.Minute
	rpcCodeNotInitialized         = -32003
)

var messageIDStripPattern = regexp.MustCompile(`;messageid=.+$`)

var teamsAuthHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type teamsProvider struct {
	sdk        *bridgesdk.Runtime
	lifecycle  *bridgesdk.ProviderLifecycle
	markers    *bridgesdk.AdapterMarkers
	http       *bridgesdk.ProviderHTTPServer
	stderr     io.Writer
	now        func() time.Time
	routes     *bridgesdk.RouteTable[resolvedInstanceConfig]
	deliveries *bridgesdk.DeliveryStateStore[deliveryState]

	mu             sync.RWMutex
	lastError      string
	listenAddr     string
	reportedHealth map[string]string
	userContexts   map[string]teamsUserContext
	apiFactory     func(resolvedInstanceConfig) teamsAPI
}

type teamsOpenIDMetadataCacheEntry struct {
	metadata  teamsOpenIDMetadata
	expiresAt time.Time
}

type teamsJWKSCacheEntry struct {
	jwks      teamsJWKS
	expiresAt time.Time
}

var teamsAuthCache = struct {
	mu       sync.Mutex
	metadata map[string]teamsOpenIDMetadataCacheEntry
	jwks     map[string]teamsJWKSCacheEntry
}{
	metadata: make(map[string]teamsOpenIDMetadataCacheEntry),
	jwks:     make(map[string]teamsJWKSCacheEntry),
}

type teamsProviderConfig struct {
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
	serviceURL         string
	appID              string
	appPassword        string
	appTenantID        string
	openIDMetadataURL  string
	tokenURL           string
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

type teamsUserContext struct {
	ServiceURL string
	TenantID   string
}

type teamsActivity struct {
	Type             string                 `json:"type"`
	ID               string                 `json:"id,omitempty"`
	Name             string                 `json:"name,omitempty"`
	Action           string                 `json:"action,omitempty"`
	Text             string                 `json:"text,omitempty"`
	TextFormat       string                 `json:"textFormat,omitempty"`
	Timestamp        string                 `json:"timestamp,omitempty"`
	ServiceURL       string                 `json:"serviceUrl,omitempty"`
	ChannelID        string                 `json:"channelId,omitempty"`
	ReplyToID        string                 `json:"replyToId,omitempty"`
	From             teamsChannelAccount    `json:"from"`
	Recipient        teamsChannelAccount    `json:"recipient"`
	Conversation     teamsConversation      `json:"conversation"`
	Attachments      []teamsAttachment      `json:"attachments,omitempty"`
	Entities         []teamsEntity          `json:"entities,omitempty"`
	ChannelData      teamsChannelData       `json:"channelData"`
	Value            json.RawMessage        `json:"value,omitempty"`
	ReactionsAdded   []teamsMessageReaction `json:"reactionsAdded,omitempty"`
	ReactionsRemoved []teamsMessageReaction `json:"reactionsRemoved,omitempty"`
}

type teamsChannelAccount struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	AADObjectID string `json:"aadObjectId,omitempty"`
}

type teamsConversation struct {
	ID               string `json:"id,omitempty"`
	Name             string `json:"name,omitempty"`
	TenantID         string `json:"tenantId,omitempty"`
	ConversationType string `json:"conversationType,omitempty"`
	IsGroup          bool   `json:"isGroup,omitempty"`
}

type teamsAttachment struct {
	ContentType string          `json:"contentType,omitempty"`
	ContentURL  string          `json:"contentUrl,omitempty"`
	Name        string          `json:"name,omitempty"`
	Content     json.RawMessage `json:"content,omitempty"`
}

type teamsEntity struct {
	Type      string               `json:"type,omitempty"`
	Text      string               `json:"text,omitempty"`
	Mentioned *teamsChannelAccount `json:"mentioned,omitempty"`
}

type teamsChannelData struct {
	Tenant *struct {
		ID string `json:"id,omitempty"`
	} `json:"tenant,omitempty"`
	Channel *struct {
		ID string `json:"id,omitempty"`
	} `json:"channel,omitempty"`
	Team *struct {
		ID         string `json:"id,omitempty"`
		AADGroupID string `json:"aadGroupId,omitempty"`
	} `json:"team,omitempty"`
	EventType string `json:"eventType,omitempty"`
}

type teamsMessageReaction struct {
	Type string `json:"type,omitempty"`
}

type teamsActionValue struct {
	Action *struct {
		Data json.RawMessage `json:"data,omitempty"`
	} `json:"action,omitempty"`
}

type teamsActionPayload struct {
	ActionID string `json:"actionId,omitempty"`
	Value    string `json:"value,omitempty"`
}

type teamsThreadRef struct {
	ConversationID string
	ServiceURL     string
}

type teamsResolvedTarget struct {
	ConversationID string
	ServiceURL     string
	UserID         string
	TenantID       string
	ReplyToID      string
}

type teamsAPI interface {
	ValidateAuth(context.Context) error
	CreateConversation(
		context.Context,
		string,
		teamsCreateConversationRequest,
	) (*teamsConversationResourceResponse, error)
	SendActivity(
		context.Context,
		string,
		string,
		string,
		teamsOutboundActivity,
	) (*teamsResourceResponse, error)
	UpdateActivity(context.Context, string, string, string, teamsOutboundActivity) error
	DeleteActivity(context.Context, string, string, string) error
}

type teamsBotClient struct {
	cfg                   resolvedInstanceConfig
	httpClient            *http.Client
	reportResponseCleanup func(error)

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

type teamsOpenIDMetadata struct {
	Issuer  string `json:"issuer,omitempty"`
	JWKSURI string `json:"jwks_uri,omitempty"`
}

type teamsJWKS struct {
	Keys []teamsJWK `json:"keys"`
}

type teamsJWK struct {
	Kid          string   `json:"kid,omitempty"`
	X5T          string   `json:"x5t,omitempty"`
	Kty          string   `json:"kty,omitempty"`
	N            string   `json:"n,omitempty"`
	E            string   `json:"e,omitempty"`
	Endorsements []string `json:"endorsements,omitempty"`
}

type teamsAuthClaims struct {
	ServiceURL string `json:"serviceUrl,omitempty"`
	jwt.RegisteredClaims
}

type teamsTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
}

type teamsCreateConversationRequest struct {
	Bot         teamsChannelAccount   `json:"bot"`
	Members     []teamsChannelAccount `json:"members"`
	IsGroup     bool                  `json:"isGroup"`
	TenantID    string                `json:"tenantId,omitempty"`
	ChannelData map[string]any        `json:"channelData,omitempty"`
}

type teamsConversationResourceResponse struct {
	ID string `json:"id,omitempty"`
}

type teamsOutboundActivity struct {
	Type       string              `json:"type"`
	Text       string              `json:"text,omitempty"`
	TextFormat string              `json:"textFormat,omitempty"`
	From       teamsChannelAccount `json:"from"`
	Recipient  teamsChannelAccount `json:"recipient"`
}

type teamsResourceResponse struct {
	ID string `json:"id,omitempty"`
}
