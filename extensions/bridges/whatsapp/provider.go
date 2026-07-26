package main

import (
	"context"

	"errors"

	"io"
	"net/http"

	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerAudioKey    = "audio"
	providerImageKey    = "image"
	providerMessagesKey = "messages"
	providerStickerKey  = "sticker"
	providerTextKey     = "text"
	providerWhatsappKey = "whatsapp"
)

const (
	whatsappListenAddrEnv = "AGH_BRIDGE_WHATSAPP_LISTEN_ADDR"
	whatsappAPIBaseEnv    = "AGH_BRIDGE_WHATSAPP_API_BASE_URL"

	whatsappDefaultAPIBaseURL        = "https://graph.facebook.com"
	whatsappDefaultAPIVersion        = "v21.0"
	whatsappWebhookReadHeaderTimeout = 10 * time.Second
	whatsappWebhookIdleTimeout       = 30 * time.Second

	whatsappSignatureHeader         = "X-Hub-Signature-256"
	whatsappAppSecretSlot           = "app_secret"
	whatsappRecipientTypeIndividual = "individual"
	whatsappVerifyTokenSlot         = "verify_token"
)

var (
	errWhatsAppInstanceConfigUnavailable = errors.New(
		"whatsapp: delivery targeted unmanaged instance",
	)
	errWhatsAppInstanceConfigInvalid = errors.New(
		"whatsapp: bridge instance configuration invalid",
	)
)

type whatsappProvider struct {
	sdk        *bridgesdk.Runtime
	lifecycle  *bridgesdk.ProviderLifecycle
	markers    *bridgesdk.AdapterMarkers
	http       *bridgesdk.ProviderHTTPServer
	stderr     io.Writer
	now        func() time.Time
	routes     *bridgesdk.RouteTable[resolvedInstanceConfig]
	deliveries *bridgesdk.DeliveryStateStore[deliveryState]

	mu                 sync.RWMutex
	lastError          string
	lastErrorSeq       uint64
	initializeErrorSeq uint64
	instanceErrors     map[string]string
	listenAddr         string
	apiFactory         func(resolvedInstanceConfig) whatsappAPI
}

type whatsappProviderConfig struct {
	APIVersion    string `json:"api_version,omitempty"`
	PhoneNumberID string `json:"phone_number_id,omitempty"`
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
	managed            *subprocess.InitializeBridgeManagedInstance
	instanceID         string
	listenAddr         string
	webhookPath        string
	apiBaseURL         string
	apiVersion         string
	phoneNumberID      string
	accessToken        string
	appSecret          string
	verifyToken        string
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

type whatsappWebhookPayload struct {
	Object string                 `json:"object,omitempty"`
	Entry  []whatsappWebhookEntry `json:"entry,omitempty"`
}

type whatsappWebhookEntry struct {
	ID      string                  `json:"id,omitempty"`
	Changes []whatsappWebhookChange `json:"changes,omitempty"`
}

type whatsappWebhookChange struct {
	Field string               `json:"field,omitempty"`
	Value whatsappWebhookValue `json:"value"`
}

type whatsappWebhookValue struct {
	MessagingProduct string                   `json:"messaging_product,omitempty"`
	Metadata         whatsappMetadata         `json:"metadata"`
	Contacts         []whatsappContact        `json:"contacts,omitempty"`
	Messages         []whatsappInboundMessage `json:"messages,omitempty"`
	Statuses         []whatsappDeliveryStatus `json:"statuses,omitempty"`
}

type whatsappMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number,omitempty"`
	PhoneNumberID      string `json:"phone_number_id,omitempty"`
}

type whatsappContact struct {
	Profile struct {
		Name string `json:"name,omitempty"`
	} `json:"profile"`
	WaID string `json:"wa_id,omitempty"`
}

type whatsappInboundMessage struct {
	ID        string `json:"id,omitempty"`
	From      string `json:"from,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Type      string `json:"type,omitempty"`
	Context   *struct {
		From string `json:"from,omitempty"`
		ID   string `json:"id,omitempty"`
	} `json:"context,omitempty"`
	Text *struct {
		Body string `json:"body,omitempty"`
	} `json:"text,omitempty"`
	Image *struct {
		ID       string `json:"id,omitempty"`
		MIMEType string `json:"mime_type,omitempty"`
		Caption  string `json:"caption,omitempty"`
		SHA256   string `json:"sha256,omitempty"`
	} `json:"image,omitempty"`
	Document *struct {
		ID       string `json:"id,omitempty"`
		MIMEType string `json:"mime_type,omitempty"`
		Caption  string `json:"caption,omitempty"`
		Filename string `json:"filename,omitempty"`
		SHA256   string `json:"sha256,omitempty"`
	} `json:"document,omitempty"`
	Audio *struct {
		ID       string `json:"id,omitempty"`
		MIMEType string `json:"mime_type,omitempty"`
		Voice    bool   `json:"voice,omitempty"`
		SHA256   string `json:"sha256,omitempty"`
	} `json:"audio,omitempty"`
	Video *struct {
		ID       string `json:"id,omitempty"`
		MIMEType string `json:"mime_type,omitempty"`
		Caption  string `json:"caption,omitempty"`
		SHA256   string `json:"sha256,omitempty"`
	} `json:"video,omitempty"`
	Sticker *struct {
		ID       string `json:"id,omitempty"`
		MIMEType string `json:"mime_type,omitempty"`
		Animated bool   `json:"animated,omitempty"`
		SHA256   string `json:"sha256,omitempty"`
	} `json:"sticker,omitempty"`
	Location *struct {
		Latitude  float64 `json:"latitude,omitempty"`
		Longitude float64 `json:"longitude,omitempty"`
		Name      string  `json:"name,omitempty"`
		Address   string  `json:"address,omitempty"`
		URL       string  `json:"url,omitempty"`
	} `json:"location,omitempty"`
}

type whatsappDeliveryStatus struct {
	ID          string `json:"id,omitempty"`
	RecipientID string `json:"recipient_id,omitempty"`
	Status      string `json:"status,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

type whatsappPhoneNumber struct {
	ID string `json:"id,omitempty"`
}

type whatsappGraphAPIErrorEnvelope struct {
	Error whatsappGraphAPIError `json:"error"`
}

type whatsappGraphAPIError struct {
	Message      string `json:"message,omitempty"`
	Type         string `json:"type,omitempty"`
	Code         int    `json:"code,omitempty"`
	ErrorSubcode int    `json:"error_subcode,omitempty"`
}

type whatsappVerifyChallenge string

type whatsappAPI interface {
	GetPhoneNumber(context.Context, string) (*whatsappPhoneNumber, error)
	ReportRetry(context.Context, bridgesdk.RetryAttempt)
	SendTextMessage(
		context.Context,
		string,
		whatsappSendMessageRequest,
	) (*whatsappSendMessageResponse, error)
}

//nolint:funlen // Construction keeps the provider's declarative runtime wiring visible in one place.
func newWhatsAppProvider(stderr io.Writer) (*whatsappProvider, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	provider := &whatsappProvider{
		stderr:  stderr,
		markers: bridgesdk.NewAdapterMarkers(providerWhatsappKey, stderr),
		now:     func() time.Time { return time.Now().UTC() },
		routes: bridgesdk.NewRouteTable(func(config resolvedInstanceConfig) []string {
			return []string{config.webhookPath}
		}),
		deliveries:     bridgesdk.NewDeliveryStateStore[deliveryState](),
		instanceErrors: make(map[string]string),
	}
	provider.apiFactory = provider.newGraphAPI
	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: providerWhatsappKey,
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
				return
			}
			provider.mu.RLock()
			sequence := provider.initializeErrorSeq
			provider.mu.RUnlock()
			provider.clearGlobalErrorIfUnchanged(sequence)
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
		ReadHeaderTimeout: whatsappWebhookReadHeaderTimeout,
		IdleTimeout:       whatsappWebhookIdleTimeout,
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
			Name:    providerWhatsappKey,
			Version: "0.1.0",
			SDKName: "bridgesdk",
		},
		Initialize:  provider.handleInitialize,
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

func (p *whatsappProvider) serve(stdin io.Reader, stdout io.Writer) error {
	return p.lifecycle.Serve(context.Background(), p.sdk, stdin, stdout)
}
