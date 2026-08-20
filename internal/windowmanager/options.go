package windowmanager

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const defaultSubscriptionBuffer = 256

type managerOptions struct {
	now                func() time.Time
	generate           idGenerator
	subscriptionBuffer int
	eventObserver      EventObserver
	clientObserver     ClientUnregisteredObserver
	workspaceConfig    WorkspaceConfigResolver
	lifecycleContext   context.Context
}

// Option customizes the manager without introducing operational globals.
type Option func(*managerOptions) error

// EventObserver receives an isolated copy of one observable committed event.
type EventObserver func(context.Context, Event)

// ClientUnregisteredObserver tears down transient state owned by a detached client.
type ClientUnregisteredObserver func(context.Context, WorkspaceID, ClientID) error

// WorkspaceConfigResolver overlays workspace-scoped behavior onto active defaults.
type WorkspaceConfigResolver interface {
	ResolveWindowManagerConfig(context.Context, WorkspaceID, Config) (Config, error)
}

// WithClock injects deterministic commit and client timestamps.
func WithClock(now func() time.Time) Option {
	return func(options *managerOptions) error {
		if now == nil {
			return errors.New("window manager clock is required")
		}
		options.now = now
		return nil
	}
}

// WithIDGenerator injects deterministic entity identifiers.
func WithIDGenerator(generate func(string) (string, error)) Option {
	return func(options *managerOptions) error {
		if generate == nil {
			return errors.New("window manager ID generator is required")
		}
		options.generate = generate
		return nil
	}
}

// WithSubscriptionBuffer configures bounded subscriber backpressure.
func WithSubscriptionBuffer(size int) Option {
	return func(options *managerOptions) error {
		if size <= 0 {
			return errors.New("window manager subscription buffer must be positive")
		}
		options.subscriptionBuffer = size
		return nil
	}
}

// WithEventObserver observes lifecycle events after their durable commit succeeds.
func WithEventObserver(observer EventObserver) Option {
	return func(options *managerOptions) error {
		if observer == nil {
			return errors.New("window manager event observer is required")
		}
		options.eventObserver = observer
		return nil
	}
}

// WithClientUnregisteredObserver observes a client after its local state is removed.
func WithClientUnregisteredObserver(observer ClientUnregisteredObserver) Option {
	return func(options *managerOptions) error {
		if observer == nil {
			return errors.New("window manager client unregister observer is required")
		}
		options.clientObserver = observer
		return nil
	}
}

// WithWorkspaceConfigResolver resolves workspace-scoped runtime behavior.
func WithWorkspaceConfigResolver(resolver WorkspaceConfigResolver) Option {
	return func(options *managerOptions) error {
		if resolver == nil {
			return errors.New("window manager workspace config resolver is required")
		}
		options.workspaceConfig = resolver
		return nil
	}
}

// WithLifecycleContext preserves boot-scoped values for detached background work.
func WithLifecycleContext(ctx context.Context) Option {
	return func(options *managerOptions) error {
		if ctx == nil {
			return errors.New("window manager lifecycle context is required")
		}
		options.lifecycleContext = context.WithoutCancel(ctx)
		return nil
	}
}

func resolveOptions(options []Option) (managerOptions, error) {
	resolved := managerOptions{
		now:                time.Now,
		generate:           randomID,
		subscriptionBuffer: defaultSubscriptionBuffer,
		lifecycleContext:   context.Background(),
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&resolved); err != nil {
			return managerOptions{}, err
		}
	}
	return resolved, nil
}

func randomID(kind string) (string, error) {
	prefix, randomBytes := idShape(kind)
	bytes := make([]byte, randomBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random ID: %w", err)
	}
	return prefix + hex.EncodeToString(bytes), nil
}

func idShape(kind string) (string, int) {
	if kind == "window" {
		return "w-", 13
	}
	return kind + "-", 16
}
