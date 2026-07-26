package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestWhatsAppProgressRendering(t *testing.T) {
	t.Parallel()

	t.Run("Should acknowledge default-off progress without calling the WhatsApp API", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_500, 0).UTC(), "brg-progress-off")
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		apiCalls := 0
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI {
			apiCalls++
			return &fakeWhatsAppAPI{}
		}

		request := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-off",
			1,
			"call-off",
			"agh__terminal",
			"Inspecting",
		)
		ack, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			request,
		)
		if err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if ack.DeliveryID != request.Event.DeliveryID || ack.Seq != request.Event.Seq {
			t.Fatalf("progress ack = %#v, want delivery id %q seq %d", ack, request.Event.DeliveryID, request.Event.Seq)
		}
		if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
			t.Fatalf("progress ack remote ids = (%q, %q), want empty", ack.RemoteMessageID, ack.ReplaceRemoteMessageID)
		}
		if apiCalls != 0 {
			t.Fatalf("WhatsApp API factory calls = %d, want zero", apiCalls)
		}

		provider.storeDeliveryState(managed.Instance.ID, request.Event.DeliveryID, deliveryState{
			RemoteMessageID: "wamid.stale",
		})
		final := request
		final.Event.Seq = 2
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Final = true
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesProgress(default-off lifecycle final) error = %v", err)
		}
		_, deliveryExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, request.Event.DeliveryID))
		if deliveryExists {
			t.Fatal("default-off lifecycle-only delivery state still exists, want terminal removal")
		}
	})

	t.Run("Should detach pending progress after opt-out even when provider config is invalid", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_525, 0).UTC(), "brg-progress-disabled")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		api := &fakeWhatsAppAPI{nextMessageID: 25}
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return api }

		first := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-disabled",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-disabled",
			2,
			"call-2",
			"agh__task_list",
			"Reading tasks",
		)
		for _, request := range []bridgepkg.DeliveryRequest{first, second} {
			if _, err := provider.handleBridgesProgress(
				context.Background(),
				&bridgesdk.Session{},
				request,
			); err != nil {
				t.Fatalf("handleBridgesProgress(seq=%d) error = %v", request.Event.Seq, err)
			}
		}
		if provider.deliveryState(managed.Instance.ID, first.Event.DeliveryID).Progress == nil {
			t.Fatal("progress dispatcher before disabling = nil, want active dispatcher")
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("WhatsApp progress posts before disabling = %d, want %d pending update", got, want)
		}

		provider.routes.Update(managed.Instance.ID, func(cfg resolvedInstanceConfig) resolvedInstanceConfig {
			cfg.managed.Instance.DeliveryDefaults = json.RawMessage(
				`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
			)
			cfg.configError = errors.New("whatsapp: provider config became invalid")
			return cfg
		})
		disabled := second
		disabled.Event.Seq = 3
		disabled.Event.Progress.ToolCallID = "call-3"
		disabled.Event.Progress.Index = 3
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			disabled,
		); err != nil {
			t.Fatalf("handleBridgesProgress(disabled invalid config) error = %v", err)
		}

		if provider.deliveryState(managed.Instance.ID, first.Event.DeliveryID).Progress != nil {
			t.Fatal("progress dispatcher after disabling != nil, want detached dispatcher")
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("WhatsApp progress posts after disabling = %d, want %d", got, want)
		}
	})

	t.Run("Should retain failed progress for retry and report unhealthy", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_550, 0).UTC(), "brg-progress-failure")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"separate","typing":false,"reactions":false}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI {
			return fakeWhatsAppAPIError{err: errors.New("messages unavailable")}
		}

		request := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-failure",
			1,
			"call-failure",
			"agh__terminal",
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			request,
		); err == nil {
			t.Fatal("handleBridgesProgress() error = nil, want failed message error")
		}
		if provider.deliveryState(managed.Instance.ID, request.Event.DeliveryID).Progress == nil {
			t.Fatal("failed progress dispatcher = nil, want retained dispatcher for retry")
		}
		if err := provider.healthCheck(); err == nil {
			t.Fatal("healthCheck() error = nil, want reported progress failure")
		}
	})

	t.Run("Should post distinct projected tools as separate status messages", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_600, 0).UTC(), "brg-progress-separate")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"new","grouping":"separate","typing":true,"reactions":true}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		api := &fakeWhatsAppAPI{nextMessageID: 50}
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return api }

		toolIDs := []string{"agh__terminal", "agh__task_list"}
		labels := []string{"Inspecting", "Reading tasks"}
		var first bridgepkg.DeliveryRequest
		for index := range toolIDs {
			request := testWhatsAppProgressRequest(
				managed.Instance.ID,
				"delivery-progress-separate",
				int64(index+1),
				fmt.Sprintf("call-%d", index+1),
				toolIDs[index],
				labels[index],
			)
			if index == 0 {
				first = request
			}
			if _, err := provider.handleBridgesProgress(
				context.Background(),
				&bridgesdk.Session{},
				request,
			); err != nil {
				t.Fatalf("handleBridgesProgress(seq=%d) error = %v", request.Event.Seq, err)
			}
		}

		final := first
		final.Event.Seq = 3
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Final = true
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesProgress(lifecycle final) error = %v", err)
		}
		_, deliveryExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, first.Event.DeliveryID))
		if deliveryExists {
			t.Fatal("lifecycle-only delivery state still exists, want terminal removal")
		}

		staleDeliveryID := "delivery-progress-separate-stale"
		provider.storeDeliveryState(managed.Instance.ID, staleDeliveryID, deliveryState{
			RemoteMessageID: "wamid.stale",
		})
		staleFinal := final
		staleFinal.Event.DeliveryID = staleDeliveryID
		staleFinal.Event.Seq = 4
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			staleFinal,
		); err != nil {
			t.Fatalf("handleBridgesProgress(stale lifecycle final) error = %v", err)
		}
		_, staleExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, staleDeliveryID))
		if staleExists {
			t.Fatal("dispatcher-free lifecycle-only state still exists, want terminal removal")
		}

		if got, want := len(api.requests), 2; got != want {
			t.Fatalf("WhatsApp progress posts = %d, want %d", got, want)
		}
		for index, request := range api.requests {
			if got, want := request.MessagingProduct, providerWhatsappKey; got != want {
				t.Fatalf("progress post %d messaging product = %q, want %q", index, got, want)
			}
			if got, want := request.To, "15551234567"; got != want {
				t.Fatalf("progress post %d target = %q, want %q", index, got, want)
			}
			if got, want := request.RecipientType, "individual"; got != want {
				t.Fatalf("progress post %d recipient type = %q, want %q", index, got, want)
			}
			if got, want := request.Type, providerTextKey; got != want {
				t.Fatalf("progress post %d type = %q, want %q", index, got, want)
			}
			if got, want := request.Text.Body, labels[index]; got != want {
				t.Fatalf("progress post %d body = %q, want %q", index, got, want)
			}
			if request.Text.PreviewURL {
				t.Fatalf("progress post %d preview URL = true, want false", index)
			}
		}
	})

	t.Run("Should append an accumulated progress edit when a configured grouping requests it", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_700, 0).UTC(), "brg-progress-accumulate")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		api := &fakeWhatsAppAPI{nextMessageID: 60}
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return api }

		first := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-accumulate",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-accumulate",
			2,
			"call-2",
			"agh__task_list",
			"Reading tasks",
		)
		for _, request := range []bridgepkg.DeliveryRequest{first, second} {
			if _, err := provider.handleBridgesProgress(
				context.Background(),
				&bridgesdk.Session{},
				request,
			); err != nil {
				t.Fatalf("handleBridgesProgress(seq=%d) error = %v", request.Event.Seq, err)
			}
		}
		final := first
		final.Event.Seq = 3
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Final = true
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesProgress(lifecycle final) error = %v", err)
		}

		if got, want := len(api.requests), 2; got != want {
			t.Fatalf("WhatsApp accumulated progress posts = %d, want %d", got, want)
		}
		if got, want := api.requests[1].Text.Body, "Reading tasks"; got != want {
			t.Fatalf("WhatsApp appended progress body = %q, want %q", got, want)
		}
	})

	t.Run("Should advance the append cursor after a remotely committed progress edit", func(t *testing.T) {
		t.Parallel()

		api := &fakeWhatsAppAPI{
			nextMessageID: 90,
			sendErrorText: "B",
			sendErr: bridgesdk.MarkCommittedMutation(
				errors.New("WhatsApp progress append result unavailable"),
			),
		}
		sink := &whatsappProgressSink{
			api:           api,
			phoneNumberID: "123456789",
			targetUserID:  "15551234567",
		}
		target := bridgepkg.DeliveryTarget{PeerID: "15551234567"}

		if _, err := sink.Post(t.Context(), target, "A"); err != nil {
			t.Fatalf("Post(A) error = %v", err)
		}
		if err := sink.Edit(t.Context(), target, "wamid.90", "A\nB"); !bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("Edit(A/B) error = %T %v, want committed mutation", err, err)
		}
		if err := sink.Edit(t.Context(), target, "wamid.90", "A\nB\nC"); err != nil {
			t.Fatalf("Edit(A/B/C) error = %v", err)
		}

		if got, want := len(api.attempts), 3; got != want {
			t.Fatalf("WhatsApp progress attempts = %d, want %d", got, want)
		}
		for index, want := range []string{"A", "B", "C"} {
			if got := api.attempts[index].Text.Body; got != want {
				t.Fatalf("WhatsApp progress attempt %d = %q, want %q", index, got, want)
			}
		}
	})

	t.Run("Should deliver the final answer after separate progress statuses", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_750, 0).UTC(), "brg-progress-final")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"new","grouping":"separate","typing":false,"reactions":false}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		api := &fakeWhatsAppAPI{nextMessageID: 65}
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return api }

		progress := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-final",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			progress,
		); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		final := testDeliveryRequest(
			managed.Instance.ID,
			progress.Event.DeliveryID,
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
			"Final answer",
		)
		if _, err := provider.handleBridgesDeliver(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}

		if got, want := len(api.requests), 2; got != want {
			t.Fatalf("WhatsApp progress and final posts = %d, want %d", got, want)
		}
		if got, want := api.requests[0].Text.Body, "Inspecting"; got != want {
			t.Fatalf("WhatsApp progress body = %q, want %q", got, want)
		}
		if got, want := api.requests[1].Text.Body, "Final answer"; got != want {
			t.Fatalf("WhatsApp final body = %q, want %q", got, want)
		}
		state := provider.deliveryState(managed.Instance.ID, progress.Event.DeliveryID)
		if state.Progress != nil {
			t.Fatal("terminal text delivery progress dispatcher != nil, want detached dispatcher")
		}
	})

	t.Run("Should deliver final text once after a committed progress flush", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_775, 0).UTC(), "brg-progress-committed-flush")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		api := &fakeWhatsAppAPI{
			nextMessageID: 75,
			sendErrorText: "Reading tasks",
			sendErr: &bridgesdk.CommittedMutationError{
				Err: errors.New("WhatsApp progress append result unavailable"),
			},
		}
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return api }

		first := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-committed-flush",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testWhatsAppProgressRequest(
			managed.Instance.ID,
			first.Event.DeliveryID,
			2,
			"call-2",
			"agh__task_list",
			"Reading tasks",
		)
		for _, request := range []bridgepkg.DeliveryRequest{first, second} {
			if _, err := provider.handleBridgesProgress(
				context.Background(),
				&bridgesdk.Session{},
				request,
			); err != nil {
				t.Fatalf("handleBridgesProgress(seq=%d) error = %v", request.Event.Seq, err)
			}
		}

		final := testDeliveryRequest(
			managed.Instance.ID,
			first.Event.DeliveryID,
			3,
			bridgepkg.DeliveryEventTypeFinal,
			true,
			"Final answer",
		)
		if _, err := provider.handleBridgesDeliver(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}

		if got, want := countWhatsAppSendRequests(api.attempts, "Reading tasks"), 1; got != want {
			t.Fatalf("WhatsApp committed progress append attempts = %d, want %d", got, want)
		}
		if got, want := countWhatsAppSendRequests(api.attempts, "Final answer"), 1; got != want {
			t.Fatalf("WhatsApp final send attempts = %d, want %d", got, want)
		}
		if state := provider.deliveryState(managed.Instance.ID, first.Event.DeliveryID); state.Progress != nil {
			t.Fatal("terminal delivery progress dispatcher != nil after final text")
		}
	})

	t.Run("Should remove and close delivery state after committed final text", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_787, 0).UTC(), "brg-final-committed")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		api := &fakeWhatsAppAPI{
			nextMessageID: 85,
			sendErrorText: "Final answer",
			sendErr: &bridgesdk.CommittedMutationError{
				Err: errors.New("WhatsApp final send result unavailable"),
			},
		}
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return api }

		progress := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-final-committed",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			progress,
		); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		dispatcher := provider.deliveryState(managed.Instance.ID, progress.Event.DeliveryID).Progress
		if dispatcher == nil {
			t.Fatal("progress dispatcher = nil before final delivery")
		}

		final := testDeliveryRequest(
			managed.Instance.ID,
			progress.Event.DeliveryID,
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
			"Final answer",
		)
		_, err = provider.handleBridgesDeliver(
			context.Background(),
			&bridgesdk.Session{},
			final,
		)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("handleBridgesDeliver(final) error = %T %v, want CommittedMutationError", err, err)
		}
		if _, exists := provider.deliveries.Load(
			deliveryStateKey(managed.Instance.ID, progress.Event.DeliveryID),
		); exists {
			t.Fatal("delivery state still exists after committed final text")
		}
		if err := dispatcher.Flush(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("removed progress dispatcher Flush() error = %v, want closed dispatcher", err)
		}
		if got, want := countWhatsAppSendRequests(api.attempts, "Final answer"), 1; got != want {
			t.Fatalf("WhatsApp committed final send attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should measure progress limits in Unicode code points", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(time.Unix(8_800, 0).UTC(), "brg-progress-unicode")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"separate","typing":false,"reactions":false}}`,
		)
		setWhatsAppProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:       &managed,
			instanceID:    managed.Instance.ID,
			phoneNumberID: "123456789",
		})
		api := &fakeWhatsAppAPI{nextMessageID: 70}
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI { return api }

		label := strings.Repeat("🙂", 4_000)
		request := testWhatsAppProgressRequest(
			managed.Instance.ID,
			"delivery-progress-unicode",
			1,
			"call-unicode",
			"agh__terminal",
			label,
		)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			request,
		); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("WhatsApp Unicode progress posts = %d, want %d", got, want)
		}
		if got := len([]rune(api.requests[0].Text.Body)); got != len([]rune(label)) {
			t.Fatalf("WhatsApp Unicode progress runes = %d, want %d", got, len([]rune(label)))
		}
	})
}

func TestMapWhatsAppInboundMessageAndDMPolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-whatsapp")
	contact := &whatsappContact{WaID: "15551234567"}
	contact.Profile.Name = "Alice Example"

	envelope, err := mapWhatsAppInboundMessage(whatsappInboundMessage{
		ID:        "wamid.abc123",
		From:      "15551234567",
		Timestamp: strconvTime(now),
		Type:      "image",
		Context: &struct {
			From string `json:"from,omitempty"`
			ID   string `json:"id,omitempty"`
		}{
			From: "15557654321",
			ID:   "wamid.parent",
		},
		Image: &struct {
			ID       string `json:"id,omitempty"`
			MIMEType string `json:"mime_type,omitempty"`
			Caption  string `json:"caption,omitempty"`
			SHA256   string `json:"sha256,omitempty"`
		}{
			ID:       "media-1",
			MIMEType: "image/jpeg",
			Caption:  "Need a summary",
		},
	}, contact, &managed, time.Time{}, "123456789")
	if err != nil {
		t.Fatalf("mapWhatsAppInboundMessage() error = %v", err)
	}
	if got, want := envelope.PeerID, "15551234567"; got != want {
		t.Fatalf("envelope.PeerID = %q, want %q", got, want)
	}
	if got := envelope.GroupID; got != "" {
		t.Fatalf("envelope.GroupID = %q, want empty", got)
	}
	if got := envelope.ThreadID; got != "" {
		t.Fatalf("envelope.ThreadID = %q, want empty", got)
	}
	if got, want := envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("envelope.Content.Text = %q, want %q", got, want)
	}
	if got, want := envelope.Sender.DisplayName, "Alice Example"; got != want {
		t.Fatalf("envelope.Sender.DisplayName = %q, want %q", got, want)
	}
	if got, want := len(envelope.Attachments), 1; got != want {
		t.Fatalf("len(envelope.Attachments) = %d, want %d", got, want)
	}

	sender := envelope.Sender
	if !allowWhatsAppDirectMessage(
		resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyOpen},
		sender,
	) {
		t.Fatal("allowWhatsAppDirectMessage(open) = false, want true")
	}
	if !allowWhatsAppDirectMessage(resolvedInstanceConfig{
		dmPolicy:       bridgepkg.BridgeDMPolicyAllowlist,
		allowUserIDs:   map[string]struct{}{"15551234567": {}},
		allowUsernames: map[string]struct{}{"alice example": {}},
	}, sender) {
		t.Fatal("allowWhatsAppDirectMessage(allowlist) = false, want true")
	}
	if !allowWhatsAppDirectMessage(resolvedInstanceConfig{
		dmPolicy:        bridgepkg.BridgeDMPolicyPairing,
		pairedUsernames: map[string]struct{}{"alice example": {}},
	}, sender) {
		t.Fatal("allowWhatsAppDirectMessage(pairing) = false, want true")
	}
	if allowWhatsAppDirectMessage(resolvedInstanceConfig{
		dmPolicy: bridgepkg.BridgeDMPolicyAllowlist,
	}, sender) {
		t.Fatal("allowWhatsAppDirectMessage(rejected) = true, want false")
	}
}

func TestVerifyChallengeAndSignature(t *testing.T) {
	t.Parallel()

	body := []byte(whatsappWebhookPayloadForPhone("123456789", "hello"))
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/whatsapp/brg-1",
		strings.NewReader(string(body)),
	)
	req.Header.Set(whatsappSignatureHeader, signWhatsAppBody(body, "top-secret"))
	if err := verifyWhatsAppSignature(context.Background(), req, body, "top-secret"); err != nil {
		t.Fatalf("verifyWhatsAppSignature(valid) error = %v", err)
	}
	if err := verifyWhatsAppSignature(context.Background(), req, body, "wrong"); err == nil {
		t.Fatal("verifyWhatsAppSignature(invalid) error = nil, want non-nil")
	}

	provider, err := newWhatsAppProvider(io.Discard)
	if err != nil {
		t.Fatalf("newWhatsAppProvider() error = %v", err)
	}
	cfg := resolvedInstanceConfig{verifyToken: "verify-me"}

	okReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://example.test/whatsapp/brg-1?hub.mode=subscribe&hub.verify_token=verify-me&hub.challenge=12345",
		http.NoBody,
	)
	okResp := httptest.NewRecorder()
	provider.handleVerifyChallenge(okResp, okReq, cfg)
	if got, want := okResp.Code, http.StatusOK; got != want {
		t.Fatalf("verify challenge status = %d, want %d", got, want)
	}
	if got, want := strings.TrimSpace(okResp.Body.String()), "12345"; got != want {
		t.Fatalf("verify challenge body = %q, want %q", got, want)
	}

	badReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://example.test/whatsapp/brg-1?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=12345",
		http.NoBody,
	)
	badResp := httptest.NewRecorder()
	provider.handleVerifyChallenge(badResp, badReq, cfg)
	if got, want := badResp.Code, http.StatusForbidden; got != want {
		t.Fatalf("verify challenge forbidden status = %d, want %d", got, want)
	}
}

func TestExecuteWhatsAppDeliveryPostResumeDeleteAndSplit(t *testing.T) {
	t.Parallel()

	api := &fakeWhatsAppAPI{nextMessageID: 500}
	cfg := resolvedInstanceConfig{
		instanceID:    "brg-1",
		phoneNumberID: "123456789",
	}

	startReq := testDeliveryRequest(
		"brg-1",
		"delivery-1",
		1,
		bridgepkg.DeliveryEventTypeStart,
		false,
		"hello",
	)
	startAck, state, err := executeWhatsAppDelivery(
		context.Background(),
		api,
		cfg,
		startReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeWhatsAppDelivery(start) error = %v", err)
	}
	if got, want := startAck.RemoteMessageID, "wamid.500"; got != want {
		t.Fatalf("startAck.RemoteMessageID = %q, want %q", got, want)
	}

	finalReq := testDeliveryRequest(
		"brg-1",
		"delivery-1",
		2,
		bridgepkg.DeliveryEventTypeFinal,
		true,
		"hello world",
	)
	finalAck, state, err := executeWhatsAppDelivery(context.Background(), api, cfg, finalReq, state)
	if err != nil {
		t.Fatalf("executeWhatsAppDelivery(final) error = %v", err)
	}
	if got, want := finalAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("finalAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}

	deleteReq := testDeleteRequest("brg-1", "delivery-1", 3, finalAck.RemoteMessageID)
	if _, _, err := executeWhatsAppDelivery(context.Background(), api, cfg, deleteReq, state); err == nil {
		t.Fatal("executeWhatsAppDelivery(delete) error = nil, want non-nil")
	}

	resumeSnapshotReq := testDeliveryRequest(
		"brg-1",
		"delivery-2",
		1,
		bridgepkg.DeliveryEventTypeResume,
		true,
		"hello",
	)
	resumeSnapshotReq.Event.Resume = &bridgepkg.DeliveryResumeState{
		LatestEventType: bridgepkg.DeliveryEventTypeFinal,
	}
	resumeSnapshotReq.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       "delivery-2",
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-1",
		RoutingKey:       resumeSnapshotReq.Event.RoutingKey,
		DeliveryTarget:   resumeSnapshotReq.Event.DeliveryTarget,
		LatestSeq:        1,
		LastSentSeq:      1,
		LastAckedSeq:     1,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   bridgepkg.MessageContent{Text: "hello"},
		RemoteMessageID:  "wamid.resume",
		Final:            true,
		UpdatedAt:        time.Date(2026, 4, 15, 12, 5, 0, 0, time.UTC),
	}
	resumeAck, _, err := executeWhatsAppDelivery(
		context.Background(),
		api,
		cfg,
		resumeSnapshotReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeWhatsAppDelivery(resume with snapshot remote) error = %v", err)
	}
	if got, want := resumeAck.RemoteMessageID, "wamid.resume"; got != want {
		t.Fatalf("resumeAck.RemoteMessageID = %q, want %q", got, want)
	}

	failOpenAPI := &fakeWhatsAppAPI{nextMessageID: 850}
	failOpenReq := testDeliveryRequest(
		"brg-1",
		"delivery-fail-open",
		8,
		bridgepkg.DeliveryEventTypeError,
		true,
		"session stopped before delivery completed",
	)
	failOpenReq.Event.Error = &bridgepkg.DeliveryErrorDetail{
		Message: "session stopped before delivery completed",
	}
	failOpenAck, _, err := executeWhatsAppDelivery(
		context.Background(),
		failOpenAPI,
		cfg,
		failOpenReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeWhatsAppDelivery(fail-open terminal post) error = %v", err)
	}
	if len(failOpenAPI.requests) != 1 ||
		failOpenAPI.requests[0].Text.Body != "session stopped before delivery completed" {
		t.Fatalf("fail-open requests = %#v, want one visible terminal post", failOpenAPI.requests)
	}
	if failOpenAck.RemoteMessageID == "wamid.resume" {
		t.Fatalf("fail-open ack = %#v, want newly materialized message", failOpenAck)
	}

	splitAPI := &fakeWhatsAppAPI{nextMessageID: 900}
	splitReq := testDeliveryRequest(
		"brg-1",
		"delivery-3",
		1,
		bridgepkg.DeliveryEventTypeStart,
		false,
		strings.Repeat("a", whatsappMaxMessageRunes+20),
	)
	splitAck, _, err := executeWhatsAppDelivery(
		context.Background(),
		splitAPI,
		cfg,
		splitReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeWhatsAppDelivery(split) error = %v", err)
	}
	if got, want := splitAck.RemoteMessageID, "wamid.901"; got != want {
		t.Fatalf("splitAck.RemoteMessageID = %q, want %q", got, want)
	}
	if got, want := len(splitAPI.requests), 2; got != want {
		t.Fatalf("len(splitAPI.requests) = %d, want %d", got, want)
	}

	t.Run("Should resume at the first unsent chunk after exhausting Retry-After attempts", func(t *testing.T) {
		t.Parallel()

		request := testDeliveryRequest(
			"brg-1",
			"delivery-partial-chunk",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
			strings.Repeat("partial WhatsApp delivery ", 400),
		)
		chunks := bridgesdk.ChunkMessage(request.Event.Content.Text, whatsappMaxMessageRunes, nil)
		if len(chunks) < 2 {
			t.Fatalf("ChunkMessage() returned %d chunk, want at least 2", len(chunks))
		}

		api := &fakeWhatsAppAPI{
			nextMessageID: 1_000,
			sendErrorText: chunks[1],
			sendErr: &bridgesdk.RateLimitError{
				Err:        errors.New("rate limited continuation"),
				RetryAfter: time.Nanosecond,
			},
		}
		_, partialState, err := executeWhatsAppDelivery(
			context.Background(), api, cfg, request, deliveryState{},
		)
		if err == nil {
			t.Fatal("executeWhatsAppDelivery(partial) error = nil, want exhausted rate limit")
		}
		if got, want := countWhatsAppSendRequests(
			api.attempts,
			chunks[1],
		), bridgesdk.DefaultRetryConfig().Attempts; got != want {
			t.Fatalf("failed continuation attempts = %d, want %d", got, want)
		}
		if got, want := countWhatsAppSendRequests(api.requests, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk sends = %d, want %d", got, want)
		}

		api.sendErr = nil
		resume := testWhatsAppResumeRequest(request)
		if _, _, err := executeWhatsAppDelivery(context.Background(), api, cfg, resume, partialState); err != nil {
			t.Fatalf("executeWhatsAppDelivery(resume) error = %v", err)
		}
		if got, want := countWhatsAppSendRequests(api.requests, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk sends after resume = %d, want %d", got, want)
		}
		if got, want := len(api.requests), len(chunks); got != want {
			t.Fatalf("successful chunk sends = %d, want %d", got, want)
		}
	})
}

func TestExecuteWhatsAppDeliveryRejectsInvalidOrUnacknowledgedSends(t *testing.T) {
	t.Parallel()

	cfg := resolvedInstanceConfig{
		instanceID:    "brg-1",
		phoneNumberID: "123456789",
	}

	t.Run("Should reject an invalid request before calling the Cloud API", func(t *testing.T) {
		t.Parallel()

		api := &whatsappDeliveryResponseAPI{}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-invalid",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			"hello",
		)
		request.Event.BridgeInstanceID = "other-instance"

		if _, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		); err == nil {
			t.Fatal("executeWhatsAppDelivery() error = nil, want invalid request error")
		}
		if got := len(api.requests); got != 0 {
			t.Fatalf("Cloud API calls = %d, want 0", got)
		}
	})

	t.Run("Should reject an out-of-order event before calling the Cloud API", func(t *testing.T) {
		t.Parallel()

		api := &whatsappDeliveryResponseAPI{}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-stale",
			1,
			bridgepkg.DeliveryEventTypeDelta,
			false,
			"hello",
		)

		if _, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{LastSeq: 1},
		); err == nil {
			t.Fatal("executeWhatsAppDelivery() error = nil, want out-of-order error")
		}
		if got := len(api.requests); got != 0 {
			t.Fatalf("Cloud API calls = %d, want 0", got)
		}
	})

	t.Run("Should preserve a Cloud API send failure without acknowledging the delivery", func(t *testing.T) {
		t.Parallel()

		sendErr := errors.New("cloud unavailable")
		api := &whatsappDeliveryResponseAPI{sendErr: sendErr}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-send-failure",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			"hello",
		)

		ack, state, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		)
		if !errors.Is(err, sendErr) {
			t.Fatalf("executeWhatsAppDelivery() error = %v, want wrapped send failure", err)
		}
		if ack != (bridgepkg.DeliveryAck{}) ||
			state.LastSeq != 0 ||
			state.RemoteMessageID != "" ||
			state.ReplaceRemoteMessageID != "" ||
			state.Chunks.NextChunk() != 0 {
			t.Fatalf("failed delivery = ack:%#v state:%#v, want no committed chunk", ack, state)
		}
	})

	t.Run("Should classify a response without a message id as committed with unavailable result", func(t *testing.T) {
		t.Parallel()

		api := &whatsappDeliveryResponseAPI{response: &whatsappSendMessageResponse{}}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-missing-id",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			"hello",
		)

		ack, state, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("executeWhatsAppDelivery() error = %T, want *bridgesdk.CommittedMutationError", err)
		}
		if ack != (bridgepkg.DeliveryAck{}) ||
			state.LastSeq != 0 ||
			state.RemoteMessageID != "" ||
			state.ReplaceRemoteMessageID != "" ||
			state.Chunks.NextChunk() != 0 {
			t.Fatalf("unacknowledged delivery = ack:%#v state:%#v, want no committed chunk", ack, state)
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("Cloud API calls = %d, want %d", got, want)
		}
	})

	t.Run("Should reject whitespace-only text before calling the Cloud API", func(t *testing.T) {
		t.Parallel()

		api := &whatsappDeliveryResponseAPI{}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-empty-text",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			"   ",
		)

		if _, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		); err == nil {
			t.Fatal("executeWhatsAppDelivery() error = nil, want empty text error")
		}
		if got := len(api.requests); got != 0 {
			t.Fatalf("Cloud API calls = %d, want 0", got)
		}
	})

	t.Run("Should reject a delivery without a peer target", func(t *testing.T) {
		t.Parallel()

		api := &whatsappDeliveryResponseAPI{}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-missing-peer",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			"hello",
		)
		request.Event.RoutingKey.PeerID = ""
		request.Event.DeliveryTarget.PeerID = ""

		if _, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		); err == nil {
			t.Fatal("executeWhatsAppDelivery() error = nil, want missing peer error")
		}
		if got := len(api.requests); got != 0 {
			t.Fatalf("Cloud API calls = %d, want 0", got)
		}
	})

	t.Run("Should classify an empty response message id as committed with unavailable result", func(t *testing.T) {
		t.Parallel()

		response := &whatsappSendMessageResponse{}
		response.Messages = append(response.Messages, struct {
			ID string `json:"id,omitempty"`
		}{})
		api := &whatsappDeliveryResponseAPI{response: response}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-empty-id",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			"hello",
		)

		_, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("executeWhatsAppDelivery() error = %T, want *bridgesdk.CommittedMutationError", err)
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("Cloud API calls = %d, want %d", got, want)
		}
	})

	t.Run("Should replay a resume snapshot that has no acknowledged remote message", func(t *testing.T) {
		t.Parallel()

		api := &fakeWhatsAppAPI{nextMessageID: 700}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-unacknowledged-resume",
			1,
			bridgepkg.DeliveryEventTypeResume,
			true,
			"hello",
		)
		request.Event.Resume = &bridgepkg.DeliveryResumeState{
			LatestEventType: bridgepkg.DeliveryEventTypeFinal,
		}
		request.Snapshot = &bridgepkg.DeliverySnapshot{
			DeliveryID:       request.Event.DeliveryID,
			SessionID:        "sess-1",
			TurnID:           "turn-1",
			BridgeInstanceID: request.Event.BridgeInstanceID,
			RoutingKey:       request.Event.RoutingKey,
			DeliveryTarget:   request.Event.DeliveryTarget,
			LatestSeq:        1,
			LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
			CurrentContent:   bridgepkg.MessageContent{Text: "hello"},
			Final:            true,
			UpdatedAt:        time.Date(2026, 4, 15, 12, 10, 0, 0, time.UTC),
		}

		ack, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		)
		if err != nil {
			t.Fatalf("executeWhatsAppDelivery() error = %v", err)
		}
		if got, want := ack.RemoteMessageID, "wamid.700"; got != want {
			t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("Cloud API calls = %d, want %d", got, want)
		}
	})
}

func TestExecuteWhatsAppDeliveryReferences(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve an explicit replacement reference when local state is empty", func(t *testing.T) {
		t.Parallel()

		api := &fakeWhatsAppAPI{nextMessageID: 800}
		cfg := resolvedInstanceConfig{
			instanceID:    "brg-1",
			phoneNumberID: "123456789",
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-explicit-edit",
			1,
			bridgepkg.DeliveryEventTypeDelta,
			false,
			"edited text",
		)
		const referencedMessage = "wamid.referenced"
		request.Event.Operation = bridgepkg.DeliveryOperationEdit
		request.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: referencedMessage}

		ack, state, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		)
		if err != nil {
			t.Fatalf("executeWhatsAppDelivery() error = %v", err)
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("SendTextMessage calls = %d, want %d", got, want)
		}
		if got, want := ack.RemoteMessageID, "wamid.800"; got != want {
			t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
		}
		if got := ack.ReplaceRemoteMessageID; got != referencedMessage {
			t.Fatalf("ack.ReplaceRemoteMessageID = %q, want %q", got, referencedMessage)
		}
		if got := state.ReplaceRemoteMessageID; got != referencedMessage {
			t.Fatalf("state.ReplaceRemoteMessageID = %q, want %q", got, referencedMessage)
		}
	})
}

func TestClassifyWhatsAppHTTPError(t *testing.T) {
	t.Parallel()

	rate := classifyWhatsAppHTTPError(
		http.StatusTooManyRequests,
		"5",
		[]byte(`{"error":{"message":"slow down","code":130429}}`),
	)
	var rateErr *bridgesdk.RateLimitError
	if !errors.As(rate, &rateErr) {
		t.Fatalf("classifyWhatsAppHTTPError(rate) = %T, want *RateLimitError", rate)
	}
	if got, want := rateErr.RetryAfter, 5*time.Second; got != want {
		t.Fatalf("rateErr.RetryAfter = %s, want %s", got, want)
	}

	auth := classifyWhatsAppHTTPError(
		http.StatusUnauthorized,
		"",
		[]byte(`{"error":{"message":"invalid token","code":190}}`),
	)
	var authErr *bridgesdk.AuthError
	if !errors.As(auth, &authErr) {
		t.Fatalf("classifyWhatsAppHTTPError(auth) = %T, want *AuthError", auth)
	}

	serverErr := classifyWhatsAppHTTPError(
		http.StatusBadGateway,
		"",
		[]byte(`{"error":{"message":"upstream failed","code":2}}`),
	)
	var serverHTTPError *bridgesdk.HTTPError
	if !errors.As(serverErr, &serverHTTPError) ||
		bridgesdk.ClassifyError(serverErr).Class != bridgesdk.ErrorClassServerError {
		t.Fatalf("classifyWhatsAppHTTPError(502) = %#v, want server_error HTTPError", serverErr)
	}

	permanent := classifyWhatsAppHTTPError(
		http.StatusBadRequest,
		"",
		[]byte(`{"error":{"message":"bad request","code":100}}`),
	)
	var httpErr *bridgesdk.HTTPError
	if !errors.As(permanent, &httpErr) {
		t.Fatalf("classifyWhatsAppHTTPError(http) = %T, want *HTTPError", permanent)
	}
	if got, want := httpErr.Message, "bad request"; got != want {
		t.Fatalf("httpErr.Message = %q, want %q", got, want)
	}
}

func TestResolveInstanceConfigAndDetermineInitialState(t *testing.T) {
	env := setProviderTestEnv(t)
	_ = env
	listenAddr := reserveListenAddr(t)
	t.Setenv(whatsappAPIBaseEnv, "http://api.example/")

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	managed.Instance.DMPolicy = bridgepkg.BridgeDMPolicyPairing
	managed.Instance.ProviderConfig = fmt.Appendf(nil, `{
		"api_version":"v99.0",
		"phone_number_id":"123456789",
		"webhook":{"listen_addr":%q,"path":"whatsapp"},
		"batching":{"delay_ms":5,"split_delay_ms":7,"split_threshold":2},
		"dm":{"allow_user_ids":["15551234567"],"allow_usernames":["Alice Example"],"paired_usernames":["Bob"]}
	}`, listenAddr)
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "access_token", Kind: "token", Value: "access-token"},
		{BindingName: "app_secret", Kind: "token", Value: "app-secret"},
		{BindingName: "verify_token", Kind: "token", Value: "verify-token"},
	}

	mustHandleLifecycle(t, hostPeer, managed)
	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	select {
	case <-runtime.lifecycle.Initialized():
	case <-t.Context().Done():
		t.Fatal("provider initialization did not finish")
	}

	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want session")
	}

	cfg := runtime.resolveInstanceConfig(session, managed)
	if cfg.configError != nil {
		t.Fatalf("resolveInstanceConfig() configError = %v, want nil", cfg.configError)
	}
	defer cfg.batcher.Close()

	if got, want := cfg.apiBaseURL, "http://api.example"; got != want {
		t.Fatalf("cfg.apiBaseURL = %q, want %q", got, want)
	}
	if got, want := cfg.apiVersion, "v99.0"; got != want {
		t.Fatalf("cfg.apiVersion = %q, want %q", got, want)
	}
	if got, want := cfg.phoneNumberID, "123456789"; got != want {
		t.Fatalf("cfg.phoneNumberID = %q, want %q", got, want)
	}
	if got, want := cfg.webhookPath, "/whatsapp"; got != want {
		t.Fatalf("cfg.webhookPath = %q, want %q", got, want)
	}
	if got, want := cfg.accessToken, "access-token"; got != want {
		t.Fatalf("cfg.accessToken = %q, want %q", got, want)
	}
	if got, want := cfg.appSecret, "app-secret"; got != want {
		t.Fatalf("cfg.appSecret = %q, want %q", got, want)
	}
	if got, want := cfg.verifyToken, "verify-token"; got != want {
		t.Fatalf("cfg.verifyToken = %q, want %q", got, want)
	}
	if cfg.batcher == nil {
		t.Fatal("cfg.batcher = nil, want batcher")
	}
	if _, ok := cfg.allowUserIDs["15551234567"]; !ok {
		t.Fatalf("cfg.allowUserIDs = %#v, want normalized user id", cfg.allowUserIDs)
	}
	if _, ok := cfg.allowUsernames["alice example"]; !ok {
		t.Fatalf("cfg.allowUsernames = %#v, want normalized username", cfg.allowUsernames)
	}

	status, degradation, err := runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:  "bad-config",
			configError: errors.New("bad config"),
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(configError) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil ||
		degradation.Reason != bridgepkg.BridgeDegradationReasonTenantConfigInvalid {
		t.Fatalf("degradation = %#v, want tenant config invalid", degradation)
	}

	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:    "missing-auth",
			phoneNumberID: "123456789",
			verifyToken:   "verify-token",
			appSecret:     "app-secret",
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(missing auth) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("degradation = %#v, want auth failed", degradation)
	}

	runtime.apiFactory = func(cfg resolvedInstanceConfig) whatsappAPI {
		switch cfg.instanceID {
		case "auth":
			return fakeWhatsAppAPIError{err: &bridgesdk.AuthError{Err: errors.New("invalid token")}}
		case "rate":
			return fakeWhatsAppAPIError{
				err: &bridgesdk.RateLimitError{
					Err:        errors.New("slow down"),
					RetryAfter: time.Second,
				},
			}
		default:
			return &fakeWhatsAppAPI{}
		}
	}

	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:    "auth",
			phoneNumberID: "123456789",
			accessToken:   "access-token",
			appSecret:     "app-secret",
			verifyToken:   "verify-token",
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(auth) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("degradation = %#v, want auth failed", degradation)
	}

	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:    "rate",
			phoneNumberID: "123456789",
			accessToken:   "access-token",
			appSecret:     "app-secret",
			verifyToken:   "verify-token",
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(rate) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonRateLimited {
		t.Fatalf("degradation = %#v, want rate limited", degradation)
	}
}

func TestRuntimeInitializeStartsServerAndWritesMarkers(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newWhatsAppAPIServer(t)
	t.Setenv(whatsappListenAddrEnv, listenAddr)
	t.Setenv(whatsappAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	managed := []subprocess.InitializeBridgeManagedInstance{
		testBridgeRuntime(now, "brg-1"),
		testBridgeRuntime(now, "brg-2"),
	}
	mustHandleLifecycle(t, hostPeer, managed...)

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	handshake := waitForJSONFile[bridgesdk.InitializeMarker](t, env.HandshakePath)
	if got, want := handshake.Request.Runtime.Bridge.Provider, "whatsapp"; got != want {
		t.Fatalf("handshake provider = %q, want %q", got, want)
	}
	ownership := waitForJSONFile[bridgesdk.OwnershipMarker](t, env.OwnershipPath)
	if got, want := len(ownership.Fetched), 2; got != want {
		t.Fatalf("len(ownership.Fetched) = %d, want %d", got, want)
	}
	states := waitForJSONLinesFile[bridgesdk.StateMarker](
		t,
		env.StatePath,
		func(items []bridgesdk.StateMarker) bool { return len(items) >= 2 },
	)
	if got, want := states[0].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("states[0].Status = %q, want %q", got, want)
	}
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})
}

func TestWebhookIngressRejectsInvalidSignatureAndIngestsMessage(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newWhatsAppAPIServer(t)
	t.Setenv(whatsappListenAddrEnv, listenAddr)
	t.Setenv(whatsappAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	mustHandleLifecycle(t, hostPeer, managed)

	var ingested []bridgepkg.InboundMessageEnvelope
	var mu sync.Mutex
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var envelope bridgepkg.InboundMessageEnvelope
			if err := json.Unmarshal(params, &envelope); err != nil {
				return nil, err
			}
			mu.Lock()
			ingested = append(ingested, envelope)
			mu.Unlock()
			return bridgepkg.BridgesMessagesIngestResult{
				SessionID:    "sess-1",
				RouteCreated: true,
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            envelope.Scope,
					WorkspaceID:      envelope.WorkspaceID,
					BridgeInstanceID: envelope.BridgeInstanceID,
					PeerID:           envelope.PeerID,
				},
			}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})

	serverAddr := runtime.http.Address()
	webhookURL := "http://" + serverAddr + "/whatsapp/brg-1"
	body := whatsappWebhookPayloadForPhone("123456789", "Need a summary")

	invalidReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(invalid) error = %v", err)
	}
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set(whatsappSignatureHeader, signWhatsAppBody([]byte(body), "wrong-secret"))
	invalidResp, err := http.DefaultClient.Do(invalidReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(invalid) error = %v", err)
	}
	defer func() {
		if closeErr := invalidResp.Body.Close(); closeErr != nil {
			t.Fatalf("invalidResp.Body.Close() error = %v", closeErr)
		}
	}()
	if got, want := invalidResp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("invalid webhook status = %d, want %d", got, want)
	}

	validReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(valid) error = %v", err)
	}
	validReq.Header.Set("Content-Type", "application/json")
	validReq.Header.Set(whatsappSignatureHeader, signWhatsAppBody([]byte(body), "app-secret"))
	validResp, err := http.DefaultClient.Do(validReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(valid) error = %v", err)
	}
	defer func() {
		if closeErr := validResp.Body.Close(); closeErr != nil {
			t.Fatalf("validResp.Body.Close() error = %v", closeErr)
		}
	}()
	if got, want := validResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("valid webhook status = %d, want %d", got, want)
	}

	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool {
			return len(items) == 1 && strings.TrimSpace(items[0].Result.SessionID) != ""
		},
	)
	if got, want := ingests[0].Envelope.PeerID, "15551234567"; got != want {
		t.Fatalf("ingests[0].Envelope.PeerID = %q, want %q", got, want)
	}
	if got, want := ingests[0].Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("ingests[0].Envelope.Content.Text = %q, want %q", got, want)
	}
	mu.Lock()
	if got, want := len(ingested), 1; got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
	mu.Unlock()

	verifyReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://"+serverAddr+"/whatsapp/brg-1?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=42",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(verify challenge) error = %v", err)
	}
	verifyResp, err := http.DefaultClient.Do(verifyReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(verify challenge) error = %v", err)
	}
	defer func() {
		if closeErr := verifyResp.Body.Close(); closeErr != nil {
			t.Fatalf("verifyResp.Body.Close() error = %v", closeErr)
		}
	}()
	verifyBody, err := io.ReadAll(verifyResp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(verify challenge) error = %v", err)
	}
	if got, want := verifyResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("verifyResp.StatusCode = %d, want %d", got, want)
	}
	if got, want := strings.TrimSpace(string(verifyBody)), "42"; got != want {
		t.Fatalf("verify challenge body = %q, want %q", got, want)
	}
}

func TestRuntimeDeliveriesCallWhatsAppGraphAPI(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newWhatsAppAPIServer(t)
	t.Setenv(whatsappListenAddrEnv, listenAddr)
	t.Setenv(whatsappAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 10, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	mustHandleLifecycle(t, hostPeer, managed)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	states := waitForJSONLinesFile[bridgesdk.StateMarker](
		t,
		env.StatePath,
		func(items []bridgesdk.StateMarker) bool { return len(items) >= 1 },
	)
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("states[last].Status = %q, want %q", got, want)
	}

	var startAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest("brg-1", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false, "hello"),
		&startAck,
	); err != nil {
		t.Fatalf("hostPeer.Call(start delivery) error = %v", err)
	}
	var finalAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest("brg-1", "delivery-1", 2, bridgepkg.DeliveryEventTypeFinal, true, "hello world"),
		&finalAck,
	); err != nil {
		t.Fatalf("hostPeer.Call(final delivery) error = %v", err)
	}

	records := waitForJSONLinesFile[bridgesdk.DeliveryMarker](
		t,
		env.DeliveryPath,
		func(items []bridgesdk.DeliveryMarker) bool { return len(items) >= 2 },
	)
	if records[0].Ack == nil || records[1].Ack == nil {
		t.Fatalf("delivery markers = %#v, want recorded acks", records)
	}
	if got, want := finalAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("finalAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}

	calls := mockAPI.Calls()
	if got, want := len(calls), 3; got != want {
		t.Fatalf("len(mockAPI calls) = %d, want %d (identity + send + send)", got, want)
	}
	if got, want := calls[0].Path, "/"+whatsappDefaultAPIVersion+"/123456789"; got != want {
		t.Fatalf("calls[0].Path = %q, want %q", got, want)
	}
	if got, want := calls[1].Body["to"], "15551234567"; got != want {
		t.Fatalf("calls[1].Body[to] = %#v, want %q", calls[1].Body["to"], want)
	}
	if got, want := calls[2].Body["type"], "text"; got != want {
		t.Fatalf("calls[2].Body[type] = %#v, want %q", calls[2].Body["type"], want)
	}

	sendErr := errors.New("cloud unavailable")
	runtime.apiFactory = func(resolvedInstanceConfig) whatsappAPI {
		return fakeWhatsAppAPIError{err: sendErr}
	}
	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest("brg-1", "delivery-failed", 1, bridgepkg.DeliveryEventTypeStart, false, "hello"),
		&bridgepkg.DeliveryAck{},
	); err == nil {
		t.Fatal("hostPeer.Call(failed delivery) error = nil, want non-nil")
	}
	records = waitForJSONLinesFile[bridgesdk.DeliveryMarker](
		t,
		env.DeliveryPath,
		func(items []bridgesdk.DeliveryMarker) bool { return len(items) >= 3 },
	)
	if records[2].Ack != nil || !strings.Contains(records[2].Error, sendErr.Error()) {
		t.Fatalf("failed delivery marker = %#v, want error without ack", records[2])
	}
}

func TestDispatchInboundBatchAndShutdown(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newWhatsAppAPIServer(t)
	t.Setenv(whatsappListenAddrEnv, listenAddr)
	t.Setenv(whatsappAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 12, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	mustHandleLifecycle(t, hostPeer, managed)

	var ingested []bridgepkg.InboundMessageEnvelope
	var mu sync.Mutex
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var envelope bridgepkg.InboundMessageEnvelope
			if err := json.Unmarshal(params, &envelope); err != nil {
				return nil, err
			}
			mu.Lock()
			ingested = append(ingested, envelope)
			mu.Unlock()
			return bridgepkg.BridgesMessagesIngestResult{
				SessionID:    "sess-1",
				RouteCreated: true,
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            envelope.Scope,
					WorkspaceID:      envelope.WorkspaceID,
					BridgeInstanceID: envelope.BridgeInstanceID,
					PeerID:           envelope.PeerID,
					ThreadID:         envelope.ThreadID,
					GroupID:          envelope.GroupID,
				},
			}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	waitForCondition(t, func() bool {
		_, err := runtime.configForInstance("brg-1")
		return err == nil
	})

	batch := bridgesdk.InboundBatch{
		Key: "batch-key",
		Items: []bridgepkg.InboundMessageEnvelope{
			{
				BridgeInstanceID:  "brg-1",
				Scope:             bridgepkg.ScopeWorkspace,
				WorkspaceID:       "ws-whatsapp",
				PlatformMessageID: "wamid.1",
				ReceivedAt:        now,
				PeerID:            "15551234567",
				Sender: bridgepkg.MessageSender{
					ID:          "15551234567",
					DisplayName: "Alice",
				},
				Content:        bridgepkg.MessageContent{Text: "first"},
				EventFamily:    bridgepkg.InboundEventFamilyMessage,
				IdempotencyKey: "whatsapp:brg-1:wamid.1",
			},
			{
				BridgeInstanceID:  "brg-1",
				Scope:             bridgepkg.ScopeWorkspace,
				WorkspaceID:       "ws-whatsapp",
				PlatformMessageID: "wamid.2",
				ReceivedAt:        now.Add(time.Second),
				PeerID:            "15551234567",
				Sender: bridgepkg.MessageSender{
					ID:          "15551234567",
					DisplayName: "Alice",
				},
				Content:        bridgepkg.MessageContent{Text: "second"},
				EventFamily:    bridgepkg.InboundEventFamilyMessage,
				IdempotencyKey: "whatsapp:brg-1:wamid.2",
			},
		},
		CreatedAt: now,
		UpdatedAt: now.Add(time.Second),
	}
	if err := runtime.dispatchInboundBatch(context.Background(), "brg-1", batch); err != nil {
		t.Fatalf("dispatchInboundBatch() error = %v", err)
	}

	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingested) == 1
	})
	mu.Lock()
	merged := ingested[0]
	mu.Unlock()
	if got, want := merged.Content.Text, "first\nsecond"; got != want {
		t.Fatalf("merged.Content.Text = %q, want %q", got, want)
	}
	if got, want := merged.IdempotencyKey, "whatsapp:brg-1:wamid.1:batch:2"; got != want {
		t.Fatalf("merged.IdempotencyKey = %q, want %q", got, want)
	}

	if err := runtime.lifecycle.Shutdown(
		context.Background(),
		nil,
		subprocess.ShutdownRequest{DeadlineMS: 50},
	); err != nil {
		t.Fatalf("handleShutdown() error = %v", err)
	}
	lines := waitForNonEmptyLines(t, env.ShutdownPath)
	if len(lines) == 0 || !strings.Contains(lines[0], "pid=") {
		t.Fatalf("shutdown marker lines = %#v, want pid entry", lines)
	}
}

func TestHandleWebhookRequestValidationAndBatching(t *testing.T) {
	t.Parallel()

	provider, err := newWhatsAppProvider(io.Discard)
	if err != nil {
		t.Fatalf("newWhatsAppProvider() error = %v", err)
	}

	managed := testBridgeRuntime(time.Date(2026, 4, 15, 13, 20, 0, 0, time.UTC), "brg-1")
	cfg := resolvedInstanceConfig{
		managed:       &managed,
		instanceID:    "brg-1",
		phoneNumberID: "123456789",
		dmPolicy:      bridgepkg.BridgeDMPolicyAllowlist,
		allowUserIDs:  map[string]struct{}{"15551234567": {}},
		dedup:         bridgesdk.NewDedupCache(5*time.Minute, 32),
	}
	var batches []bridgesdk.InboundBatch
	cfg.batcher, err = bridgesdk.NewInboundBatcher(bridgesdk.InboundBatcherConfig{
		Delay: 0,
		Dispatch: func(_ context.Context, batch bridgesdk.InboundBatch) error {
			batches = append(batches, batch)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewInboundBatcher() error = %v", err)
	}
	defer cfg.batcher.Close()

	rec := httptest.NewRecorder()
	if err := provider.handleWebhookRequest(rec, nil, cfg, bridgesdk.WebhookRequest{Body: []byte("{")}); err == nil {
		t.Fatal("handleWebhookRequest(invalid payload) error = nil, want non-nil")
	}

	body := []byte(
		`{"object":"whatsapp_business_account","entry":[{"changes":[{"field":"statuses","value":{}},{"field":"messages","value":{"metadata":{"phone_number_id":"123456789"},"contacts":[{"profile":{"name":"Alice Example"},"wa_id":"15551234567"},{"profile":{"name":"Blocked User"},"wa_id":"16667778888"}],"messages":[{"from":"15551234567","id":"wamid.allowed","timestamp":"1775866800","type":"text","text":{"body":"hello"}},{"from":"16667778888","id":"wamid.blocked","timestamp":"1775866801","type":"text","text":{"body":"blocked"}}]}},{"field":"messages","value":{"metadata":{"phone_number_id":"999999999"},"messages":[{"from":"15551234567","id":"wamid.other","timestamp":"1775866802","type":"text","text":{"body":"wrong phone"}}]}}]}]}`,
	)
	req := bridgesdk.WebhookRequest{
		Body:       body,
		ReceivedAt: time.Date(2026, 4, 15, 13, 20, 0, 0, time.UTC),
	}
	rec = httptest.NewRecorder()
	if err := provider.handleWebhookRequest(rec, nil, cfg, req); err != nil {
		t.Fatalf("handleWebhookRequest(valid) error = %v", err)
	}
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("handleWebhookRequest(valid) status = %d, want %d", got, want)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), "ok"; got != want {
		t.Fatalf("handleWebhookRequest(valid) body = %q, want %q", got, want)
	}
	if got, want := len(batches), 1; got != want {
		t.Fatalf("len(batches) = %d, want %d", got, want)
	}
	if got, want := len(batches[0].Items), 1; got != want {
		t.Fatalf("len(batches[0].Items) = %d, want %d", got, want)
	}
	if got, want := batches[0].Items[0].PeerID, "15551234567"; got != want {
		t.Fatalf("batches[0].Items[0].PeerID = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	if err := provider.handleWebhookRequest(rec, nil, cfg, req); err != nil {
		t.Fatalf("handleWebhookRequest(duplicate) error = %v", err)
	}
	if got, want := len(batches), 1; got != want {
		t.Fatalf("len(batches) after duplicate = %d, want %d", got, want)
	}
}

func TestRejectMisconfiguredRoutes(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectInvalidInstanceDeliveryBeforeAPICall", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}

		badConfig := errors.New("whatsapp: provider_config.phone_number_id is required")
		apiCalled := false
		provider.apiFactory = func(resolvedInstanceConfig) whatsappAPI {
			apiCalled = true
			return &fakeWhatsAppAPI{}
		}

		setWhatsAppProviderRoute(provider, "brg-1", resolvedInstanceConfig{
			instanceID:  "brg-1",
			configError: badConfig,
		})

		ack, err := provider.handleBridgesDeliver(
			context.Background(),
			nil,
			testDeliveryRequest(
				"brg-1",
				"delivery-1",
				1,
				bridgepkg.DeliveryEventTypeStart,
				false,
				"hello",
			),
		)
		if err == nil {
			t.Fatal("handleBridgesDeliver() error = nil, want non-nil")
		}
		if !errors.Is(err, errWhatsAppInstanceConfigInvalid) {
			t.Fatalf("handleBridgesDeliver() error = %v, want invalid-config sentinel", err)
		}
		if !errors.Is(err, badConfig) {
			t.Fatalf("handleBridgesDeliver() error = %v, want wrapped config error", err)
		}
		if ack != (bridgepkg.DeliveryAck{}) {
			t.Fatalf("handleBridgesDeliver() ack = %#v, want zero value", ack)
		}
		if apiCalled {
			t.Fatal("handleBridgesDeliver() called Graph API for invalid config")
		}
	})

	t.Run("ShouldFailClosedForDuplicateWebhookPaths", func(t *testing.T) {
		t.Parallel()

		provider, err := newWhatsAppProvider(io.Discard)
		if err != nil {
			t.Fatalf("newWhatsAppProvider() error = %v", err)
		}

		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-1": {
				instanceID:  "brg-1",
				webhookPath: "/whatsapp/shared",
				verifyToken: "verify-1",
			},
			"brg-2": {
				instanceID:  "brg-2",
				webhookPath: "/whatsapp/shared",
				verifyToken: "verify-2",
				configError: errors.New("whatsapp: webhook path \"/whatsapp/shared\" is shared"),
			},
		}, nil)

		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"http://example.test/whatsapp/shared?hub.mode=subscribe&hub.verify_token=verify-1&hub.challenge=42",
			http.NoBody,
		)
		resp := httptest.NewRecorder()
		provider.serveWebhookHTTP(resp, req)
		if got, want := resp.Code, http.StatusNotFound; got != want {
			t.Fatalf("serveWebhookHTTP() status = %d, want %d", got, want)
		}
	})
}

func TestHealthHelpers(t *testing.T) {
	t.Parallel()

	provider, err := newWhatsAppProvider(io.Discard)
	if err != nil {
		t.Fatalf("newWhatsAppProvider() error = %v", err)
	}
	provider.setLastError(errors.New("boom"))
	if err := provider.healthCheck(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("healthCheck() error = %v, want boom", err)
	}
	provider.clearLastError()
	if err := provider.healthCheck(); err != nil {
		t.Fatalf("healthCheck(clear) error = %v", err)
	}
}

func TestExtractWhatsAppTextContentAndAttachments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		message         whatsappInboundMessage
		wantText        string
		wantAttachments int
		validate        func(*testing.T, []bridgepkg.MessageAttachment)
	}{
		{
			name: "text",
			message: whatsappInboundMessage{
				Type: "text",
				Text: &struct {
					Body string `json:"body,omitempty"`
				}{Body: "hello"},
			},
			wantText: "hello",
		},
		{
			name: "image with caption",
			message: whatsappInboundMessage{
				Type: "image",
				Image: &struct {
					ID       string `json:"id,omitempty"`
					MIMEType string `json:"mime_type,omitempty"`
					Caption  string `json:"caption,omitempty"`
					SHA256   string `json:"sha256,omitempty"`
				}{ID: "img-1", MIMEType: "image/jpeg", Caption: "look"},
			},
			wantText:        "look",
			wantAttachments: 1,
			validate: func(t *testing.T, attachments []bridgepkg.MessageAttachment) {
				t.Helper()
				if got, want := attachments[0].ID, "img-1"; got != want {
					t.Fatalf("attachments[0].ID = %q, want %q", got, want)
				}
			},
		},
		{
			name: "document fallback",
			message: whatsappInboundMessage{
				Type: "document",
				Document: &struct {
					ID       string `json:"id,omitempty"`
					MIMEType string `json:"mime_type,omitempty"`
					Caption  string `json:"caption,omitempty"`
					Filename string `json:"filename,omitempty"`
					SHA256   string `json:"sha256,omitempty"`
				}{ID: "doc-1", MIMEType: "application/pdf", Filename: "report.pdf"},
			},
			wantText:        "[Document: report.pdf]",
			wantAttachments: 1,
		},
		{
			name: "audio",
			message: whatsappInboundMessage{
				Type: "audio",
				Audio: &struct {
					ID       string `json:"id,omitempty"`
					MIMEType string `json:"mime_type,omitempty"`
					Voice    bool   `json:"voice,omitempty"`
					SHA256   string `json:"sha256,omitempty"`
				}{ID: "aud-1", MIMEType: "audio/ogg"},
			},
			wantText:        "[Audio message]",
			wantAttachments: 1,
		},
		{
			name: "video fallback",
			message: whatsappInboundMessage{
				Type: "video",
				Video: &struct {
					ID       string `json:"id,omitempty"`
					MIMEType string `json:"mime_type,omitempty"`
					Caption  string `json:"caption,omitempty"`
					SHA256   string `json:"sha256,omitempty"`
				}{ID: "vid-1", MIMEType: "video/mp4"},
			},
			wantText:        "[Video]",
			wantAttachments: 1,
		},
		{
			name: "sticker",
			message: whatsappInboundMessage{
				Type: "sticker",
				Sticker: &struct {
					ID       string `json:"id,omitempty"`
					MIMEType string `json:"mime_type,omitempty"`
					Animated bool   `json:"animated,omitempty"`
					SHA256   string `json:"sha256,omitempty"`
				}{ID: "stk-1", MIMEType: "image/webp"},
			},
			wantText:        "[Sticker]",
			wantAttachments: 1,
		},
		{
			name: "location with fallback map url",
			message: whatsappInboundMessage{
				Type: "location",
				Location: &struct {
					Latitude  float64 `json:"latitude,omitempty"`
					Longitude float64 `json:"longitude,omitempty"`
					Name      string  `json:"name,omitempty"`
					Address   string  `json:"address,omitempty"`
					URL       string  `json:"url,omitempty"`
				}{Latitude: -23.5, Longitude: -46.6, Name: "HQ", Address: "Rua 1"},
			},
			wantText:        "[Location: HQ - Rua 1]",
			wantAttachments: 1,
			validate: func(t *testing.T, attachments []bridgepkg.MessageAttachment) {
				t.Helper()
				if got := attachments[0].URL; !strings.Contains(got, "google.com/maps") {
					t.Fatalf("attachments[0].URL = %q, want google maps fallback", got)
				}
			},
		},
		{
			name: "unsupported",
			message: whatsappInboundMessage{
				Type: "contacts",
			},
			wantText: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got, want := extractWhatsAppTextContent(tc.message), tc.wantText; got != want {
				t.Fatalf("extractWhatsAppTextContent() = %q, want %q", got, want)
			}
			attachments := normalizeWhatsAppAttachments(tc.message)
			if got, want := len(attachments), tc.wantAttachments; got != want {
				t.Fatalf("len(normalizeWhatsAppAttachments()) = %d, want %d", got, want)
			}
			if tc.validate != nil {
				tc.validate(t, attachments)
			}
		})
	}
}

func TestWhatsAppGraphClientMethods(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer access-token"; got != want {
			t.Fatalf("authorization header = %q, want %q", got, want)
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v99.0/123456789":
			if err := json.NewEncoder(w).Encode(map[string]any{"id": "123456789"}); err != nil {
				t.Errorf("encode phone response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/v99.0/123456789/messages":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("json.Decode(payload) error = %v", err)
			}
			if got, want := payload["to"], "15551234567"; got != want {
				t.Fatalf("payload[to] = %#v, want %q", got, want)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"messages": []map[string]any{{"id": "wamid.graph"}},
			}); err != nil {
				t.Errorf("encode send response: %v", err)
			}
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := &whatsappGraphClient{
		baseURL:     server.URL,
		apiVersion:  "v99.0",
		accessToken: "access-token",
		httpClient:  server.Client(),
	}

	phone, err := client.GetPhoneNumber(context.Background(), "123456789")
	if err != nil {
		t.Fatalf("GetPhoneNumber() error = %v", err)
	}
	if got, want := phone.ID, "123456789"; got != want {
		t.Fatalf("phone.ID = %q, want %q", got, want)
	}

	resp, err := client.SendTextMessage(
		context.Background(),
		"123456789",
		whatsappSendMessageRequest{
			MessagingProduct: "whatsapp",
			RecipientType:    "individual",
			To:               "15551234567",
			Type:             "text",
			Text: struct {
				Body       string `json:"body"`
				PreviewURL bool   `json:"preview_url"`
			}{
				Body:       "hello",
				PreviewURL: false,
			},
		},
	)
	if err != nil {
		t.Fatalf("SendTextMessage() error = %v", err)
	}
	if got, want := resp.Messages[0].ID, "wamid.graph"; got != want {
		t.Fatalf("resp.Messages[0].ID = %q, want %q", got, want)
	}

	t.Run("Should not retry a send after a successful status with a truncated result", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			if _, err := w.Write([]byte(`{"messages":[{"id":`)); err != nil {
				t.Errorf("write truncated WhatsApp response: %v", err)
			}
		}))
		defer server.Close()

		client := &whatsappGraphClient{
			baseURL:     server.URL,
			apiVersion:  "v99.0",
			accessToken: "access-token",
			httpClient:  server.Client(),
		}
		_, err := sendWhatsAppDeliveryMessage(
			t.Context(),
			client,
			"123456789",
			whatsappSendMessageRequest{MessagingProduct: "whatsapp", To: "15551234567", Type: "text"},
		)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("sendWhatsAppDeliveryMessage() error = %T %v, want CommittedMutationError", err, err)
		}
		if got, want := attempts.Load(), int32(1); got != want {
			t.Fatalf("send attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should continue a recoverable send when retry-state reporting fails", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		var reportFailures atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if attempts.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				if _, err := w.Write([]byte(`{"error":{"message":"temporary outage","code":2}}`)); err != nil {
					t.Errorf("write transient WhatsApp response: %v", err)
				}
				return
			}
			if _, err := w.Write([]byte(`{"messages":[{"id":"wamid.recovered"}]}`)); err != nil {
				t.Errorf("write recovered WhatsApp response: %v", err)
			}
		}))
		defer server.Close()

		client := &whatsappGraphClient{
			baseURL:     server.URL,
			apiVersion:  "v99.0",
			accessToken: "access-token",
			httpClient:  server.Client(),
			reportRetry: func(context.Context, bridgesdk.RetryAttempt) error {
				return errors.New("host api temporarily unavailable")
			},
			reportRetryFailure: func(error) {
				reportFailures.Add(1)
			},
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		response, err := sendWhatsAppDeliveryMessage(
			ctx,
			client,
			"123456789",
			whatsappSendMessageRequest{MessagingProduct: "whatsapp", To: "15551234567", Type: "text"},
		)
		if err != nil {
			t.Fatalf("sendWhatsAppDeliveryMessage() error = %v", err)
		}
		if got, want := response.Messages[0].ID, "wamid.recovered"; got != want {
			t.Fatalf("message id = %q, want %q", got, want)
		}
		if got, want := attempts.Load(), int32(2); got != want {
			t.Fatalf("send attempts = %d, want %d", got, want)
		}
		if got, want := reportFailures.Load(), int32(1); got != want {
			t.Fatalf("retry report failures = %d, want %d", got, want)
		}
	})

	t.Run("Should preserve auth classification when the error body read fails", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("WhatsApp error body read failed")
		body := &whatsappPartialErrorResponseBody{
			content: []byte(`{"error":{"message":"expired access token","code":190}}`),
			readErr: readErr,
		}
		client := &whatsappGraphClient{
			baseURL:     "https://graph.facebook.test",
			apiVersion:  "v99.0",
			accessToken: "access-token",
			httpClient: &http.Client{Transport: whatsappErrorResponseTransport{
				statusCode: http.StatusUnauthorized,
				headers:    make(http.Header),
				body:       body,
			}},
		}

		_, err := client.SendTextMessage(
			t.Context(),
			"123456789",
			whatsappSendMessageRequest{MessagingProduct: "whatsapp", To: "15551234567", Type: "text"},
		)
		if !errors.Is(err, readErr) {
			t.Fatalf("SendTextMessage() error = %v, want response body read failure", err)
		}
		var authErr *bridgesdk.AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("SendTextMessage() error = %T %v, want AuthError", err, err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassAuth; got != want {
			t.Fatalf("SendTextMessage() error class = %q, want %q", got, want)
		}
		if !strings.Contains(authErr.Error(), "expired access token") {
			t.Fatalf("SendTextMessage() auth error = %q, want partial provider body", authErr.Error())
		}
		if bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("SendTextMessage() error = %v, want pre-commit rejection", err)
		}
		if !body.closed {
			t.Fatal("response body closed = false after error classification")
		}
	})

	for _, testCase := range []struct {
		name       string
		statusCode int
	}{
		{name: "Should refuse a credentialed 301 redirect", statusCode: http.StatusMovedPermanently},
		{name: "Should refuse a credentialed 302 redirect", statusCode: http.StatusFound},
		{name: "Should refuse a credentialed 303 redirect", statusCode: http.StatusSeeOther},
		{name: "Should refuse a credentialed 307 redirect", statusCode: http.StatusTemporaryRedirect},
		{name: "Should refuse a credentialed 308 redirect", statusCode: http.StatusPermanentRedirect},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var targetHits atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				targetHits.Add(1)
				if _, err := io.WriteString(w, `{"messages":[{"id":"wamid.redirect"}]}`); err != nil {
					t.Errorf("write redirect target WhatsApp response: %v", err)
				}
			}))
			defer target.Close()

			redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(testCase.statusCode)
				if _, err := io.WriteString(w, `{"error":{"message":"redirect refused"}}`); err != nil {
					t.Errorf("write redirect WhatsApp response: %v", err)
				}
				if got, want := r.Method, http.MethodPost; got != want {
					t.Errorf("redirect source method = %q, want %q", got, want)
				}
			}))
			defer redirect.Close()

			client := &whatsappGraphClient{
				baseURL:     redirect.URL,
				apiVersion:  "v99.0",
				accessToken: "access-token",
				httpClient:  redirect.Client(),
			}
			_, err := client.SendTextMessage(
				t.Context(),
				"123456789",
				whatsappSendMessageRequest{MessagingProduct: "whatsapp", To: "15551234567", Type: "text"},
			)
			var httpErr *bridgesdk.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != testCase.statusCode {
				t.Fatalf(
					"SendTextMessage(%d) error = %T %v, want first-response HTTPError",
					testCase.statusCode,
					err,
					err,
				)
			}
			if bridgesdk.IsCommittedMutation(err) {
				t.Fatalf("SendTextMessage(%d) error = %v, want pre-commit rejection", testCase.statusCode, err)
			}
			if got := targetHits.Load(); got != 0 {
				t.Fatalf("redirect target hits = %d, want 0", got)
			}
		})
	}

	t.Run("Should keep an invalid phone-number read response outside committed mutation handling", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte(`not-json`)); err != nil {
				t.Errorf("write malformed WhatsApp response: %v", err)
			}
		}))
		defer server.Close()

		client := &whatsappGraphClient{
			baseURL:     server.URL,
			apiVersion:  "v99.0",
			accessToken: "access-token",
			httpClient:  server.Client(),
		}
		_, err := client.GetPhoneNumber(t.Context(), "123456789")
		if err == nil {
			t.Fatal("GetPhoneNumber() error = nil, want decode error")
		}
		if committedErr, ok := errors.AsType[*bridgesdk.CommittedMutationError](err); ok && committedErr != nil {
			t.Fatalf("GetPhoneNumber() error = %T %v, want non-committed error", err, err)
		}
	})
}

type whatsappErrorResponseTransport struct {
	statusCode int
	headers    http.Header
	body       io.ReadCloser
}

func (t whatsappErrorResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.statusCode,
		Header:     t.headers,
		Body:       t.body,
		Request:    req,
	}, nil
}

type whatsappPartialErrorResponseBody struct {
	content []byte
	readErr error
	reads   int
	closed  bool
}

func (b *whatsappPartialErrorResponseBody) Read(buffer []byte) (int, error) {
	b.reads++
	if b.reads > 1 {
		return 0, io.EOF
	}
	return copy(buffer, b.content), b.readErr
}

func (b *whatsappPartialErrorResponseBody) Close() error {
	b.closed = true
	return nil
}

func TestRunHelpers(t *testing.T) {
	t.Parallel()

	if err := run([]string{"bad"}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("run(unsupported) error = nil, want non-nil")
	}
}

type fakeWhatsAppAPI struct {
	nextMessageID int
	requests      []whatsappSendMessageRequest
	attempts      []whatsappSendMessageRequest
	sendErrorText string
	sendErr       error
}

type whatsappDeliveryResponseAPI struct {
	response *whatsappSendMessageResponse
	sendErr  error
	requests []whatsappSendMessageRequest
}

func (f *whatsappDeliveryResponseAPI) GetPhoneNumber(
	context.Context,
	string,
) (*whatsappPhoneNumber, error) {
	return &whatsappPhoneNumber{ID: "123456789"}, nil
}

func (f *whatsappDeliveryResponseAPI) ReportRetry(
	context.Context,
	bridgesdk.RetryAttempt,
) {
}

func (f *whatsappDeliveryResponseAPI) SendTextMessage(
	_ context.Context,
	_ string,
	request whatsappSendMessageRequest,
) (*whatsappSendMessageResponse, error) {
	f.requests = append(f.requests, request)
	return f.response, f.sendErr
}

func (f *fakeWhatsAppAPI) GetPhoneNumber(context.Context, string) (*whatsappPhoneNumber, error) {
	return &whatsappPhoneNumber{ID: "123456789"}, nil
}

func (f *fakeWhatsAppAPI) ReportRetry(context.Context, bridgesdk.RetryAttempt) {}

func (f *fakeWhatsAppAPI) SendTextMessage(
	_ context.Context,
	_ string,
	req whatsappSendMessageRequest,
) (*whatsappSendMessageResponse, error) {
	f.attempts = append(f.attempts, req)
	if f.sendErr != nil && (f.sendErrorText == "" || req.Text.Body == f.sendErrorText) {
		return nil, f.sendErr
	}
	f.requests = append(f.requests, req)
	messageID := fmt.Sprintf("wamid.%d", f.nextMessageID)
	f.nextMessageID++
	return &whatsappSendMessageResponse{
		Messages: []struct {
			ID string `json:"id,omitempty"`
		}{
			{ID: messageID},
		},
	}, nil
}

func countWhatsAppSendRequests(requests []whatsappSendMessageRequest, text string) int {
	count := 0
	for _, request := range requests {
		if request.Text.Body == text {
			count++
		}
	}
	return count
}

func testWhatsAppResumeRequest(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryRequest {
	resume := request
	resume.Event.EventType = bridgepkg.DeliveryEventTypeResume
	resume.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeFinal}
	resume.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       request.Event.DeliveryID,
		SessionID:        "sess-resume",
		TurnID:           "turn-resume",
		BridgeInstanceID: request.Event.BridgeInstanceID,
		RoutingKey:       request.Event.RoutingKey,
		DeliveryTarget:   request.Event.DeliveryTarget,
		LatestSeq:        request.Event.Seq,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   request.Event.Content,
		Operation:        request.Event.Operation,
		Final:            true,
		UpdatedAt:        time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
	}
	return resume
}

type fakeWhatsAppAPIError struct {
	err error
}

func (f fakeWhatsAppAPIError) GetPhoneNumber(
	context.Context,
	string,
) (*whatsappPhoneNumber, error) {
	return nil, f.err
}

func (fakeWhatsAppAPIError) ReportRetry(context.Context, bridgesdk.RetryAttempt) {}

func (f fakeWhatsAppAPIError) SendTextMessage(
	context.Context,
	string,
	whatsappSendMessageRequest,
) (*whatsappSendMessageResponse, error) {
	return nil, f.err
}

type whatsappAPIServer struct {
	server        *httptest.Server
	mu            sync.Mutex
	calls         []whatsappAPICall
	nextMessageID int
}

type whatsappAPICall struct {
	Path string
	Body map[string]any
}

func newWhatsAppAPIServer(t *testing.T) *whatsappAPIServer {
	t.Helper()

	srv := &whatsappAPIServer{nextMessageID: 700}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				t.Errorf("decode WhatsApp API request: %v", err)
			}
		}

		srv.mu.Lock()
		srv.calls = append(srv.calls, whatsappAPICall{Path: r.URL.Path, Body: body})
		srv.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/123456789"):
			if err := json.NewEncoder(w).Encode(map[string]any{"id": "123456789"}); err != nil {
				t.Errorf("encode WhatsApp phone response: %v", err)
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/123456789/messages"):
			srv.mu.Lock()
			messageID := fmt.Sprintf("wamid.%d", srv.nextMessageID)
			srv.nextMessageID++
			srv.mu.Unlock()
			if err := json.NewEncoder(w).Encode(map[string]any{
				"messages": []map[string]any{{"id": messageID}},
			}); err != nil {
				t.Errorf("encode WhatsApp send response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "unknown method",
					"code":    http.StatusNotFound,
				},
			}); err != nil {
				t.Errorf("encode unknown WhatsApp method response: %v", err)
			}
		}
	}))
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *whatsappAPIServer) URL() string {
	return s.server.URL
}

func (s *whatsappAPIServer) Calls() []whatsappAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := make([]whatsappAPICall, len(s.calls))
	copy(cloned, s.calls)
	return cloned
}

func newRuntimePeerPair(t *testing.T) (*whatsappProvider, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	runtime, err := newWhatsAppProvider(io.Discard)
	if err != nil {
		t.Fatalf("newWhatsAppProvider() error = %v", err)
	}

	hostPeer := bridgesdk.NewPeer(hostConn, hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 2)
	go func() { errCh <- runtime.serve(runtimeConn, runtimeConn) }()
	go func() { errCh <- hostPeer.Serve(ctx) }()

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			cancel()
			runtime.lifecycle.Stop()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := runtime.http.Shutdown(shutdownCtx); err != nil {
				t.Fatalf("provider HTTP shutdown error = %v", err)
			}
			shutdownCancel()
			if err := hostConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("host connection close error = %v", err)
			}
			if err := runtimeConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("runtime connection close error = %v", err)
			}
			for range 2 {
				err := <-errCh
				if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
					continue
				}
				if strings.Contains(err.Error(), "closed") {
					continue
				}
				t.Fatalf("runtime peer serve error = %v", err)
			}
			if err := runtime.lifecycle.Wait(context.Background()); err != nil {
				t.Fatalf("provider lifecycle wait error = %v", err)
			}
		})
	}

	return runtime, hostPeer, cleanup
}

func mustHandle(t *testing.T, peer *bridgesdk.Peer, method string, handler bridgesdk.RPCHandler) {
	t.Helper()
	if err := peer.Handle(method, handler); err != nil {
		t.Fatalf("peer.Handle(%q) error = %v", method, err)
	}
}

func mustHandleLifecycle(
	t *testing.T,
	peer *bridgesdk.Peer,
	managed ...subprocess.InitializeBridgeManagedInstance,
) {
	t.Helper()

	mustHandle(
		t,
		peer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			instances := make([]bridgepkg.BridgeInstance, 0, len(managed))
			for _, item := range managed {
				instances = append(instances, item.Instance)
			}
			return instances, nil
		},
	)
	mustHandle(
		t,
		peer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgeInstanceTargetParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			for _, item := range managed {
				if item.Instance.ID == payload.BridgeInstanceID {
					return item.Instance, nil
				}
			}
			return nil, errors.New("unexpected instance")
		},
	)
	mustHandle(
		t,
		peer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			for _, item := range managed {
				if item.Instance.ID == payload.BridgeInstanceID {
					instance := item.Instance
					instance.Status = payload.Status
					instance.Degradation = payload.Degradation
					return instance, nil
				}
			}
			return nil, errors.New("unexpected state instance")
		},
	)
}

func setWhatsAppProviderRoute(
	provider *whatsappProvider,
	instanceID string,
	config resolvedInstanceConfig,
) {
	provider.routes.Replace(map[string]resolvedInstanceConfig{instanceID: config}, nil)
}

func testBridgeRuntime(
	now time.Time,
	instanceID string,
) subprocess.InitializeBridgeManagedInstance {
	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:            instanceID,
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-whatsapp",
			Platform:      "whatsapp",
			ExtensionName: "whatsapp",
			DisplayName:   "WhatsApp",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			ProviderConfig: []byte(`{
				"phone_number_id":"123456789"
			}`),
			CreatedAt: now,
			UpdatedAt: now,
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "access_token", Kind: "token", Value: "access-token"},
			{BindingName: "app_secret", Kind: "token", Value: "app-secret"},
			{BindingName: "verify_token", Kind: "token", Value: "verify-token"},
		},
	}
}

func testInitializeRequest(
	_ time.Time,
	managed ...subprocess.InitializeBridgeManagedInstance,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "0.5.0",
		SessionNonce:             "nonce-test",
		Extension: subprocess.InitializeExtension{
			Name:       "whatsapp",
			Version:    "0.1.0",
			SourceTier: "user",
		},
		Capabilities: subprocess.InitializeCapabilities{
			Provides: []string{"bridge.adapter"},
			GrantedActions: []extensionprotocol.HostAPIMethod{
				extensionprotocol.HostAPIMethodBridgesInstancesList,
				extensionprotocol.HostAPIMethodBridgesInstancesGet,
				extensionprotocol.HostAPIMethodBridgesInstancesReportState,
				extensionprotocol.HostAPIMethodBridgesMessagesIngest,
			},
			GrantedSecurity: []string{"bridge.read", "bridge.write"},
		},
		Methods: subprocess.InitializeMethods{
			ExtensionServices: []string{"bridges/deliver", "health_check", "shutdown"},
		},
		Runtime: subprocess.InitializeRuntime{
			HealthCheckIntervalMS: 30_000,
			HealthCheckTimeoutMS:  5_000,
			ShutdownTimeoutMS:     5_000,
			DefaultHookTimeoutMS:  5_000,
			Bridge: &subprocess.InitializeBridgeRuntime{
				RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
				Purpose:          subprocess.BridgeRuntimePurposeService,
				Provider:         "whatsapp",
				Platform:         "whatsapp",
				ManagedInstances: managed,
			},
		},
	}
}

func testDeliveryRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	eventType bridgepkg.DeliveryEventType,
	final bool,
	text string,
) bridgepkg.DeliveryRequest {
	return bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       deliveryID,
			BridgeInstanceID: instanceID,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-whatsapp",
				BridgeInstanceID: instanceID,
				PeerID:           "15551234567",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instanceID,
				PeerID:           "15551234567",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       seq,
			EventType: eventType,
			Content:   bridgepkg.MessageContent{Text: text},
			Final:     final,
		},
	}
}

func testWhatsAppProgressRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	toolCallID string,
	toolID string,
	label string,
) bridgepkg.DeliveryRequest {
	request := testDeliveryRequest(
		instanceID,
		deliveryID,
		seq,
		bridgepkg.DeliveryEventTypeProgress,
		false,
		"",
	)
	request.Event.Progress = &bridgepkg.ToolProgress{
		ToolCallID: toolCallID,
		ToolID:     toolID,
		Phase:      bridgepkg.ToolProgressPhaseStarted,
		Label:      label,
		Index:      int(seq),
	}
	return request
}

func testDeleteRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	remoteMessageID string,
) bridgepkg.DeliveryRequest {
	req := testDeliveryRequest(
		instanceID,
		deliveryID,
		seq,
		bridgepkg.DeliveryEventTypeDelete,
		true,
		"",
	)
	req.Event.Operation = bridgepkg.DeliveryOperationDelete
	req.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteMessageID}
	return req
}

func whatsappWebhookPayloadForPhone(phoneNumberID string, text string) string {
	return fmt.Sprintf(
		`{"object":"whatsapp_business_account","entry":[{"id":"waba-1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"display_phone_number":"+15551234567","phone_number_id":%q},"contacts":[{"profile":{"name":"Alice Example"},"wa_id":"15551234567"}],"messages":[{"from":"15551234567","id":"wamid.abc123","timestamp":"1775866800","type":"text","text":{"body":%q}}]}}]}]}`,
		phoneNumberID,
		text,
	)
}

func signWhatsAppBody(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		panic(fmt.Sprintf("write WhatsApp signature body: %v", err))
	}
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func strconvTime(ts time.Time) string {
	return strconv.FormatInt(ts.Unix(), 10)
}

func TestWhatsAppResolvedConfigListenErrorsAndServe(t *testing.T) {
	t.Run("Should classify invalid webhook configuration and listener failures", func(t *testing.T) {
		t.Parallel()

		missingPath := resolvedInstanceConfig{}
		validateWhatsAppResolvedConfig(&missingPath)
		if missingPath.configError == nil || !strings.Contains(missingPath.configError.Error(), "webhook path") {
			t.Fatalf("missing path config error = %v", missingPath.configError)
		}
		missingPhone := resolvedInstanceConfig{webhookPath: "/whatsapp"}
		validateWhatsAppResolvedConfig(&missingPhone)
		if missingPhone.configError == nil || !strings.Contains(missingPhone.configError.Error(), "phone_number_id") {
			t.Fatalf("missing phone config error = %v", missingPhone.configError)
		}

		configs := []resolvedInstanceConfig{{instanceID: "brg-1"}}
		applyWhatsAppListenErrors(configs, "", func(string) error { return nil })
		if configs[0].configError == nil || !strings.Contains(configs[0].configError.Error(), "listen address") {
			t.Fatalf("missing listener config error = %v", configs[0].configError)
		}
		configs = []resolvedInstanceConfig{{instanceID: "brg-1"}}
		applyWhatsAppListenErrors(configs, "127.0.0.1:1", func(string) error { return errors.New("listen failed") })
		if configs[0].configError == nil || !strings.Contains(configs[0].configError.Error(), "listen failed") {
			t.Fatalf("startup config error = %v", configs[0].configError)
		}
	})

	t.Run("Should serve an empty command stream through the shared runner", func(t *testing.T) {
		t.Parallel()

		if err := runServe(strings.NewReader(""), io.Discard, io.Discard); err != nil {
			t.Fatalf("runServe(empty input) error = %v", err)
		}
	})
}

func setProviderTestEnv(t *testing.T) bridgesdk.AdapterMarkerPaths {
	t.Helper()

	root := filepath.Join(t.TempDir(), "markers")
	env := bridgesdk.AdapterMarkerPaths{
		HandshakePath: filepath.Join(root, "handshake.json"),
		OwnershipPath: filepath.Join(root, "ownership.json"),
		StatePath:     filepath.Join(root, "state.jsonl"),
		DeliveryPath:  filepath.Join(root, "delivery.jsonl"),
		IngestPath:    filepath.Join(root, "ingest.jsonl"),
		StartsPath:    filepath.Join(root, "starts.log"),
		ShutdownPath:  filepath.Join(root, "shutdown.log"),
		CrashOncePath: filepath.Join(root, "crash-once.json"),
	}

	t.Setenv(bridgesdk.AdapterHandshakePathEnv, env.HandshakePath)
	t.Setenv(bridgesdk.AdapterOwnershipPathEnv, env.OwnershipPath)
	t.Setenv(bridgesdk.AdapterStatePathEnv, env.StatePath)
	t.Setenv(bridgesdk.AdapterDeliveryPathEnv, env.DeliveryPath)
	t.Setenv(bridgesdk.AdapterIngestPathEnv, env.IngestPath)
	t.Setenv(bridgesdk.AdapterStartsPathEnv, env.StartsPath)
	t.Setenv(bridgesdk.AdapterShutdownPathEnv, env.ShutdownPath)
	t.Setenv(bridgesdk.AdapterCrashOncePathEnv, "")

	return env
}

func reserveListenAddr(t *testing.T) string {
	t.Helper()

	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listenConfig.Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("ln.Close() error = %v", err)
	}
	return addr
}

func waitForJSONFile[T any](t *testing.T, path string) T {
	t.Helper()

	var item T
	waitForCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		return json.Unmarshal(payload, &item) == nil
	})
	return item
}

func waitForJSONLinesFile[T any](t *testing.T, path string, predicate func([]T) bool) []T {
	t.Helper()

	var items []T
	waitForCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines := nonEmptyLines(string(payload))
		decoded := make([]T, 0, len(lines))
		for _, line := range lines {
			var item T
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				return false
			}
			decoded = append(decoded, item)
		}
		items = decoded
		return predicate(items)
	})
	return items
}

func waitForNonEmptyLines(t *testing.T, path string) []string {
	t.Helper()

	var lines []string
	waitForCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines = nonEmptyLines(string(payload))
		return len(lines) > 0
	})
	return lines
}

func waitForCondition(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not succeed before timeout")
}

func nonEmptyLines(input string) []string {
	lines := strings.Split(input, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}
