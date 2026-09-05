package daytona

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/sandbox"
)

type sidecarProcessStatus struct {
	ID           string `json:"id"`
	Exited       bool   `json:"exited"`
	ExitVerified bool   `json:"exitVerified"`
	ExitCode     *int   `json:"exitCode"`
}

func (t *sidecarTransport) processExitVerified(ctx context.Context, info sandboxInfo) (_ bool, err error) {
	endpoint, err := t.openTunnel(ctx, info)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, endpoint.Close()) }()
	return t.processExitAtEndpoint(ctx, endpoint, info.LauncherProcessID)
}

func (t *sidecarTransport) processExitAtEndpoint(
	ctx context.Context, endpoint sidecarEndpoint, id string,
) (_ bool, err error) {
	requestCtx, cancel := context.WithTimeout(ctx, sidecarRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet,
		endpoint.url(sidecarSessionStreamBasePath, id), http.NoBody)
	if err != nil {
		return false, err
	}
	client := endpoint.httpClient
	if client == nil {
		client = t.httpClient
	}
	response, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer mergeHTTPResponseCloseError(&err, response, "recovered process status")
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf(
			"sandbox/daytona: process status %d; remote exit remains unverified",
			response.StatusCode,
		)
	}
	var status sidecarProcessStatus
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		return false, fmt.Errorf("sandbox/daytona: decode recovered process status: %w", err)
	}
	if status.ID != id {
		return false, errors.New("sandbox/daytona: recovered process status identity mismatch")
	}
	return status.Exited && status.ExitVerified && status.ExitCode != nil, nil
}

func (t *sidecarTransport) signalProcess(
	ctx context.Context,
	info sandboxInfo,
	signal sandbox.ProcessSignal,
) (err error) {
	endpoint, err := t.openTunnel(ctx, info)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, endpoint.Close()) }()
	requestCtx, cancel := context.WithTimeout(ctx, sidecarRequestTimeout)
	defer cancel()
	body, err := json.Marshal(struct {
		Signal sandbox.ProcessSignal `json:"signal"`
	}{Signal: signal})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost,
		endpoint.url(sidecarSessionStreamBasePath, info.LauncherProcessID, "signal"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := endpoint.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer mergeHTTPResponseCloseError(&err, response, "recovered process signal")
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("sandbox/daytona: recovered process signal status %d", response.StatusCode)
	}
	return nil
}
