package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	toolspkg "github.com/compozy/agh/internal/tools"
)

var _ watchpkg.Poller = (*Manager)(nil)

// Poll resolves the watch-source kind to an active extension and calls watch/poll.
func (m *Manager) Poll(ctx context.Context, req watchpkg.PollRequest) (watchpkg.PollResponse, error) {
	kind, err := watchSourceKind(req.Spec)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	extensionName, err := m.watchSourceExtensionName(ctx, kind)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	return m.WatchPoll(ctx, extensionName, req)
}

// WatchPoll calls the negotiated watch/poll service method on one extension.
func (m *Manager) WatchPoll(
	ctx context.Context,
	extensionName string,
	req watchpkg.PollRequest,
) (watchpkg.PollResponse, error) {
	if _, err := watchSourceKind(req.Spec); err != nil {
		return watchpkg.PollResponse{}, err
	}
	process, name, err := m.extensionServiceProcess(
		ctx,
		extensionName,
		extensionprotocol.ExtensionServiceMethodWatchPoll,
	)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}

	req.Spec = cloneRawMessage(req.Spec)
	var response watchpkg.PollResponse
	if err := process.Call(ctx, string(extensionprotocol.ExtensionServiceMethodWatchPoll), req, &response); err != nil {
		return watchpkg.PollResponse{}, fmt.Errorf("extension: watch poll via %q: %w", name, err)
	}
	response.Payload = cloneRawMessage(response.Payload)
	return response, nil
}

func (m *Manager) watchSourceExtensionName(ctx context.Context, kind string) (string, error) {
	if ctx == nil {
		return "", ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if m == nil {
		return "", ErrManagerRequired
	}
	trimmedKind := strings.TrimSpace(kind)
	if trimmedKind == "" {
		return "", errors.New("extension: watch source kind is required")
	}

	m.mu.RLock()
	names := make([]string, 0, len(m.extensions))
	for name, ext := range m.extensions {
		if ext == nil || !ext.active || ext.process == nil || ext.initialize == nil {
			continue
		}
		if !slices.Contains(ext.initialize.WatchSourceKinds, trimmedKind) {
			continue
		}
		provides := ext.info.Capabilities.Provides
		if ext.manifest != nil && len(ext.manifest.Capabilities.Provides) > 0 {
			provides = ext.manifest.Capabilities.Provides
		}
		if !slices.Contains(provides, extensionprotocol.CapabilityProvideWatchSource) {
			continue
		}
		names = append(names, name)
	}
	m.mu.RUnlock()

	if len(names) == 0 {
		return "", fmt.Errorf(
			"extension: no active watch source for kind %q: %w",
			trimmedKind,
			toolspkg.ErrToolUnavailable,
		)
	}
	sort.Strings(names)
	return names[0], nil
}

func watchSourceKind(spec json.RawMessage) (string, error) {
	if len(spec) == 0 || !json.Valid(spec) {
		return "", errors.New("extension: watch source spec must be valid JSON")
	}
	var payload struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(spec, &payload); err != nil {
		return "", fmt.Errorf("extension: decode watch source spec: %w", err)
	}
	kind := strings.TrimSpace(payload.Kind)
	if kind == "" {
		return "", errors.New("extension: watch source spec.kind is required")
	}
	return kind, nil
}
