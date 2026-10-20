package terminal

import (
	"context"
	"errors"
	"maps"
	"runtime"
	"strings"
	"sync"
)

// LaunchEnvironment contains resolved declarations and their redaction lifetime.
// Release is retained until terminal removal so retained output remains protected.
type LaunchEnvironment struct {
	Values  map[string]string
	Unset   []string
	Release func()
}

type EnvironmentProvider func(context.Context, string, Actor) (LaunchEnvironment, error)

func WithEnvironmentProvider(provider EnvironmentProvider) Option {
	return func(service *Service) error {
		if provider == nil {
			return errors.New("terminal: environment provider is required")
		}
		service.environment = provider
		return nil
	}
}

func (m *Service) prepareEnvironment(
	ctx context.Context,
	workspaceID string,
	actor Actor,
	spec *ProcSpec,
) (func(), error) {
	release := func() {}
	if m.environment != nil {
		declared, err := m.environment(ctx, workspaceID, actor)
		if err != nil {
			return nil, err
		}
		if declared.Release != nil {
			release = sync.OnceFunc(declared.Release)
		}
		spec.Env = maps.Clone(spec.Env)
		if spec.Env == nil {
			spec.Env = make(map[string]string)
		}
		for _, name := range declared.Unset {
			for key := range spec.Env {
				if key == name || (runtime.GOOS == "windows" && strings.EqualFold(key, name)) {
					delete(spec.Env, key)
				}
			}
		}
		maps.Copy(spec.Env, declared.Values)
		spec.UnsetEnv = append(spec.UnsetEnv, declared.Unset...)
	}
	spec.Env = processEnvironment(actor, spec.Env)
	return release, nil
}

func (s *session) releaseEnvironment() {
	if s.environmentRelease != nil {
		s.environmentRelease()
	}
}

func (r *execRun) releaseUnpublishedEnvironment() {
	if r.registered.Load() {
		return
	}
	if !r.lifecycleStarted {
		r.item.releaseEnvironment()
		return
	}
	go func() {
		<-r.journalFinished
		r.item.releaseEnvironment()
	}()
}
