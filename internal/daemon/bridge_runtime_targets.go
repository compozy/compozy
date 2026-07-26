package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	extensionpkg "github.com/compozy/agh/internal/extension"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

func (r *bridgeRuntime) ListProviders(ctx context.Context) ([]bridgepkg.BridgeProvider, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return nil, errors.New("daemon: list bridge providers context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.registry == nil {
		return nil, nil
	}

	infos, err := r.registry.List()
	if err != nil {
		return nil, fmt.Errorf("daemon: list bridge providers: %w", err)
	}

	extensions := r.extensionRuntime()
	providers := make([]bridgepkg.BridgeProvider, 0, len(infos))
	for idx := range infos {
		provider, ok := r.bridgeProviderFromInfo(ctx, &infos[idx], extensions)
		if !ok {
			continue
		}
		providers = append(providers, *provider)
	}

	slices.SortFunc(providers, func(left, right bridgepkg.BridgeProvider) int {
		if byName := strings.Compare(left.DisplayName, right.DisplayName); byName != 0 {
			return byName
		}
		return strings.Compare(left.ExtensionName, right.ExtensionName)
	})
	return providers, nil
}

func (r *bridgeRuntime) extensionRuntime() extensionRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.extensions
}

func (r *bridgeRuntime) bridgeProviderFromInfo(
	ctx context.Context,
	info *extensionpkg.ExtensionInfo,
	extensions extensionRuntime,
) (*bridgepkg.BridgeProvider, bool) {
	if info == nil || !slices.Contains(info.Capabilities.Provides, extensionprotocol.CapabilityProvideBridgeAdapter) {
		return nil, false
	}

	ext, err := loadExtensionSnapshot(ctx, r.registry, extensions, r.logger, info.Name)
	if err != nil {
		r.logger.Warn("daemon: skip invalid bridge provider extension", "extension_name", info.Name, "error", err)
		return nil, false
	}

	provider, err := r.describeBridgeProvider(info, ext, extensions != nil)
	if err != nil {
		r.logger.Warn("daemon: skip bridge provider", "extension_name", info.Name, "error", err)
		return nil, false
	}
	return provider, true
}

func (r *bridgeRuntime) describeBridgeProvider(
	info *extensionpkg.ExtensionInfo,
	ext *extensionpkg.Extension,
	runtimeReady bool,
) (*bridgepkg.BridgeProvider, error) {
	if info == nil {
		return nil, errors.New("extension metadata is required")
	}
	if ext == nil || ext.Manifest == nil {
		return nil, errors.New("missing manifest")
	}

	platform := strings.TrimSpace(ext.Manifest.Bridge.Platform)
	if platform == "" {
		return nil, errors.New("missing platform")
	}
	displayName := strings.TrimSpace(ext.Manifest.Bridge.DisplayName)
	if displayName == "" {
		return nil, errors.New("missing display name")
	}

	status := extensionpkg.DescribeExtension(ext, runtimeReady, r.now())
	return &bridgepkg.BridgeProvider{
		Platform:      platform,
		ExtensionName: info.Name,
		DisplayName:   displayName,
		Description:   strings.TrimSpace(ext.Manifest.Description),
		SecretSlots:   normalizedBridgeSecretSlots(ext),
		ConfigSchema:  normalizedBridgeConfigSchema(ext),
		Enabled:       info.Enabled,
		State:         status.State,
		Health:        status.Health,
		HealthMessage: status.HealthMessage,
	}, nil
}

func normalizedBridgeSecretSlots(ext *extensionpkg.Extension) []bridgepkg.BridgeSecretSlot {
	if ext == nil || ext.Manifest == nil {
		return nil
	}

	secretSlots := make([]bridgepkg.BridgeSecretSlot, 0, len(ext.Manifest.Bridge.SecretSlots))
	for _, slot := range ext.Manifest.Bridge.SecretSlots {
		secretSlots = append(secretSlots, slot.Normalize())
	}
	return secretSlots
}

func normalizedBridgeConfigSchema(
	ext *extensionpkg.Extension,
) *bridgepkg.BridgeProviderConfigSchema {
	if ext == nil || ext.Manifest == nil || ext.Manifest.Bridge.ConfigSchema == nil {
		return nil
	}

	normalized := ext.Manifest.Bridge.ConfigSchema.Normalize()
	if normalized.IsZero() {
		return nil
	}
	return &normalized
}

func (r *bridgeRuntime) Close() {
	if r == nil {
		return
	}
	r.stopTargetDirectoryRefresh()
	if r.broker != nil {
		r.broker.Close()
	}
}

func (r *bridgeRuntime) startTargetDirectoryRefresh(ctx context.Context) {
	if r == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.targetRefreshMu.Lock()
	defer r.targetRefreshMu.Unlock()
	if r.targetRefreshCancel != nil {
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	r.targetRefreshCancel = cancel
	r.targetRefreshDone = done
	go r.targetDirectoryRefreshLoop(loopCtx, done)
}

func (r *bridgeRuntime) stopTargetDirectoryRefresh() {
	if r == nil {
		return
	}

	r.targetRefreshMu.Lock()
	cancel := r.targetRefreshCancel
	done := r.targetRefreshDone
	r.targetRefreshCancel = nil
	r.targetRefreshDone = nil
	r.targetRefreshMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (r *bridgeRuntime) targetDirectoryRefreshLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	r.refreshBridgeTargetDirectoryWithTimeout(ctx)

	ticker := time.NewTicker(bridgeTargetRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refreshBridgeTargetDirectoryWithTimeout(ctx)
		}
	}
}

func (r *bridgeRuntime) refreshBridgeTargetDirectoryWithTimeout(ctx context.Context) {
	if r == nil {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, bridgeTargetRefreshTimeout)
	defer cancel()
	if err := r.refreshBridgeTargetDirectory(refreshCtx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		logger := r.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("daemon: bridge target directory refresh failed", "error", err)
	}
}

func (r *bridgeRuntime) refreshBridgeTargetDirectory(ctx context.Context) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: bridge target refresh context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	transport, ok := r.extensionRuntime().(bridgepkg.TargetSnapshotTransport)
	if !ok || transport == nil {
		return bridgepkg.ErrBridgeTargetDirectoryUnavailable
	}

	instances, err := r.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("daemon: list bridge instances for target refresh: %w", err)
	}
	for _, instance := range instances {
		if !bridgeInstanceNeedsTargetRefresh(instance) {
			continue
		}
		snapshots, err := transport.BridgeTargetSnapshots(
			ctx,
			instance.ExtensionName,
			bridgepkg.BridgeTargetSnapshotRequest{BridgeInstanceID: instance.ID},
		)
		if err != nil {
			logger := r.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn(
				"daemon: bridge target snapshot failed",
				"bridge_id",
				instance.ID,
				"extension_name",
				instance.ExtensionName,
				"error",
				err,
			)
			continue
		}
		if _, err := r.RefreshBridgeTargets(ctx, instance.ID, snapshots); err != nil {
			logger := r.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("daemon: persist bridge target snapshot failed", "bridge_id", instance.ID, "error", err)
			continue
		}
	}
	return nil
}

func (r *bridgeRuntime) refreshBridgeTargetDirectoryForInstance(ctx context.Context, bridgeInstanceID string) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: bridge target refresh context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID == "" {
		return errors.New("daemon: bridge target refresh bridge instance id is required")
	}
	transport, ok := r.extensionRuntime().(bridgepkg.TargetSnapshotTransport)
	if !ok || transport == nil {
		return bridgepkg.ErrBridgeTargetDirectoryUnavailable
	}
	instance, err := r.GetInstance(ctx, trimmedID)
	if err != nil {
		return fmt.Errorf("daemon: load bridge instance %q for target refresh: %w", trimmedID, err)
	}
	if instance == nil || !bridgeInstanceNeedsTargetRefresh(*instance) {
		return nil
	}
	snapshots, err := transport.BridgeTargetSnapshots(
		ctx,
		instance.ExtensionName,
		bridgepkg.BridgeTargetSnapshotRequest{BridgeInstanceID: instance.ID},
	)
	if err != nil {
		return fmt.Errorf("daemon: bridge target snapshot for %q: %w", instance.ID, err)
	}
	if _, err := r.RefreshBridgeTargets(ctx, instance.ID, snapshots); err != nil {
		return fmt.Errorf("daemon: persist bridge target snapshot for %q: %w", instance.ID, err)
	}
	return nil
}

func bridgeInstanceNeedsTargetRefresh(instance bridgepkg.BridgeInstance) bool {
	if strings.TrimSpace(instance.ID) == "" || strings.TrimSpace(instance.ExtensionName) == "" {
		return false
	}
	if !instance.Enabled {
		return false
	}
	return instance.Status.Normalize() != bridgepkg.BridgeStatusDisabled
}

func (r *bridgeRuntime) setExtensionRuntime(runtime extensionRuntime) {
	if r == nil {
		return
	}

	r.stopTargetDirectoryRefresh()
	var transport bridgepkg.DeliveryTransport
	if runtimeTransport, ok := runtime.(bridgepkg.DeliveryTransport); ok {
		transport = runtimeTransport
	}

	r.mu.Lock()
	r.extensions = runtime
	r.mu.Unlock()

	if r.broker != nil {
		r.broker.SetTransport(transport)
	}
	if _, ok := runtime.(bridgepkg.TargetSnapshotTransport); ok {
		r.startTargetDirectoryRefresh(context.Background())
	}
}
