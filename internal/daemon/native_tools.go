package daemon

import (
	"errors"
	"fmt"
	"maps"

	"time"

	core "github.com/compozy/agh/internal/api/core"

	extensionpkg "github.com/compozy/agh/internal/extension"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	builtintools "github.com/compozy/agh/internal/tools/builtin"
)

const (
	daemonBlockKey          = "block"
	nativeToolsClaimedKey   = "claimed"
	nativeToolsDirectKey    = "direct"
	nativeToolsEventsKey    = "events"
	nativeToolsHealthKey    = "health"
	nativeToolsHistoryKey   = "history"
	nativeToolsLeaseKey     = "lease"
	nativeToolsLogsKey      = "logs"
	nativeToolsMessagesKey  = "messages"
	nativeToolsNetworkKey   = "network"
	nativeToolsNoteKey      = "note"
	nativeToolsProvidersKey = "providers"
	nativeToolsRedactedKey  = "redacted"
	nativeToolsRunsKey      = "runs"
	nativeToolsScopeKey     = "scope"
	nativeToolsSessionKey   = "session"
	nativeToolsSessionsKey  = "sessions"
	nativeToolsSkillsKey    = "skills"
	nativeToolsTaskKey      = "task"
	nativeToolsTextKey      = "text"
	nativeToolsWorkspaceKey = "workspace"
	nativeToolsAgentsKey    = "agents"
)

type nativeMemoryActorKind string

const (
	nativeMemoryActorKindRoot     nativeMemoryActorKind = "agent_root"
	nativeMemoryActorKindSubagent nativeMemoryActorKind = "agent_subagent"
)

func normalizeNativeMemoryActorKind(actorKind string) nativeMemoryActorKind {
	return nativeMemoryActorKind(taskpkg.ActorKind(actorKind).Normalize())
}

type daemonNativeTools struct {
	deps *daemonNativeToolsDeps
}

type memoryToolWriteRecorder interface {
	RecordToolWrite(sessionID string, turnSeq int64)
}

type nativeToolBinding struct {
	call         toolspkg.NativeToolFunc
	availability toolspkg.NativeAvailabilityFunc
}

const defaultNativeWakeEventLimit = 10

func newDaemonNativeProvider(deps *daemonNativeToolsDeps) (toolspkg.Provider, error) {
	if deps == nil {
		return nil, errors.New("daemon: native tool dependencies are required")
	}
	adapter := &daemonNativeTools{deps: deps}
	bindings := adapter.bindings()
	descriptors := builtintools.NativeDescriptors()
	nativeTools := make([]toolspkg.NativeTool, 0, len(descriptors))
	for _, descriptor := range descriptors {
		binding, ok := bindings[descriptor.ID]
		if !ok {
			return nil, fmt.Errorf("daemon: missing native handler for %s", descriptor.ID)
		}
		nativeTools = append(nativeTools, toolspkg.NativeTool{
			Descriptor:   descriptor,
			Call:         binding.call,
			Availability: binding.availability,
		})
	}
	return toolspkg.NewNativeProvider(builtintools.Source(), nativeTools...)
}

func appendToolEventSinkOption(
	options []toolspkg.RegistryOption,
	registry Registry,
	now func() time.Time,
) []toolspkg.RegistryOption {
	writer := extensionEventSummaryStore(registry)
	if writer == nil {
		return options
	}
	return append(options, toolspkg.WithToolEventSink(&daemonToolEventSink{
		writer: writer,
		now:    now,
	}))
}

func newDaemonExtensionToolProvider(state *bootState) (toolspkg.Provider, error) {
	if state == nil || state.registry == nil {
		return nil, nil
	}
	dbSource, ok := state.registry.(extensionDBSource)
	if !ok || dbSource.DB() == nil {
		return nil, nil
	}
	provider, err := extensionpkg.NewExtensionToolProvider(
		extensionpkg.NewRegistry(dbSource.DB()),
		func() extensionpkg.ExtensionToolRuntime {
			runtime := state.currentExtensionRuntime()
			if runtime == nil {
				return nil
			}
			toolRuntime, ok := runtime.(extensionpkg.ExtensionToolRuntime)
			if !ok {
				return nil
			}
			return toolRuntime
		},
	)
	if err != nil {
		return nil, err
	}
	return newDaemonScopedExtensionToolProvider(provider, state.workspaceResolver), nil
}

func (n *daemonNativeTools) bundleService() core.BundleService {
	if n == nil || n.deps == nil || n.deps.BundleService == nil {
		return nil
	}
	return n.deps.BundleService()
}

func (n *daemonNativeTools) loopService() core.LoopService {
	if n == nil || n.deps == nil || n.deps.Loops == nil {
		return nil
	}
	return n.deps.Loops()
}

func extensionRegistryDependency(registry Registry) *extensionpkg.Registry {
	if registry == nil {
		return nil
	}
	dbSource, ok := registry.(extensionDBSource)
	if !ok || dbSource.DB() == nil {
		return nil
	}
	return extensionpkg.NewRegistry(dbSource.DB())
}

func addNativeToolBindings(
	dst map[toolspkg.ToolID]nativeToolBinding,
	src map[toolspkg.ToolID]nativeToolBinding,
) {
	maps.Copy(dst, src)
}
