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
	MaxAwaitDuration   = 30 * time.Minute
	MaxConcurrentAwait = 32
)

type Option func(*serviceOptions)

type serviceOptions struct {
	store     Store
	directory Directory
	claimer   ActivationClaimer
	canceler  ActivationRunCanceler
	invoker   SessionInvoker
	config    config.CallsConfig
	now       func() time.Time
	newID     func(string) (string, error)
}

type Service struct {
	store        Store
	directory    Directory
	claimer      ActivationClaimer
	canceler     ActivationRunCanceler
	invoker      SessionInvoker
	config       config.CallsConfig
	resultPolicy contracts.CallsResultsConfig
	registry     contracts.Registry
	idleTTL      time.Duration
	now          func() time.Time
	newID        func(string) (string, error)
	waitMu       sync.Mutex
	waiters      map[string]map[uint64]chan struct{}
	nextWaiterID uint64
}

func WithStore(value Store) Option { return func(opts *serviceOptions) { opts.store = value } }
func WithDirectory(value Directory) Option {
	return func(opts *serviceOptions) { opts.directory = value }
}
func WithActivationClaimer(value ActivationClaimer) Option {
	return func(opts *serviceOptions) { opts.claimer = value }
}
func WithActivationRunCanceler(value ActivationRunCanceler) Option {
	return func(opts *serviceOptions) { opts.canceler = value }
}
func WithSessionInvoker(value SessionInvoker) Option {
	return func(opts *serviceOptions) { opts.invoker = value }
}
func WithConfig(value config.CallsConfig) Option {
	return func(opts *serviceOptions) { opts.config = value }
}
func WithClock(value func() time.Time) Option {
	return func(opts *serviceOptions) { opts.now = value }
}
func WithIDGenerator(value func(string) (string, error)) Option {
	return func(opts *serviceOptions) { opts.newID = value }
}

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
	return &Service{
		store: opts.store, directory: opts.directory, claimer: opts.claimer,
		canceler: opts.canceler, invoker: opts.invoker, config: opts.config,
		resultPolicy: resultPolicy, idleTTL: idleTTL, now: opts.now, newID: opts.newID,
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

func (s *Service) scope(profileID string, scope Scope, workspaceID string) CallScope {
	return CallScope{ProfileID: strings.TrimSpace(profileID), Scope: scope, WorkspaceID: strings.TrimSpace(workspaceID)}
}
