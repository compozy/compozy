package standalone

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/helpers"
)

// Test suite validating Redis Pub/Sub behavior using embedded miniredis via
// the mode-aware cache factory. These tests use native go-redis PubSub types.
func TestStreaming_MiniredisCompatibility(t *testing.T) {
	t.Run("Should publish and subscribe to events", func(t *testing.T) {
		ctx := t.Context()
		env := helpers.SetupStandaloneStreaming(ctx, t)
		defer env.Cleanup()

		t.Log("Testing publish/subscribe using embedded miniredis backend")

		events := make(chan string, 10)
		err := env.Subscribe(ctx, "test-channel", events)
		require.NoError(t, err)

		testEvent := "test-event-payload"
		err = env.Publish(ctx, "test-channel", testEvent)
		require.NoError(t, err)

		select {
		case evt := <-events:
			assert.Equal(t, testEvent, evt)
		case <-time.After(5 * time.Second):
			t.Fatal("Event not received within timeout")
		}

		// Send multiple sequential events and verify ordering
		want := []string{"e1", "e2", "e3"}
		for _, w := range want {
			require.NoError(t, env.Publish(ctx, "test-channel", w))
		}
		got := make([]string, 0, len(want))
		deadline := time.After(5 * time.Second)
		for len(got) < len(want) {
			select {
			case v := <-events:
				got = append(got, v)
			case <-deadline:
				t.Fatalf("Timed out waiting for events; got %v", got)
			}
		}
		assert.Equal(t, want, got, "Events must arrive in publish order")
	})

	t.Run("Should support pattern subscriptions", func(t *testing.T) {
		ctx := t.Context()
		env := helpers.SetupStandaloneStreaming(ctx, t)
		defer env.Cleanup()

		events := make(chan string, 10)
		err := env.SubscribePattern(ctx, "workflow:*", events)
		require.NoError(t, err)

		channels := []string{"workflow:123", "workflow:456", "workflow:789"}
		for _, ch := range channels {
			err = env.Publish(ctx, ch, "event-data")
			require.NoError(t, err)
		}
		// Publish to unrelated channel; should not be delivered
		require.NoError(t, env.Publish(ctx, "task:1", "ignore"))

		received := 0
		timeout := time.After(5 * time.Second)
		for received < len(channels) {
			select {
			case <-events:
				received++
			case <-timeout:
				t.Fatalf("Only received %d of %d events", received, len(channels))
			}
		}
		// Ensure we don't get the unrelated one within a brief window
		select {
		case extra := <-events:
			t.Fatalf("unexpected extra event: %s", extra)
		case <-time.After(500 * time.Millisecond):
		}
		assert.Equal(t, len(channels), received)
	})

	t.Run("Should support multiple subscribers", func(t *testing.T) {
		ctx := t.Context()
		env := helpers.SetupStandaloneStreaming(ctx, t)
		defer env.Cleanup()

		const numSubscribers = 5
		subscribers := make([]chan string, numSubscribers)
		for i := 0; i < numSubscribers; i++ {
			subscribers[i] = make(chan string, 10)
			err := env.Subscribe(ctx, "broadcast-channel", subscribers[i])
			require.NoError(t, err)
		}

		testEvent := "broadcast-event"
		err := env.Publish(ctx, "broadcast-channel", testEvent)
		require.NoError(t, err)

		for i, sub := range subscribers {
			select {
			case evt := <-sub:
				assert.Equal(t, testEvent, evt, "Subscriber %d didn't receive event", i)
			case <-time.After(5 * time.Second):
				t.Fatalf("Subscriber %d didn't receive event", i)
			}
		}
	})

	t.Run("Should deliver events reliably", func(t *testing.T) {
		ctx := t.Context()
		env := helpers.SetupStandaloneStreaming(ctx, t)
		defer env.Cleanup()

		events := make(chan string, 256)
		err := env.Subscribe(ctx, "reliable", events)
		require.NoError(t, err)

		const numEvents = 50
		for i := 0; i < numEvents; i++ {
			require.NoError(t, env.Publish(ctx, "reliable", fmt.Sprintf("event-%d", i)))
		}

		received := 0
		// Collect all and ensure order
		got := make([]string, 0, numEvents)
		deadline := time.After(10 * time.Second)
		for received < numEvents {
			select {
			case v := <-events:
				got = append(got, v)
				received++
			case <-deadline:
				t.Fatalf("Only received %d of %d events", received, numEvents)
			}
		}
		// Validate order
		want := make([]string, numEvents)
		for i := 0; i < numEvents; i++ {
			want[i] = fmt.Sprintf("event-%d", i)
		}
		assert.Equal(t, want, got, "Some events were lost or out of order")

		// Large payload delivery
		big := make([]byte, 32*1024) // 32KB
		for i := range big {
			big[i] = 'A'
		}
		bigStr := string(big)
		require.NoError(t, env.Publish(ctx, "reliable", bigStr))
		select {
		case v := <-events:
			assert.Equal(t, bigStr, v)
		case <-time.After(5 * time.Second):
			t.Fatal("Large event not received within timeout")
		}
	})

	t.Run("Should handle subscription lifecycle", func(t *testing.T) {
		ctx := t.Context()
		env := helpers.SetupStandaloneStreaming(ctx, t)
		defer env.Cleanup()

		sub := env.SubscribeRaw(ctx, "lifecycle")
		require.NoError(t, env.Publish(ctx, "lifecycle", "event-1"))
		msg, err := sub.ReceiveMessage(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event-1", msg.Payload)

		// Unsubscribe and close
		require.NoError(t, sub.Unsubscribe(ctx, "lifecycle"))
		require.NoError(t, sub.Close())

		// Re-subscribe using helper and confirm new messages flow
		events2 := make(chan string, 10)
		require.NoError(t, env.Subscribe(ctx, "lifecycle", events2))
		require.NoError(t, env.Publish(ctx, "lifecycle", "event-2"))
		select {
		case evt := <-events2:
			assert.Equal(t, "event-2", evt)
		case <-time.After(5 * time.Second):
			t.Fatal("Event not received after re-subscribe")
		}
	})

	t.Run("Should handle error cases gracefully", func(t *testing.T) {
		ctx := t.Context()
		env := helpers.SetupStandaloneStreaming(ctx, t)
		defer env.Cleanup()

		// Invalid pattern (empty) should error at helper level
		events := make(chan string, 1)
		err := env.SubscribePattern(ctx, "", events)
		require.Error(t, err)

		// Publish with no subscribers should not error
		require.NoError(t, env.Publish(ctx, "no-subs", "payload"))

		// Subscriber disconnection handling
		raw := env.SubscribeRaw(ctx, "disconnect")
		require.NoError(t, raw.Close())
		// After closing, ReceiveMessage should fail quickly
		cctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_, err = raw.ReceiveMessage(cctx)
		require.Error(t, err)
	})
}
