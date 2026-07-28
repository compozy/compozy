package daytona

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gorilla/websocket"
	shellquote "github.com/kballard/go-shellquote"
)

func (t *sidecarTransport) launch(
	ctx context.Context,
	endpoint sidecarEndpoint,
	command string,
) (_ string, err error) {
	body, err := json.Marshal(sidecarLaunchRequest{Command: command})
	if err != nil {
		return "", fmt.Errorf("sandbox/daytona: marshal sidecar launch request: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, sidecarRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		endpoint.url(sidecarLaunchPath),
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("sandbox/daytona: build sidecar launch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := endpoint.httpClient
	if client == nil {
		client = t.httpClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sandbox/daytona: launch command via sidecar: %w", err)
	}
	defer mergeHTTPResponseCloseError(&err, resp, "launcher sidecar launch")
	if resp.StatusCode != http.StatusCreated {
		payload := readResponseSnippet(resp.Body)
		return "", fmt.Errorf(
			"sandbox/daytona: sidecar launch status %d: %s",
			resp.StatusCode,
			payload,
		)
	}
	var launched sidecarLaunchResponse
	if err := json.NewDecoder(resp.Body).Decode(&launched); err != nil {
		return "", fmt.Errorf("sandbox/daytona: decode sidecar launch response: %w", err)
	}
	if strings.TrimSpace(launched.ID) == "" {
		return "", errors.New("sandbox/daytona: sidecar launch response missing session id")
	}
	return strings.TrimSpace(launched.ID), nil
}

func (t *sidecarTransport) connect(
	ctx context.Context,
	endpoint sidecarEndpoint,
	sessionID string,
) (transportSession, error) {
	dialer := endpoint.wsDialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	conn, resp, err := dialer.DialContext(
		ctx,
		endpoint.wsURL(sidecarSessionStreamBasePath, sessionID, "stream"),
		nil,
	)
	if err != nil {
		if resp != nil && resp.Body != nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close failed sidecar handshake response: %w", closeErr))
			}
		}
		return nil, fmt.Errorf("sandbox/daytona: connect launcher sidecar stream: %w", err)
	}
	httpClient := endpoint.httpClient
	if httpClient == nil {
		httpClient = t.httpClient
	}
	return newSidecarSession(conn, endpoint, sessionID, httpClient, t.closeTimeout), nil
}

func (e sidecarEndpoint) url(parts ...string) string {
	clone := *e.base
	joined := append([]string{strings.TrimSuffix(clone.Path, "/")}, parts...)
	clone.Path = path.Join(joined...)
	clone.RawPath = ""
	return clone.String()
}

func (e sidecarEndpoint) wsURL(parts ...string) string {
	u := e.url(parts...)
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	}
	return parsed.String()
}

func (e sidecarEndpoint) Close() error {
	if e.closeFn == nil {
		return nil
	}
	return e.closeFn()
}

func launcherSidecarStartCommand() string {
	script := strings.Join([]string{
		"chmod 755 " + shellquote.Join(launcherSidecarPath),
		"&&",
		"nohup",
		shellquote.Join(launcherSidecarPath),
		"--port",
		fmt.Sprintf("%d", launcherSidecarPort),
		">" + shellquote.Join(launcherSidecarLogPath),
		"2>&1",
		"</dev/null",
		"&",
	}, " ")
	return strings.Join([]string{
		"sh", "-lc", shellquote.Join(script),
	}, " ")
}
