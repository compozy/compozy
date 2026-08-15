package acp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestTokenUsageParsing(t *testing.T) {
	t.Parallel()

	inputTokens := int64(10)
	outputTokens := int64(12)
	totalTokens := int64(22)
	thoughtTokens := int64(3)
	cacheReadTokens := int64(4)
	cacheWriteTokens := int64(5)
	used := int64(80)
	size := int64(100)
	amount := 1.25
	currency := "USD"

	promptUsage := tokenUsageFromPromptResponse("turn-1", &wireUsage{
		InputTokens:      &inputTokens,
		OutputTokens:     &outputTokens,
		TotalTokens:      &totalTokens,
		ThoughtTokens:    &thoughtTokens,
		CacheReadTokens:  &cacheReadTokens,
		CacheWriteTokens: &cacheWriteTokens,
	})
	if promptUsage.InputTokens == nil || *promptUsage.InputTokens != inputTokens {
		t.Fatalf("tokenUsageFromPromptResponse() input_tokens = %#v, want %d", promptUsage.InputTokens, inputTokens)
	}
	if promptUsage.CacheWriteTokens == nil || *promptUsage.CacheWriteTokens != cacheWriteTokens {
		t.Fatalf(
			"tokenUsageFromPromptResponse() cache_write_tokens = %#v, want %d",
			promptUsage.CacheWriteTokens,
			cacheWriteTokens,
		)
	}

	merged := promptUsage.Merge(tokenUsageFromUsageUpdate("turn-1", wireUsageUpdate{
		Used: &used,
		Size: &size,
		Cost: &wireCost{
			Amount:   &amount,
			Currency: &currency,
		},
	}))
	if merged.ContextUsed == nil || *merged.ContextUsed != used {
		t.Fatalf("merged.ContextUsed = %#v, want %d", merged.ContextUsed, used)
	}
	if merged.CostCurrency == nil || *merged.CostCurrency != currency {
		t.Fatalf("merged.CostCurrency = %#v, want %q", merged.CostCurrency, currency)
	}

	empty := tokenUsageFromPromptResponse("turn-2", nil)
	if !empty.IsZero() {
		t.Fatalf("tokenUsageFromPromptResponse(nil) should be zero, got %#v", empty)
	}
}

func TestPromptPrependsSystemPromptOnce(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := startHelperProcess(t, driver, "echo_prompt", "", StartOpts{
		SystemPrompt: "Memory context first.\nThen agent prompt.",
	})
	defer stopProcess(t, driver, proc)

	firstEventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-1",
		Message: "first request",
	})
	if err != nil {
		t.Fatalf("Prompt(first) error = %v", err)
	}
	firstEvents := collectEvents(t, firstEventsCh)
	if len(firstEvents) == 0 {
		t.Fatal("Prompt(first) returned no events")
	}
	if !strings.Contains(firstEvents[0].Text, "Session instructions") {
		t.Fatalf("first prompt text = %q, want injected system prompt prefix", firstEvents[0].Text)
	}
	if !strings.Contains(firstEvents[0].Text, "Memory context first.\nThen agent prompt.") {
		t.Fatalf("first prompt text = %q, want system prompt content", firstEvents[0].Text)
	}
	if !strings.Contains(firstEvents[0].Text, "User request:\n\nfirst request") {
		t.Fatalf("first prompt text = %q, want user request content", firstEvents[0].Text)
	}

	secondEventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-2",
		Message: "second request",
	})
	if err != nil {
		t.Fatalf("Prompt(second) error = %v", err)
	}
	secondEvents := collectEvents(t, secondEventsCh)
	if len(secondEvents) == 0 {
		t.Fatal("Prompt(second) returned no events")
	}
	if secondEvents[0].Text != "second request" {
		t.Fatalf("second prompt text = %q, want plain user request", secondEvents[0].Text)
	}
}

func TestPromptAttachesSystemPromptDeliveryMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should annotate first-turn system prompt fallback", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "echo_prompt_meta", "", StartOpts{
			SystemPrompt: "Compozy runtime envelope.",
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-system-meta",
			Message: "first request",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}

		var payload PromptMeta
		if err := json.Unmarshal([]byte(events[0].Text), &payload); err != nil {
			t.Fatalf("json.Unmarshal(prompt meta echo) error = %v", err)
		}
		if payload.System == nil {
			t.Fatal("payload.System = nil, want system prompt delivery metadata")
		}
		if got, want := payload.System.PromptDelivery, string(SystemPromptDeliveryFirstTurnPrefix); got != want {
			t.Fatalf("payload.System.PromptDelivery = %q, want %q", got, want)
		}
	})

	t.Run("Should annotate native system prompt delivery without fallback prefix", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "echo_prompt_meta", "", StartOpts{
			SystemPrompt:         "Compozy runtime envelope.",
			SystemPromptDelivery: SystemPromptDeliveryNative,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-system-native",
			Message: "first request",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}

		var payload PromptMeta
		if err := json.Unmarshal([]byte(events[0].Text), &payload); err != nil {
			t.Fatalf("json.Unmarshal(prompt meta echo) error = %v", err)
		}
		if payload.System == nil {
			t.Fatal("payload.System = nil, want system prompt delivery metadata")
		}
		if got, want := payload.System.PromptDelivery, string(SystemPromptDeliveryNative); got != want {
			t.Fatalf("payload.System.PromptDelivery = %q, want %q", got, want)
		}
	})
}

func TestPromptCacheControlForStartOpts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		opts    StartOpts
		want    bool
		wantTTL string
	}{
		{
			name: "Should skip unsupported provider",
			opts: StartOpts{ProviderName: "codex"},
		},
		{
			name: "Should enable short-lived cache control for Claude provider",
			opts: StartOpts{ProviderName: "claude"},
			want: true,
		},
		{
			name: "Should enable long-lived cache control for Anthropic endpoint",
			opts: StartOpts{
				ProviderName: "pi",
				ProviderConfig: &compozyconfig.ProviderConfig{
					RuntimeProvider: "anthropic",
					BaseURL:         "https://api.anthropic.com/v1",
				},
			},
			want:    true,
			wantTTL: "1h",
		},
		{
			name: "Should enable long-lived cache control for Vertex Anthropic endpoint",
			opts: StartOpts{
				ProviderName: "pi",
				ProviderConfig: &compozyconfig.ProviderConfig{
					RuntimeProvider: "anthropic",
					BaseURL:         "https://us-east5-aiplatform.googleapis.com/v1",
				},
			},
			want:    true,
			wantTTL: "1h",
		},
		{
			name: "Should skip OpenRouter even when a base URL is present",
			opts: StartOpts{
				ProviderName: "openrouter",
				ProviderConfig: &compozyconfig.ProviderConfig{
					RuntimeProvider: "openrouter",
					BaseURL:         "https://openrouter.ai/api/v1",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := promptCacheControlForStartOpts(tc.opts)
			if !tc.want {
				if got != nil {
					t.Fatalf("promptCacheControlForStartOpts() = %#v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("promptCacheControlForStartOpts() = nil, want cache control")
			}
			if got.Type != "ephemeral" {
				t.Fatalf("cache control type = %q, want ephemeral", got.Type)
			}
			if got.TTL != tc.wantTTL {
				t.Fatalf("cache control TTL = %q, want %q", got.TTL, tc.wantTTL)
			}
		})
	}
}

func TestBuildWirePromptRequestAttachesPromptCacheControlMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should annotate text content without changing prompt text", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{
			SessionID: "sess-cache",
			promptCacheControl: &promptCacheControl{
				Type: "ephemeral",
				TTL:  "1h",
			},
		}
		request, err := buildWirePromptRequest(proc, PromptRequest{
			TurnID:  "turn-cache",
			Message: "hello cache",
		})
		if err != nil {
			t.Fatalf("buildWirePromptRequest() error = %v", err)
		}
		if got, want := len(request.Prompt), 1; got != want {
			t.Fatalf("len(Prompt) = %d, want %d", got, want)
		}
		text := request.Prompt[0].Text
		if text == nil {
			t.Fatal("Prompt[0].Text = nil, want text block")
		}
		if got, want := text.Text, "hello cache"; got != want {
			t.Fatalf("Prompt[0].Text.Text = %q, want %q", got, want)
		}
		cacheControl, ok := text.Meta["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("Prompt[0].Text.Meta = %#v, want cache_control map", text.Meta)
		}
		if got, want := cacheControl["type"], "ephemeral"; got != want {
			t.Fatalf("cache_control.type = %#v, want %q", got, want)
		}
		if got, want := cacheControl["ttl"], "1h"; got != want {
			t.Fatalf("cache_control.ttl = %#v, want %q", got, want)
		}
	})

	t.Run("Should leave text content metadata empty when provider is unsupported", func(t *testing.T) {
		t.Parallel()

		request, err := buildWirePromptRequest(&AgentProcess{SessionID: "sess-cache"}, PromptRequest{
			TurnID:  "turn-cache",
			Message: "hello cache",
		})
		if err != nil {
			t.Fatalf("buildWirePromptRequest() error = %v", err)
		}
		if request.Prompt[0].Text == nil {
			t.Fatal("Prompt[0].Text = nil, want text block")
		}
		if len(request.Prompt[0].Text.Meta) != 0 {
			t.Fatalf("Prompt[0].Text.Meta = %#v, want empty metadata", request.Prompt[0].Text.Meta)
		}
	})
}

func TestBuildWirePromptRequestAppendsAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should append attachment blocks after text", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{SessionID: "sess-attachments"}
		proc.setCaps(Caps{PromptImage: true})
		request, err := buildWirePromptRequest(proc, PromptRequest{
			TurnID:  "turn-attachments",
			Message: "inspect this",
			Attachments: []PromptAttachment{{
				Name: "diagram.png", MIMEType: "image/png", Data: []byte("image"),
			}},
		})
		if err != nil {
			t.Fatalf("buildWirePromptRequest() error = %v", err)
		}
		if len(request.Prompt) != 2 || request.Prompt[0].Text == nil || request.Prompt[1].Image == nil {
			t.Fatalf("Prompt = %#v, want text followed by image", request.Prompt)
		}
	})

	t.Run("Should omit the text block for an attachment-only prompt", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{SessionID: "sess-attachment-only"}
		proc.setCaps(Caps{PromptImage: true})
		request, err := buildWirePromptRequest(proc, PromptRequest{
			TurnID: "turn-attachment-only",
			Attachments: []PromptAttachment{{
				Name: "diagram.png", MIMEType: "image/png", Data: []byte("image"),
			}},
		})
		if err != nil {
			t.Fatalf("buildWirePromptRequest() error = %v", err)
		}
		if len(request.Prompt) != 1 || request.Prompt[0].Image == nil {
			t.Fatalf("Prompt = %#v, want one image block", request.Prompt)
		}
	})
}

func TestPromptSkipsFirstTurnPrefixForNativeSystemPromptDelivery(t *testing.T) {
	t.Parallel()

	t.Run("Should send plain user request when system prompt is native", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "echo_prompt", "", StartOpts{
			SystemPrompt:         "Compozy runtime envelope.",
			SystemPromptDelivery: SystemPromptDeliveryNative,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-native-text",
			Message: "first request",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}
		if got, want := events[0].Text, "first request"; got != want {
			t.Fatalf("first prompt text = %q, want %q", got, want)
		}
		if strings.Contains(events[0].Text, "Compozy runtime envelope.") ||
			strings.Contains(events[0].Text, "Session instructions") {
			t.Fatalf("first prompt text = %q, want no fallback prefix", events[0].Text)
		}
	})
}

func TestPromptActivityReporterReportsWhilePromptIsInFlight(t *testing.T) {
	t.Run("ShouldReportWhilePromptIsInFlight", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()

		reports := make(chan PromptActivityReport, 4)
		stop := startPromptActivityReporter(ctx, PromptRequest{
			TurnID:                    "turn-reporter",
			Message:                   "hello",
			ActivityHeartbeatInterval: 5 * time.Millisecond,
			ActivityReporter: func(report PromptActivityReport) {
				select {
				case reports <- report:
				default:
				}
			},
		})
		defer stop()

		first := readPromptActivityReport(t, reports)
		if got, want := first.Kind, "agent_waiting"; got != want {
			t.Fatalf("first report kind = %q, want %q", got, want)
		}
		if first.Timestamp.IsZero() {
			t.Fatal("first report timestamp is zero")
		}

		second := readPromptActivityReport(t, reports)
		if got, want := second.Kind, "agent_waiting"; got != want {
			t.Fatalf("second report kind = %q, want %q", got, want)
		}
		if second.Timestamp.Before(first.Timestamp) {
			t.Fatalf("second report timestamp %s before first %s", second.Timestamp, first.Timestamp)
		}
	})
}

func TestPromptTransmitsStructuredMetadata(t *testing.T) {
	t.Parallel()

	driver := New()
	proc := startHelperProcess(t, driver, "echo_prompt_meta", "", StartOpts{})
	defer stopProcess(t, driver, proc)

	eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
		TurnID:  "turn-meta",
		Message: "network delivery",
		Meta: PromptMeta{
			TurnSource: PromptTurnSourceNetwork,
			Network: &PromptNetworkMeta{
				MessageID:   "msg-meta-1",
				Kind:        "say",
				Channel:     "builders",
				Surface:     "direct",
				DirectID:    "direct_meta_1",
				From:        "ops.peer",
				To:          "worker.peer",
				WorkID:      "work-meta-1",
				ReplyTo:     "msg-root-1",
				TraceID:     "trace-meta-1",
				CausationID: "msg-root-1",
				Trust:       "untrusted",
			},
		},
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	events := collectEvents(t, eventsCh)
	if len(events) == 0 {
		t.Fatal("Prompt() returned no events")
	}

	var payload PromptMeta
	if err := json.Unmarshal([]byte(events[0].Text), &payload); err != nil {
		t.Fatalf("json.Unmarshal(prompt meta echo) error = %v", err)
	}
	if got, want := payload.TurnSource, PromptTurnSourceNetwork; got != want {
		t.Fatalf("payload.TurnSource = %q, want %q", got, want)
	}
	if payload.Network == nil {
		t.Fatal("payload.Network = nil, want populated network metadata")
	}
	if got, want := payload.Network.MessageID, "msg-meta-1"; got != want {
		t.Fatalf("payload.Network.MessageID = %q, want %q", got, want)
	}
	if got, want := payload.Network.Surface, "direct"; got != want {
		t.Fatalf("payload.Network.Surface = %q, want %q", got, want)
	}
	if got, want := payload.Network.DirectID, "direct_meta_1"; got != want {
		t.Fatalf("payload.Network.DirectID = %q, want %q", got, want)
	}
	if got, want := payload.Network.WorkID, "work-meta-1"; got != want {
		t.Fatalf("payload.Network.WorkID = %q, want %q", got, want)
	}
	if got, want := payload.Network.Trust, "untrusted"; got != want {
		t.Fatalf("payload.Network.Trust = %q, want %q", got, want)
	}
}

func TestPromptStreamsSessionUpdates(t *testing.T) {
	t.Parallel()

	t.Run("Should stream session updates and refresh session metadata from prompt events", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-stream",
			Message: "hello",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}

		var eventTypes []string
		for _, event := range events {
			eventTypes = append(eventTypes, event.Type)
		}
		if !slices.Contains(eventTypes, EventTypeAgentMessage) {
			t.Fatalf("Prompt() event types = %#v, want agent message", eventTypes)
		}
		if !slices.Contains(eventTypes, EventTypeThought) {
			t.Fatalf("Prompt() event types = %#v, want thought", eventTypes)
		}
		if !slices.Contains(eventTypes, EventTypeToolCall) {
			t.Fatalf("Prompt() event types = %#v, want tool call", eventTypes)
		}
		if !slices.Contains(eventTypes, EventTypeDone) {
			t.Fatalf("Prompt() event types = %#v, want done", eventTypes)
		}
		for _, event := range events {
			if event.Type == EventTypeDone && event.PromptStopReason != PromptStopReasonEndTurn {
				t.Fatalf("Prompt() stop reason = %q, want %q", event.PromptStopReason, PromptStopReasonEndTurn)
			}
		}
		if proc.SessionID != "sess-new" {
			t.Fatalf("Start() session id = %q, want %q", proc.SessionID, "sess-new")
		}
		if caps := proc.CapsSnapshot(); !slices.Equal(caps.SupportedModes, []string{"new-mode"}) {
			t.Fatalf("Start() supported modes = %#v, want %#v", caps.SupportedModes, []string{"new-mode"})
		}
	})
}

func TestPromptErrorPreservesRequestErrorData(t *testing.T) {
	t.Parallel()

	t.Run("Should emit structured request error data for downstream marker classification", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "prompt_request_error_with_reason", "", StartOpts{})
		t.Cleanup(func() {
			stopProcess(t, driver, proc)
		})

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-mcp-auth",
			Message: "trigger structured auth error",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() events = empty, want error event")
		}
		event := events[len(events)-1]
		if event.Type != EventTypeError {
			t.Fatalf("Prompt() last event type = %q, want %q", event.Type, EventTypeError)
		}
		var payload struct {
			Data struct {
				ReasonCodes []string `json:"reason_codes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(event.Raw, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event.Raw) error = %v raw=%s", err, string(event.Raw))
		}
		if !slices.Contains(payload.Data.ReasonCodes, "mcp_auth_required") {
			t.Fatalf("request error reason codes = %#v, want mcp_auth_required", payload.Data.ReasonCodes)
		}
	})
}

func TestPromptStopDoesNotEmitRuntimeError(t *testing.T) {
	t.Parallel()

	t.Run("Should not emit runtime error after explicit stop", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "block_prompt_until_cancel", "", StartOpts{})
		stopped := false
		t.Cleanup(func() {
			if stopped {
				return
			}
			stopProcess(t, driver, proc)
		})

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-stop",
			Message: "block until stopped",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		select {
		case event := <-eventsCh:
			if got, want := event.Type, EventTypeAgentMessage; got != want {
				t.Fatalf("first prompt event = %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for blocking prompt to start")
		}

		if err := driver.Stop(testutil.Context(t), proc); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopped = true
		for _, event := range collectEvents(t, eventsCh) {
			if event.Type == EventTypeError {
				t.Fatalf("prompt events contain %q after explicit stop: %#v", EventTypeError, event)
			}
		}
	})
}

func TestShouldSuppressPromptErrorOnStop(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Should suppress context canceled errors",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "Should suppress deadline exceeded errors",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "Should suppress wrapped canceled failures",
			err:  WrapFailure(store.FailureCanceled, "stopped", context.Canceled),
			want: true,
		},
		{
			name: "Should suppress request errors carrying canceled details",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "context canceled"},
			},
			want: true,
		},
		{
			name: "Should suppress peer disconnect request errors after stop",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"error": "peer disconnected before response"},
			},
			want: true,
		},
		{
			name: "Should not suppress generic request failures",
			err: &acpsdk.RequestError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]any{"details": "Tool invocation failed"},
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldSuppressPromptErrorOnStop(tc.err); got != tc.want {
				t.Fatalf("shouldSuppressPromptErrorOnStop() = %v, want %v", got, tc.want)
			}
		})
	}
}
