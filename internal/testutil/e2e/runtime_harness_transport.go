package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net"
	"net/http"

	"path/filepath"

	"strings"

	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"

	"golang.org/x/sys/execabs"
)

// RunInDir executes one CLI command against the isolated daemon runtime using the provided working directory.
func (c *CLIClient) RunInDir(ctx context.Context, workdir string, args ...string) (string, string, error) {
	// #nosec G204 -- test helper intentionally shells out to the current agh test binary.
	cmd := execabs.CommandContext(ctx, c.binaryPath, args...)
	cmd.Env = append([]string(nil), c.env...)
	trimmedDir := strings.TrimSpace(workdir)
	switch {
	case trimmedDir == "":
		cmd.Dir = c.workdir
	case filepath.IsAbs(trimmedDir):
		cmd.Dir = trimmedDir
	default:
		cmd.Dir = filepath.Join(c.workdir, trimmedDir)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// RunJSON executes one CLI command and decodes its JSON stdout.
func (c *CLIClient) RunJSON(ctx context.Context, dest any, args ...string) error {
	return c.RunJSONInDir(ctx, "", dest, args...)
}

// RunJSONInDir executes one CLI command in the provided working directory and decodes its JSON stdout.
func (c *CLIClient) RunJSONInDir(ctx context.Context, workdir string, dest any, args ...string) error {
	stdout, stderr, err := c.RunInDir(ctx, workdir, args...)
	if err != nil {
		return fmt.Errorf("run CLI %q: %w; stderr=%s", strings.Join(args, " "), err, strings.TrimSpace(stderr))
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal([]byte(stdout), dest); err != nil {
		return fmt.Errorf("decode CLI JSON %q: %w; stdout=%s", strings.Join(args, " "), err, strings.TrimSpace(stdout))
	}
	return nil
}

func (h *RuntimeHarness) waitForReady(ctx context.Context, pollInterval time.Duration) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastProbeErr error
	for {
		if exited, err := h.pollExit(); exited {
			return daemonExitedBeforeReadinessError(err)
		}
		err := h.probeReady(ctx)
		if err == nil {
			return nil
		}
		lastProbeErr = err
		select {
		case <-ctx.Done():
			if exited, err := h.pollExit(); exited {
				return daemonExitedBeforeReadinessError(err)
			}
			if lastProbeErr != nil {
				return fmt.Errorf("daemon did not become ready before timeout: %w", lastProbeErr)
			}
			return errors.New("daemon did not become ready before timeout")
		case err, ok := <-h.waitCh:
			h.processWaitMu.Lock()
			h.processExited = true
			if ok {
				h.processErr = err
			}
			storedErr := h.processErr
			h.processWaitMu.Unlock()
			return daemonExitedBeforeReadinessError(storedErr)
		case <-ticker.C:
		}
	}
}

func daemonExitedBeforeReadinessError(err error) error {
	if err != nil {
		return fmt.Errorf("%w: %w", errDaemonExitedBeforeReadiness, err)
	}
	return errDaemonExitedBeforeReadiness
}

func (h *RuntimeHarness) probeReady(ctx context.Context) error {
	if runtimeHarnessRequiresHTTPReadinessProbe(h.Config.HTTP.Host) {
		var httpStatus aghcontract.StatusPayload
		if err := h.HTTPJSON(ctx, http.MethodGet, "/api/status", nil, &httpStatus); err != nil {
			return err
		}
	}

	var udsStatus aghcontract.StatusPayload
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/status", nil, &udsStatus); err != nil {
		return err
	}

	var cliStatus aghcontract.StatusPayload
	if err := h.CLI.RunJSON(ctx, &cliStatus, "status", "-o", "json"); err != nil {
		return err
	}

	return nil
}

func runtimeHarnessRequiresHTTPReadinessProbe(host string) bool {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return true
	}
	if strings.EqualFold(trimmed, "localhost") {
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func (h *RuntimeHarness) waitForExit(ctx context.Context) error {
	if exited, err := h.pollExit(); exited {
		return err
	}

	select {
	case err, ok := <-h.waitCh:
		h.processWaitMu.Lock()
		defer h.processWaitMu.Unlock()
		h.processExited = true
		if ok {
			h.processErr = err
		}
		return h.processErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *RuntimeHarness) pollExit() (bool, error) {
	h.processWaitMu.Lock()
	defer h.processWaitMu.Unlock()

	if h.processExited {
		return true, h.processErr
	}

	select {
	case err, ok := <-h.waitCh:
		h.processExited = true
		if ok {
			h.processErr = err
		}
		return true, h.processErr
	default:
		return false, nil
	}
}

func doJSONRequest(
	ctx context.Context,
	client *http.Client,
	targetURL string,
	method string,
	body any,
	dest any,
) (err error) {
	response, err := doRequest(ctx, client, targetURL, method, body)
	if err != nil {
		return err
	}
	defer mergeHTTPResponseCloseError(&err, response, method, targetURL)

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read %s %s response: %w", method, targetURL, err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf(
			"%s %s status %d: %s",
			method,
			targetURL,
			response.StatusCode,
			strings.TrimSpace(string(payload)),
		)
	}
	if dest == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, dest); err != nil {
		return fmt.Errorf(
			"decode %s %s response: %w; body=%s",
			method,
			targetURL,
			err,
			strings.TrimSpace(string(payload)),
		)
	}
	return nil
}

func mergeHTTPResponseCloseError(
	target *error,
	response *http.Response,
	method string,
	targetURL string,
) {
	if target == nil || response == nil || response.Body == nil {
		return
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		*target = errors.Join(
			*target,
			fmt.Errorf("close %s %s response body: %w", method, targetURL, closeErr),
		)
	}
}

func doRequest(
	ctx context.Context,
	client *http.Client,
	targetURL string,
	method string,
	body any,
) (*http.Response, error) {
	if client == nil {
		return nil, errors.New("runtime harness http client is required")
	}
	reader, err := requestBody(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return nil, fmt.Errorf("new request %s %s: %w", method, targetURL, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform %s %s: %w", method, targetURL, err)
	}
	return response, nil
}

func requestBody(body any) (io.Reader, error) {
	switch typed := body.(type) {
	case nil:
		return nil, nil
	case []byte:
		return bytes.NewReader(typed), nil
	case string:
		return strings.NewReader(typed), nil
	default:
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		return bytes.NewReader(payload), nil
	}
}
