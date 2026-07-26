package bridges

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/compozy/agh/internal/toolmeta"
)

type deliveryQueueKind string

const (
	deliveryQueueKindStart    deliveryQueueKind = "start"
	deliveryQueueKindDelta    deliveryQueueKind = "delta"
	deliveryQueueKindProgress deliveryQueueKind = "progress"
	deliveryQueueKindTerminal deliveryQueueKind = "terminal"
	deliveryQueueKindResume   deliveryQueueKind = "resume"
)

type deliveryQueueItem struct {
	deliveryID string
	kind       deliveryQueueKind
	progress   progressQueueKey
}

type routeWorker struct {
	hash             string
	bridgeInstanceID string
	extensionName    string
	queue            []deliveryQueueItem
	wakeCh           chan struct{}
	stopCh           chan struct{}
}

type turnIndexKey struct {
	sessionID string
	turnID    string
}

type activeDelivery struct {
	deliveryID       string
	sessionID        string
	turnID           string
	bridgeInstanceID string
	extensionName    string
	routingKey       RoutingKey
	target           DeliveryTarget
	routeHash        string

	latestSeq        int64
	lastSentSeq      int64
	lastAckedSeq     int64
	latestEventType  DeliveryEventType
	currentContent   MessageContent
	operation        DeliveryOperation
	reference        *DeliveryMessageReference
	providerMetadata json.RawMessage
	final            bool
	errorText        string
	updatedAt        time.Time
	progress         ProgressConfig
	progressIndex    int
	lastProgressTool string
	textStarted      bool
	toolCalls        map[string]projectedToolCall

	remoteMessageID        string
	replaceRemoteMessageID string
	startDelivered         bool

	pendingStart    *DeliveryEvent
	pendingDelta    *DeliveryEvent
	pendingTerminal *DeliveryEvent
	pendingProgress map[progressQueueKey]*DeliveryEvent
	queuedStart     bool
	queuedDelta     bool
	queuedTerminal  bool
	queuedResume    bool
	queuedProgress  map[progressQueueKey]bool

	seen map[string]struct{}
}

type instanceDeliveryMetrics struct {
	scope                 Scope
	workspaceID           string
	droppedByReason       map[string]int
	deliveryFailuresTotal int
	lastError             string
	lastErrorAt           time.Time
	lastSuccessAt         time.Time
	updatedAt             time.Time
	revision              uint64
	persistedRevision     uint64
}

type deliveryMetricPersistenceIssue struct {
	err error
	at  time.Time
}

// Broker projects session output into ordered delivery requests for one
// bridge-capable extension runtime.
type Broker struct {
	mu             sync.Mutex
	registrationMu sync.Mutex

	transport   DeliveryTransport
	descriptors toolmeta.DescriptorLookup
	ledgerStore DeliveryLedgerStore

	now            func() time.Time
	queueCapacity  int
	retryDelay     time.Duration
	requestTimeout time.Duration
	lifecycleCtx   context.Context
	cancel         context.CancelFunc

	wg sync.WaitGroup

	deliveries          map[string]*activeDelivery
	turnIndex           map[turnIndexKey]string
	sessionIndex        map[string]map[string]struct{}
	routes              map[string]*routeWorker
	bridgeRoutes        map[string]map[string]*routeWorker
	metrics             map[string]*instanceDeliveryMetrics
	metricWakeCh        chan struct{}
	metricPersistErr    error
	metricPersistIssues map[string]deliveryMetricPersistenceIssue

	registrationsReady bool
}

func newActiveDelivery(
	deliveryID string,
	routeHash string,
	registration PromptDeliveryRegistration,
	updatedAt time.Time,
) *activeDelivery {
	return &activeDelivery{
		deliveryID:       deliveryID,
		sessionID:        registration.SessionID,
		turnID:           registration.TurnID,
		bridgeInstanceID: registration.RoutingKey.BridgeInstanceID,
		extensionName:    registration.ExtensionName,
		routingKey:       registration.RoutingKey,
		target:           registration.DeliveryTarget,
		routeHash:        routeHash,
		operation:        DeliveryOperationPost,
		updatedAt:        updatedAt,
		progress:         registration.Progress,
		toolCalls:        make(map[string]projectedToolCall),
		pendingProgress:  make(map[progressQueueKey]*DeliveryEvent),
		queuedProgress:   make(map[progressQueueKey]bool),
		seen:             make(map[string]struct{}),
	}
}
