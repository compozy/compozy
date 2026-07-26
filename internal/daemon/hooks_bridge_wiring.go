package daemon

import (
	"fmt"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/toolruntime"
)

func daemonNativeHooks(
	observer sessionLifecycleObserver,
	dreamRuntime dreamCheckEnqueuer,
	memoryExtractor sessionMessagePersistedObserver,
	autoTitle sessionMessagePersistedObserver,
) ([]hookspkg.HookDecl, map[string]hookspkg.Executor) {
	decls := make([]hookspkg.HookDecl, 0, 5)
	executors := make(map[string]hookspkg.Executor, 5)

	if observer != nil {
		const (
			createName = "daemon.observe.session_post_create"
			stopName   = "daemon.observe.session_post_stop"
		)

		decls = append(decls,
			hookspkg.HookDecl{
				Name:         createName,
				Event:        hookspkg.HookSessionPostCreate,
				Mode:         hookspkg.HookModeSync,
				Priority:     1000,
				PrioritySet:  true,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
			hookspkg.HookDecl{
				Name:         stopName,
				Event:        hookspkg.HookSessionPostStop,
				Mode:         hookspkg.HookModeSync,
				Priority:     1000,
				PrioritySet:  true,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
		)
		executors[createName] = observeSessionCreateExecutor(observer)
		executors[stopName] = observeSessionStopExecutor(observer)
	}

	if dreamRuntime != nil {
		const dreamName = "daemon.dream.session_post_stop"

		decls = append(decls, hookspkg.HookDecl{
			Name:         dreamName,
			Event:        hookspkg.HookSessionPostStop,
			Mode:         hookspkg.HookModeSync,
			Priority:     900,
			PrioritySet:  true,
			ExecutorKind: hookspkg.HookExecutorNative,
		})
		executors[dreamName] = dreamSessionStopExecutor(dreamRuntime)
	}

	if memoryExtractor != nil {
		const extractorName = "daemon.memory.extractor.session_message_persisted"

		decls = append(decls, hookspkg.HookDecl{
			Name:         extractorName,
			Event:        hookspkg.HookSessionMessagePersisted,
			Mode:         hookspkg.HookModeAsync,
			Priority:     900,
			PrioritySet:  true,
			ExecutorKind: hookspkg.HookExecutorNative,
		})
		executors[extractorName] = sessionMessagePersistedExecutor(memoryExtractor)
	}

	if autoTitle != nil {
		const titleName = "daemon.session.auto_title.session_message_persisted"

		decls = append(decls, hookspkg.HookDecl{
			Name:         titleName,
			Event:        hookspkg.HookSessionMessagePersisted,
			Mode:         hookspkg.HookModeAsync,
			Priority:     950,
			PrioritySet:  true,
			ExecutorKind: hookspkg.HookExecutorNative,
		})
		executors[titleName] = sessionMessagePersistedExecutor(autoTitle)
	}

	return decls, executors
}

func daemonExecutorResolver(nativeExecutors map[string]hookspkg.Executor) hookspkg.ExecutorResolver {
	return daemonExecutorResolverWithSecrets(nativeExecutors, nil)
}

func daemonExecutorResolverWithSecrets(
	nativeExecutors map[string]hookspkg.Executor,
	secretResolver hookspkg.SecretRefResolver,
	registries ...*toolruntime.Registry,
) hookspkg.ExecutorResolver {
	var registry *toolruntime.Registry
	if len(registries) > 0 {
		registry = registries[0]
	}
	return func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
		if decl.ExecutorKind == hookspkg.HookExecutorNative {
			executor := nativeExecutors[strings.TrimSpace(decl.Name)]
			if executor == nil {
				return nil, fmt.Errorf("daemon: missing native hook executor for %q", decl.Name)
			}
			return executor, nil
		}
		return defaultDaemonExecutorResolverWithRegistry(decl, secretResolver, registry)
	}
}

func defaultDaemonExecutorResolver(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
	return defaultDaemonExecutorResolverWithRegistry(decl, nil, nil)
}

func defaultDaemonExecutorResolverWithRegistry(
	decl hookspkg.HookDecl,
	secretResolver hookspkg.SecretRefResolver,
	registry *toolruntime.Registry,
) (hookspkg.Executor, error) {
	switch decl.ExecutorKind {
	case hookspkg.HookExecutorSubprocess:
		opts := []hookspkg.SubprocessExecutorOption{
			hookspkg.WithSubprocessEnv(decl.Env),
			hookspkg.WithSubprocessSecretEnv(decl.SecretEnv, secretResolver),
		}
		if registry != nil {
			opts = append(opts, hookspkg.WithSubprocessProcessRegistry(registry))
		}
		if dir := strings.TrimSpace(decl.WorkingDir); dir != "" {
			opts = append(opts, hookspkg.WithSubprocessDir(dir))
		} else if root := strings.TrimSpace(decl.Matcher.WorkspaceRoot); root != "" {
			opts = append(opts, hookspkg.WithSubprocessDir(root))
		}
		return hookspkg.NewSubprocessExecutor(
			decl.Command,
			decl.Args,
			opts...,
		), nil
	case hookspkg.HookExecutorWASM:
		return &hookspkg.WasmExecutor{}, nil
	case hookspkg.HookExecutorNative:
		return nil, fmt.Errorf("daemon: native executor for hook %q requires an explicit binding", decl.Name)
	default:
		return nil, fmt.Errorf("daemon: unsupported executor kind %q for hook %q", decl.ExecutorKind, decl.Name)
	}
}
