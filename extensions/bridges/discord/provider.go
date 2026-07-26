package main

import (
	"encoding/json"

	"io"

	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerMessageCreateValue      = "MESSAGE_CREATE"
	providerMessageReactionAddValue = "MESSAGE_REACTION_ADD"
	providerChannelIDKey            = "channel_id"
	providerChannelTypeKey          = "channel_type"
	providerDiscordKey              = "discord"
	providerGuildIDKey              = "guild_id"
	providerMessageIDKey            = "message_id"
	providerParentIDKey             = "parent_id"
	providerTypeKey                 = "type"
)

const (
	discordListenAddrEnv = "AGH_BRIDGE_DISCORD_LISTEN_ADDR"
	discordAPIBaseEnv    = "AGH_BRIDGE_DISCORD_API_BASE_URL"

	discordDefaultAPIBaseURL        = "https://discord.com/api/v10"
	discordWebhookReadHeaderTimeout = 10 * time.Second
	discordWebhookIdleTimeout       = 2 * time.Minute

	discordInteractionTypePing               = 1
	discordInteractionTypeApplicationCommand = 2
	discordInteractionTypeMessageComponent   = 3

	discordInteractionResponseTypePong                   = 1
	discordInteractionResponseTypeDeferredChannelMessage = 5
	discordInteractionResponseTypeDeferredUpdateMessage  = 6
	discordWebhookEnvelopeTypePing                       = 0
	discordWebhookEnvelopeTypeEvent                      = 1
	discordChannelTypeDM                                 = 1
	discordChannelTypeGroupDM                            = 3
	discordChannelTypeAnnouncementThread                 = 10
	discordChannelTypePublicThread                       = 11
	discordChannelTypePrivateThread                      = 12
	discordApplicationCommandOptionTypeSubcommand        = 1
	discordApplicationCommandOptionTypeSubcommandGroup   = 2
)

type discordProvider struct {
	sdk        *bridgesdk.Runtime
	lifecycle  *bridgesdk.ProviderLifecycle
	markers    *bridgesdk.AdapterMarkers
	http       *bridgesdk.ProviderHTTPServer
	stderr     io.Writer
	now        func() time.Time
	routes     *bridgesdk.RouteTable[resolvedInstanceConfig]
	deliveries *bridgesdk.DeliveryStateStore[deliveryState]

	mu                        sync.RWMutex
	lastError                 string
	listenAddr                string
	apiFactory                func(resolvedInstanceConfig) discordAPI
	progressDispatcherOptions []bridgesdk.ProgressDispatcherOption
}

type discordProviderConfig struct {
	ApplicationID string `json:"application_id,omitempty"`
	Webhook       struct {
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
	managed            subprocess.InitializeBridgeManagedInstance
	instanceID         string
	listenAddr         string
	webhookPath        string
	apiBaseURL         string
	applicationID      string
	botToken           string
	publicKey          string
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

type discordPayloadProbe struct {
	Token json.RawMessage `json:"token,omitempty"`
	Event json.RawMessage `json:"event,omitempty"`
}

type discordWebhookEventEnvelope struct {
	Type  int                  `json:"type"`
	Event *discordWebhookEvent `json:"event,omitempty"`
}

type discordWebhookEvent struct {
	ID        string          `json:"id,omitempty"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type discordMessageEvent struct {
	ID          string              `json:"id"`
	ChannelID   string              `json:"channel_id"`
	GuildID     string              `json:"guild_id,omitempty"`
	ParentID    string              `json:"parent_id,omitempty"`
	ChannelType int                 `json:"channel_type,omitempty"`
	Content     string              `json:"content,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	Author      discordUser         `json:"author"`
	Attachments []discordAttachment `json:"attachments,omitempty"`
}

type discordAttachment struct {
	ID          string `json:"id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	URL         string `json:"url,omitempty"`
}

type discordReactionEvent struct {
	ID          string                    `json:"id,omitempty"`
	ChannelID   string                    `json:"channel_id"`
	GuildID     string                    `json:"guild_id,omitempty"`
	ParentID    string                    `json:"parent_id,omitempty"`
	ChannelType int                       `json:"channel_type,omitempty"`
	MessageID   string                    `json:"message_id"`
	UserID      string                    `json:"user_id"`
	Member      *discordInteractionMember `json:"member,omitempty"`
	Emoji       discordEmoji              `json:"emoji"`
	Timestamp   string                    `json:"timestamp,omitempty"`
}

type discordInteraction struct {
	ID            string                     `json:"id"`
	ApplicationID string                     `json:"application_id,omitempty"`
	Type          int                        `json:"type"`
	Token         string                     `json:"token,omitempty"`
	GuildID       string                     `json:"guild_id,omitempty"`
	ChannelID     string                     `json:"channel_id,omitempty"`
	Channel       *discordInteractionChannel `json:"channel,omitempty"`
	Data          *discordInteractionData    `json:"data,omitempty"`
	Member        *discordInteractionMember  `json:"member,omitempty"`
	User          *discordUser               `json:"user,omitempty"`
	Message       *discordInteractionMessage `json:"message,omitempty"`
}

type discordInteractionChannel struct {
	ID       string `json:"id,omitempty"`
	Type     int    `json:"type,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}

type discordInteractionMember struct {
	User *discordUser `json:"user,omitempty"`
}

type discordInteractionMessage struct {
	ID string `json:"id,omitempty"`
}

type discordInteractionData struct {
	ID            string                     `json:"id,omitempty"`
	Name          string                     `json:"name,omitempty"`
	Type          int                        `json:"type,omitempty"`
	CustomID      string                     `json:"custom_id,omitempty"`
	ComponentType int                        `json:"component_type,omitempty"`
	Values        []string                   `json:"values,omitempty"`
	Options       []discordInteractionOption `json:"options,omitempty"`
}

type discordInteractionOption struct {
	Name    string                     `json:"name,omitempty"`
	Type    int                        `json:"type,omitempty"`
	Value   any                        `json:"value,omitempty"`
	Options []discordInteractionOption `json:"options,omitempty"`
}

type discordEmoji struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type discordUser struct {
	ID         string `json:"id,omitempty"`
	Username   string `json:"username,omitempty"`
	GlobalName string `json:"global_name,omitempty"`
	Bot        bool   `json:"bot,omitempty"`
}

type discordMappedInbound struct {
	Envelope bridgepkg.InboundMessageEnvelope
	Direct   bool
	User     discordUserIdentity
}

type discordUserIdentity struct {
	ID          string
	Username    string
	DisplayName string
}
