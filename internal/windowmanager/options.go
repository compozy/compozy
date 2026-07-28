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
	workspaceConfig    WorkspaceConfigResolver
}

// Option customizes the manager without introducing operational globals.
type Option func(*managerOptions) error

// EventObserver receives an isolated copy of one observable committed event.
type EventObserver func(context.Context, Event)

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

func resolveOptions(options []Option) (managerOptions, error) {
	resolved := managerOptions{now: time.Now, generate: randomID, subscriptionBuffer: defaultSubscriptionBuffer}
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
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random ID: %w", err)
	}
	return kind + "-" + hex.EncodeToString(bytes), nil
}
