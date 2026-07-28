package daytona

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"

	"strings"

	"time"
)

func (t *sidecarTransport) sidecarBinary(ctx context.Context, sandbox sandboxInfo) ([]byte, error) {
	arch, err := t.remoteArch(ctx, sandbox)
	if err != nil {
		return nil, err
	}

	t.binaryMu.Lock()
	cached := append([]byte(nil), t.binaries[arch]...)
	t.binaryMu.Unlock()
	if len(cached) != 0 {
		return cached, nil
	}

	built, err := embeddedLauncherSidecarBinary(arch)
	if err != nil {
		return nil, err
	}
	t.binaryMu.Lock()
	t.binaries[arch] = append([]byte(nil), built...)
	t.binaryMu.Unlock()
	return built, nil
}

func (t *sidecarTransport) remoteArch(ctx context.Context, sandbox sandboxInfo) (string, error) {
	if t.bootstrap == nil {
		return "", errors.New("sandbox/daytona: launcher sidecar bootstrap transport is required")
	}
	session, err := t.bootstrap.Dial(ctx, sandbox, "uname -m")
	if err != nil {
		return "", fmt.Errorf("sandbox/daytona: detect sandbox architecture: %w", err)
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			t.logger.Warn("sandbox/daytona: close arch probe session failed", "error", closeErr)
		}
	}()
	output, readErr := io.ReadAll(session)
	waitErr := session.Wait()
	if err := errors.Join(readErr, waitErr); err != nil {
		return "", fmt.Errorf("sandbox/daytona: detect sandbox architecture: %w stderr=%q", err, session.Stderr())
	}
	switch strings.TrimSpace(string(output)) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf(
			"sandbox/daytona: unsupported sandbox architecture %q",
			strings.TrimSpace(string(output)),
		)
	}
}

func (t *sidecarTransport) loadSandbox(ctx context.Context, info sandboxInfo) (daytonaSandbox, error) {
	if t.newClient == nil {
		return nil, errors.New("sandbox/daytona: launcher sidecar sandbox client is required")
	}
	client, err := t.newClient(clientConfig{APIURL: info.APIURL})
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: launcher sidecar create client: %w", err)
	}
	sandbox, err := client.Get(ctx, info.ID)
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: launcher sidecar load sandbox %q: %w", info.ID, err)
	}
	return sandbox, nil
}

func (t *sidecarTransport) health(ctx context.Context, endpoint sidecarEndpoint) (_ bool, err error) {
	requestCtx, cancel := context.WithTimeout(ctx, sidecarRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		endpoint.url(sidecarHealthPath),
		http.NoBody,
	)
	if err != nil {
		return false, fmt.Errorf("sandbox/daytona: build sidecar health request: %w", err)
	}
	client := endpoint.httpClient
	if client == nil {
		client = t.httpClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer mergeHTTPResponseCloseError(&err, resp, "launcher sidecar health")
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("sandbox/daytona: launcher sidecar health status %d", resp.StatusCode)
	}
	var payload sidecarHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("sandbox/daytona: decode launcher sidecar health: %w", err)
	}
	return payload.OK && payload.Version == launcherSidecarVersion, nil
}

func (t *sidecarTransport) waitForHealth(ctx context.Context, endpoint sidecarEndpoint) error {
	waitCtx, cancel := context.WithTimeout(ctx, t.healthTimeout)
	defer cancel()

	ticker := time.NewTicker(t.healthPollInterval)
	defer ticker.Stop()

	for {
		healthy, err := t.health(waitCtx, endpoint)
		if err == nil && healthy {
			return nil
		}
		select {
		case <-waitCtx.Done():
			if err != nil {
				return fmt.Errorf("sandbox/daytona: wait for launcher sidecar health: %w", err)
			}
			return fmt.Errorf(
				"sandbox/daytona: wait for launcher sidecar health: %w",
				waitCtx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func (t *sidecarTransport) startSidecar(ctx context.Context, sandbox sandboxInfo) error {
	if t.bootstrap == nil {
		return errors.New("sandbox/daytona: launcher sidecar bootstrap transport is required")
	}
	session, err := t.bootstrap.Dial(ctx, sandbox, launcherSidecarStartCommand())
	if err != nil {
		return fmt.Errorf("sandbox/daytona: start launcher sidecar: %w", err)
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			t.logger.Warn("sandbox/daytona: close launcher bootstrap session failed", "error", closeErr)
		}
	}()
	if err := session.Wait(); err != nil {
		stderr := strings.TrimSpace(session.Stderr())
		if stderr != "" {
			return fmt.Errorf("sandbox/daytona: start launcher sidecar: %w stderr=%q", err, stderr)
		}
		return fmt.Errorf("sandbox/daytona: start launcher sidecar: %w", err)
	}
	return nil
}
