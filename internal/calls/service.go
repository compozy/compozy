package calls

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/store"
)

const (
	// MaxAwaitDuration bounds one synchronous await request.
	MaxAwaitDuration = 30 * time.Minute
	// MaxConcurrentAwait bounds registered waiters for one call.
	MaxConcurrentAwait     = 32
	callRecoveryBatchLimit = 100
)

// Option configures a Service dependency or deterministic runtime value.
type Option func(*serviceOptions)

type serviceOptions struct {
	store     Store
	directory Directory
	claimer   ActivationClaimer
	canceler  ActivationRunCanceler
	invoker   SessionInvoker
	publisher PublishBridge
	hooks     HookDispatcher
	config    config.CallsConfig
	now       func() time.Time
	newID     func(string) (string, error)
}

// Service owns durable agent-call admission, lifecycle, reads, and delivery.
type Service struct {
	store           Store
	directory       Directory
	claimer         ActivationClaimer
	canceler        ActivationRunCanceler
	invoker         SessionInvoker
	publisher       PublishBridge
	hooks           HookDispatcher
	config          config.CallsConfig
	resultPolicy    contracts.CallsResultsConfig
	registry        contracts.Registry
	idleTTL         time.Duration
	messageDedup    time.Duration
	messageMaxBytes int
	now             func() time.Time
	newID           func(string) (string, error)
	waitMu          sync.Mutex
	waiters         map[string]map[uint64]chan struct{}
	nextWaiterID    uint64
	publishMu       sync.Mutex
}

// WithStore configures durable call persistence.
func WithStore(value Store) Option { return func(opts *serviceOptions) { opts.store = value } }

// WithDirectory configures target and roster resolution.
func WithDirectory(value Directory) Option {
	return func(opts *serviceOptions) { opts.directory = value }
}

// WithActivationClaimer configures durable activation claims.
func WithActivationClaimer(value ActivationClaimer) Option {
	return func(opts *serviceOptions) { opts.claimer = value }
}

// WithActivationRunCanceler configures activation cancellation.
func WithActivationRunCanceler(value ActivationRunCanceler) Option {
	return func(opts *serviceOptions) { opts.canceler = value }
}

// WithSessionInvoker configures child-session lifecycle operations.
func WithSessionInvoker(value SessionInvoker) Option {
	return func(opts *serviceOptions) { opts.invoker = value }
}

// WithPublishBridge configures one-way Network publication.
func WithPublishBridge(value PublishBridge) Option {
	return func(opts *serviceOptions) { opts.publisher = value }
}

// WithHookDispatcher configures fail-open lifecycle observation.
func WithHookDispatcher(value HookDispatcher) Option {
	return func(opts *serviceOptions) { opts.hooks = value }
}

// WithConfig configures call limits and result policies.
func WithConfig(value config.CallsConfig) Option {
	return func(opts *serviceOptions) { opts.config = value }
}

// WithClock configures the service clock.
func WithClock(value func() time.Time) Option {
	return func(opts *serviceOptions) { opts.now = value }
}

// WithIDGenerator configures durable identifier generation.
func WithIDGenerator(value func(string) (string, error)) Option {
	return func(opts *serviceOptions) { opts.newID = value }
}

// NewService constructs a call service with validated dependencies and policies.
func NewService(options ...Option) (*Service, error) {
	opts := serviceOptions{
		config: config.DefaultCallsConfig(),
		now:    func() time.Time { return time.Now().UTC() },
		newID:  prefixedID,
	}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if opts.store == nil {
		return nil, errors.New("calls: store is required")
	}
	if opts.directory == nil {
		return nil, errors.New("calls: directory is required")
	}
	if opts.now == nil {
		return nil, errors.New("calls: clock is required")
	}
	if opts.newID == nil {
		return nil, errors.New("calls: id generator is required")
	}
	if err := opts.config.Validate(); err != nil {
		return nil, fmt.Errorf("calls: validate config: %w", err)
	}
	resultPolicy, err := opts.config.Results.ContractPolicy()
	if err != nil {
		return nil, fmt.Errorf("calls: resolve result policy: %w", err)
	}
	idleTTL, err := opts.config.IdleTTLDuration()
	if err != nil {
		return nil, fmt.Errorf("calls: resolve idle ttl: %w", err)
	}
	messageDedup, err := opts.config.Messages.DedupWindowDuration()
	if err != nil {
		return nil, fmt.Errorf("calls: resolve message dedup window: %w", err)
	}
	messageMaxBytes, err := config.ParseByteSize(opts.config.Messages.MaxBytes)
	if err != nil {
		return nil, fmt.Errorf("calls: resolve message byte limit: %w", err)
	}
	return &Service{
		store: opts.store, directory: opts.directory, claimer: opts.claimer,
		canceler: opts.canceler, invoker: opts.invoker, publisher: opts.publisher, hooks: opts.hooks,
		config:       opts.config,
		resultPolicy: resultPolicy, idleTTL: idleTTL, messageDedup: messageDedup,
		messageMaxBytes: messageMaxBytes, now: opts.now, newID: opts.newID,
		registry: contracts.NewRegistry(opts.store),
		waiters:  make(map[string]map[uint64]chan struct{}),
	}, nil
}

func prefixedID(prefix string) (string, error) {
	id, err := store.NewID("")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(prefix) + "_" + id, nil
}

// NormalizeCallScope trims, infers, and validates a service ownership boundary.
func NormalizeCallScope(scope CallScope) (CallScope, error) {
	scope.ProfileID = strings.TrimSpace(scope.ProfileID)
	normalized, workspaceID, err := NormalizeReadScope(scope.Scope, scope.WorkspaceID)
	if err != nil {
		return CallScope{}, newError(CodeValidation, "invalid call scope", err)
	}
	scope.Scope = normalized
	scope.WorkspaceID = workspaceID
	return scope, nil
}
