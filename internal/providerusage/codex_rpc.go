package providerusage

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/subprocess"
)

func collectCodexRPC(ctx context.Context, t Target) (Snapshot, error) {
	proc, err := subprocess.Launch(ctx, subprocess.LaunchConfig{
		Command:          t.Route.NativeCommand,
		Env:              t.Route.Env,
		Args:             []string{"app-server"},
		DisableTransport: true,
		ShutdownTimeout:  time.Second,
		PostSignalGrace:  100 * time.Millisecond,
		StderrTransform:  func(string) string { return "[redacted]" },
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return Snapshot{}, NewUsageError(ErrorCodeProviderUnavailable, "Codex")
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := proc.Shutdown(stopCtx); err != nil {
				slog.Debug(
					"provider_usage.process_stop",
					"provider",
					"codex",
					"error_code",
					ErrorCodeProviderUnavailable,
				)
			}
		})
	}
	canceled := context.AfterFunc(ctx, stop)
	defer canceled()
	defer stop()
	client := codexUsageRPC{writer: json.NewEncoder(proc.Stdin()), reader: bufio.NewScanner(proc.Stdout()), stop: stop}
	client.reader.Buffer(make([]byte, 4096), MaxResponseBytes)
	if err := client.call(
		"initialize",
		map[string]any{"clientInfo": map[string]string{"name": "compozy-provider-usage", "version": "1"}},
		nil,
		8*time.Second,
	); err != nil {
		return Snapshot{}, err
	}
	if err := client.writer.Encode(map[string]string{"method": "initialized"}); err != nil {
		return Snapshot{}, NewUsageError(ErrorCodeProviderUnavailable, "Codex")
	}
	var account struct {
		Account *struct {
			Email string `json:"email"`
			Plan  string `json:"planType"`
		} `json:"account"`
	}
	if err := client.call("account/read", map[string]bool{"refreshToken": false}, &account, 3*time.Second); err != nil {
		return Snapshot{}, err
	}
	var response struct {
		RateLimits codexRPCLimits `json:"rateLimits"`
	}
	if err := client.call("account/rateLimits/read", nil, &response, 3*time.Second); err != nil {
		return Snapshot{}, err
	}
	snapshot, err := decodeCodexRPC(response.RateLimits)
	if err != nil {
		return Snapshot{}, err
	}
	if account.Account != nil {
		snapshot.Account.Label = account.Account.Email
		if snapshot.Account.Plan == "" {
			snapshot.Account.Plan = account.Account.Plan
		}
	}
	return snapshot, nil
}

type codexUsageRPC struct {
	writer *json.Encoder
	reader *bufio.Scanner
	stop   func()
	id     int
}

func (c *codexUsageRPC) call(method string, params, result any, timeout time.Duration) error {
	timer := time.AfterFunc(timeout, c.stop)
	defer timer.Stop()
	c.id++
	if err := c.writer.Encode(map[string]any{"id": c.id, "method": method, "params": params}); err != nil {
		return NewUsageError(ErrorCodeProviderUnavailable, "Codex")
	}
	for c.reader.Scan() {
		var response struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(c.reader.Bytes(), &response); err != nil {
			return NewUsageError(ErrorCodeParseError, "Codex")
		}
		if response.Method != "" {
			if len(response.ID) > 0 && string(response.ID) != jsonNull {
				return NewUsageError(ErrorCodeParseError, "Codex")
			}
			continue
		}
		if strings.Trim(string(response.ID), "\"") != strconv.Itoa(c.id) {
			continue
		}
		if len(response.Error) > 0 && string(response.Error) != jsonNull {
			return NewUsageError(ErrorCodeProviderUnavailable, "Codex")
		}
		if result == nil {
			return nil
		}
		if err := json.Unmarshal(response.Result, result); err != nil {
			return NewUsageError(ErrorCodeParseError, "Codex")
		}
		return nil
	}
	return NewUsageError(ErrorCodeProviderUnavailable, "Codex")
}
