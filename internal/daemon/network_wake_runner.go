package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	networkWakePollInterval = time.Second
	networkWakeQueueSize    = 256
	networkWakeScanLimit    = 1000
	networkWakeRetryDelay   = 100 * time.Millisecond
)

type networkWakeRunnerState string

const (
	networkWakeRunnerConstructed networkWakeRunnerState = "constructed"
	networkWakeRunnerStarting    networkWakeRunnerState = "starting"
	networkWakeRunnerReady       networkWakeRunnerState = "ready"
	networkWakeRunnerDraining    networkWakeRunnerState = "draining"
	networkWakeRunnerStopped     networkWakeRunnerState = "stopped"
)

type networkWakeTaskManager interface {
	ClaimNextRun(
		context.Context,
		taskpkg.ClaimCriteria,
		taskpkg.ActorContext,
	) (*taskpkg.ClaimResult, error)
	HeartbeatRunLease(
		context.Context,
		taskpkg.LeaseHeartbeat,
		taskpkg.ActorContext,
	) (*taskpkg.Run, error)
	ReleaseRunLease(
		context.Context,
		taskpkg.LeaseRelease,
		taskpkg.ActorContext,
	) (*taskpkg.Run, error)
	SettleNetworkWake(
		context.Context,
		taskpkg.NetworkWakeSettlement,
		taskpkg.ActorContext,
	) (*taskpkg.NetworkWakeSettlementResult, error)
}

type networkWakePrompter interface {
	Resume(ctx context.Context, sessionID string) (*session.Session, error)
	PromptNetwork(
		ctx context.Context,
		sessionID string,
		message string,
		meta ...acp.PromptNetworkMeta,
	) (<-chan acp.AgentEvent, error)
	CancelPrompt(ctx context.Context, sessionID string) error
}

type networkWakeStore interface {
	store.NetworkWakeQueueStore
	store.NetworkAcceptanceStore
}

type networkWakeRunner struct {
	tasks      networkWakeTaskManager
	prompter   networkWakePrompter
	store      networkWakeStore
	logger     *slog.Logger
	now        func() time.Time
	maxWorkers int

	notifications chan store.CommittedNetworkNotification
	mu            sync.Mutex
	state         networkWakeRunnerState
	enabled       bool
	active        map[string]context.CancelFunc
	runCancel     context.CancelFunc
	runDone       chan error
	ready         chan struct{}
	readyOnce     sync.Once
	workerWG      sync.WaitGroup
}

func newNetworkWakeRunner(
	tasks networkWakeTaskManager,
	prompter networkWakePrompter,
	wakeStore networkWakeStore,
	logger *slog.Logger,
	maxWorkers int,
) (*networkWakeRunner, error) {
	if tasks == nil {
		return nil, errors.New("daemon: network wake task manager is required")
	}
	if prompter == nil {
		return nil, errors.New("daemon: network wake prompter is required")
	}
	if wakeStore == nil {
		return nil, errors.New("daemon: network wake store is required")
	}
	if maxWorkers <= 0 {
		return nil, errors.New("daemon: network wake worker limit must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &networkWakeRunner{
		tasks:         tasks,
		prompter:      prompter,
		store:         wakeStore,
		logger:        logger,
		now:           func() time.Time { return time.Now().UTC() },
		maxWorkers:    maxWorkers,
		notifications: make(chan store.CommittedNetworkNotification, networkWakeQueueSize),
		state:         networkWakeRunnerConstructed,
		enabled:       true,
		active:        make(map[string]context.CancelFunc),
		ready:         make(chan struct{}),
	}, nil
}

// NotifyNetworkWake implements network.WakeNotifier after the acceptance commit.
func (r *networkWakeRunner) NotifyNetworkWake(notification store.CommittedNetworkNotification) {
	if r == nil {
		return
	}
	select {
	case r.notifications <- notification:
	default:
		r.logger.Warn(
			"daemon.network_wake.notification_deferred",
			"task_run_id", notification.TaskRunID,
			"recipient_session_id", notification.RecipientSessionID,
		)
	}
}
