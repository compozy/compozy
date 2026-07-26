package main

import (
	"context"

	"io"

	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	telegramProviderVersion = "0.1.0"
	providerTelegramKey     = "telegram"
)

const (
	telegramListenAddrEnv = "AGH_BRIDGE_TELEGRAM_LISTEN_ADDR"
	telegramAPIBaseEnv    = "AGH_BRIDGE_TELEGRAM_API_BASE_URL"

	telegramDefaultAPIBaseURL        = "https://api.telegram.org"
	telegramGeneralTopicID           = "1"
	telegramWebhookReadHeaderTimeout = 10 * time.Second
	telegramWebhookIdleTimeout       = 30 * time.Second
)

type telegramProvider struct {
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
	apiFactory func(*resolvedInstanceConfig) telegramAPI
}

type telegramProviderConfig struct {
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
	webhookSecret      string
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

type telegramUpdate struct {
	UpdateID          int64            `json:"update_id"`
	Message           *telegramMessage `json:"message,omitempty"`
	EditedMessage     *telegramMessage `json:"edited_message,omitempty"`
	ChannelPost       *telegramMessage `json:"channel_post,omitempty"`
	EditedChannelPost *telegramMessage `json:"edited_channel_post,omitempty"`
}

type telegramMessage struct {
	MessageID       int64            `json:"message_id"`
	MessageThreadID int64            `json:"message_thread_id,omitempty"`
	Date            int64            `json:"date"`
	EditDate        int64            `json:"edit_date,omitempty"`
	Chat            telegramChat     `json:"chat"`
	From            telegramUser     `json:"from"`
	SenderChat      *telegramChat    `json:"sender_chat,omitempty"`
	Text            string           `json:"text,omitempty"`
	Caption         string           `json:"caption,omitempty"`
	ReplyToMessage  *telegramMessage `json:"reply_to_message,omitempty"`
}

type telegramChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Username string `json:"username,omitempty"`
	IsForum  bool   `json:"is_forum,omitempty"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type telegramBotIdentity struct {
	ID       int64  `json:"id"`
	Username string `json:"username,omitempty"`
}

type telegramSentMessage struct {
	MessageID int64 `json:"message_id"`
}

type telegramAPIEnvelope[T any] struct {
	OK          bool                    `json:"ok"`
	Result      T                       `json:"result"`
	Description string                  `json:"description,omitempty"`
	ErrorCode   int                     `json:"error_code,omitempty"`
	Parameters  telegramAPIErrorDetails `json:"parameters"`
}

type telegramAPIErrorDetails struct {
	RetryAfter int `json:"retry_after,omitempty"`
}

type telegramSendMessageRequest struct {
	ChatID          string `json:"chat_id"`
	Text            string `json:"text"`
	MessageThreadID int64  `json:"message_thread_id,omitempty"`
	ParseMode       string `json:"parse_mode,omitempty"`
}

type telegramEditMessageTextRequest struct {
	ChatID    string `json:"chat_id"`
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type telegramDeleteMessageRequest struct {
	ChatID    string `json:"chat_id"`
	MessageID int64  `json:"message_id"`
}

type telegramAPI interface {
	GetMe(context.Context) (*telegramBotIdentity, error)
	SendMessage(context.Context, telegramSendMessageRequest) (*telegramSentMessage, error)
	EditMessageText(context.Context, telegramEditMessageTextRequest) error
	DeleteMessage(context.Context, telegramDeleteMessageRequest) error
	SendChatAction(context.Context, telegramSendChatActionRequest) error
	SetMessageReaction(context.Context, telegramSetMessageReactionRequest) error
}
