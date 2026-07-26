package extensionpkg

import (
	"context"
	"fmt"
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

type bridgeControlExtension struct {
	info     ExtensionInfo
	rootDir  string
	manifest *Manifest
}

func (m *Manager) admitBridgeControl(
	ctx context.Context,
	name string,
) (context.Context, bridgeControlExtension, func(), error) {
	m.mu.Lock()
	if !m.started || m.lifecycleCtx == nil {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(name, "manager is not started")
	}
	if m.stopping || m.lifecycleCtx.Err() != nil {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(name, "manager is stopping")
	}

	extension := m.extensions[name]
	if extension == nil {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(name, "is not registered")
	}
	if !extension.info.Enabled {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(name, "is disabled")
	}
	if !extension.registered {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(name, "has not completed registration")
	}
	if extension.phase != ExtensionPhaseActivate {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(
			name,
			fmt.Sprintf("is not started (phase %q)", extension.phase),
		)
	}
	if extension.manifest == nil {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(name, "has no manifest")
	}
	if !slices.Contains(
		extension.manifest.Capabilities.Provides,
		extensionprotocol.CapabilityProvideBridgeAdapter,
	) {
		m.mu.Unlock()
		return nil, bridgeControlExtension{}, nil, bridgeControlUnavailableError(name, "is not a bridge adapter")
	}

	snapshot := bridgeControlExtension{
		info:     cloneExtensionInfo(extension.info),
		rootDir:  extension.rootDir,
		manifest: cloneManifest(extension.manifest),
	}
	controlCtx, cancelControl := context.WithCancel(ctx)
	stopLifecycleCancel := context.AfterFunc(m.lifecycleCtx, cancelControl)
	m.wg.Add(1)
	m.mu.Unlock()

	release := func() {
		stopLifecycleCancel()
		cancelControl()
		m.wg.Done()
	}
	return controlCtx, snapshot, release, nil
}

func bridgeControlUnavailableError(name string, reason string) error {
	cause := fmt.Errorf(
		"%w: extension %q %s",
		bridgepkg.ErrBridgeControlTransportUnavailable,
		strings.TrimSpace(name),
		strings.TrimSpace(reason),
	)
	return redactedBridgeControlError("admit transient bridge control process", cause)
}
