//go:build integration

package daytona

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const launcherLatencyThreshold = 200 * time.Millisecond

func TestDaytonaLauncherTransportValidation(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv(daytonaAPIKeyEnv))
	if apiKey == "" {
		t.Skipf("%s is required for Daytona launcher transport validation", daytonaAPIKeyEnv)
	}
	if err := seedKnownHosts(t, daytonaSSHHost()); err != nil {
		t.Fatalf("seedKnownHosts() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	client := newDaytonaValidationClient(apiKey)
	sandboxID, err := client.createSandbox(ctx)
	if err != nil {
		t.Fatalf("create Daytona sandbox: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cleanupCancel()
		if cleanupErr := client.deleteSandbox(cleanupCtx, sandboxID); cleanupErr != nil {
			t.Errorf("delete Daytona sandbox %q: %v", sandboxID, cleanupErr)
		}
	})

	tokenManager := newSSHTokenManager(newRESTSSHTokenSource(time.Now), time.Now)
	transport := newSidecarTransport(nil, newSDKClient, newSSHTransport(tokenManager))
	session, err := transport.Dial(ctx, sandboxInfo{
		ID:      sandboxID,
		APIURL:  client.apiURL,
		SSHHost: daytonaSSHHost(),
	}, "cat")
	if err != nil {
		t.Fatalf("launch sidecar transport session: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cleanupCancel()
		if cleanupErr := session.Stop(cleanupCtx); cleanupErr != nil && !isNormalSessionShutdown(cleanupErr) {
			t.Errorf("stop sidecar transport session: %v", cleanupErr)
		}
	})

	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{name: "small-100B", payload: mustJSONPayload(t, 100)},
		{name: "medium-10KB", payload: mustJSONPayload(t, 10*1024)},
		{name: "large-100KB", payload: mustJSONPayload(t, 100*1024)},
		{name: "newline-delimited-json", payload: newlineDelimitedJSONPayload(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := roundTripLauncherSession(session, tc.payload)
			if err != nil {
				t.Fatalf("launcher transport round trip failed: %v", err)
			}
			assertCleanRoundTrip(t, tc.payload, result)
			t.Logf("launcher payload=%s bytes=%d latency=%s artifacts=none", tc.name, len(tc.payload), result.latency)
		})
	}

	payload := mustJSONPayload(t, 1024)
	result, err := roundTripLauncherSession(session, payload)
	if err != nil {
		t.Fatalf("launcher transport latency round trip failed: %v", err)
	}
	assertCleanRoundTrip(t, payload, result)
	if result.latency > launcherLatencyThreshold {
		t.Fatalf(
			"launcher transport 1KB round-trip latency = %s, want <= %s",
			result.latency,
			launcherLatencyThreshold,
		)
	}
	t.Logf(
		"launcher payload=latency-1KB bytes=%d latency=%s threshold=%s",
		len(payload),
		result.latency,
		launcherLatencyThreshold,
	)

	if err := session.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite() error = %v", err)
	}
	if err := session.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func roundTripLauncherSession(session transportSession, payload []byte) (sshRoundTripResult, error) {
	started := time.Now()
	writeErrCh := make(chan error, 1)
	go func() {
		writeErrCh <- writeAll(session, payload)
	}()

	output := make([]byte, len(payload))
	_, readErr := io.ReadFull(session, output)
	latency := time.Since(started)
	writeErr := <-writeErrCh
	if err := errors.Join(writeErr, readErr); err != nil {
		return sshRoundTripResult{}, fmt.Errorf("launcher round trip: %w", err)
	}
	return sshRoundTripResult{output: output, latency: latency}, nil
}

func isNormalSessionShutdown(err error) bool {
	if err == nil {
		return true
	}
	var missing *ssh.ExitMissingError
	return errors.As(err, &missing)
}
