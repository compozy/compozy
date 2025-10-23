package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/pubsub"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
)

type stubSubscription struct{}

var closedDoneChan = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

func (s *stubSubscription) Messages() <-chan pubsub.Message { return nil }

func (s *stubSubscription) Done() <-chan struct{} { return closedDoneChan }

func (s *stubSubscription) Err() error { return nil }

func (s *stubSubscription) Close() error { return nil }

type stubProvider struct {
	channels  []string
	payloads  []string
	pubErr    error
	subCalled bool
}

func (s *stubProvider) Subscribe(context.Context, string) (pubsub.Subscription, error) {
	s.subCalled = true
	return &stubSubscription{}, errors.New("not implemented")
}

func (s *stubProvider) Publish(_ context.Context, channel string, payload []byte) error {
	if s.pubErr != nil {
		return s.pubErr
	}
	s.channels = append(s.channels, channel)
	s.payloads = append(s.payloads, string(payload))
	return nil
}

func TestTextStreamPublisherPublishesRedactedLines(t *testing.T) {
	provider := &stubProvider{}
	publisher := NewTextStreamPublisher(provider)
	require.NotNil(t, publisher)
	cfg := &task.Config{}
	output := core.Output{"response": "hello\nBearer very-secret-token"}
	state := &task.State{
		TaskExecID: core.MustNewID(),
		Output:     &output,
	}
	publisher.Publish(context.Background(), cfg, state)
	require.Len(t, provider.channels, 2)
	for _, channel := range provider.channels {
		require.Equal(t, fmt.Sprintf(streamChannelPattern, state.TaskExecID.String()), channel)
	}
	require.Equal(t, "hello", provider.payloads[0])
	require.Equal(t, "Bearer [REDACTED]", provider.payloads[1])
}

func TestTextStreamPublisherSkipsStructuredOutput(t *testing.T) {
	provider := &stubProvider{}
	publisher := NewTextStreamPublisher(provider)
	output := core.Output{"response": "ignored"}
	state := &task.State{TaskExecID: core.MustNewID(), Output: &output}
	cfg := &task.Config{BaseConfig: task.BaseConfig{OutputSchema: &schema.Schema{"type": "object"}}}
	publisher.Publish(context.Background(), cfg, state)
	require.Empty(t, provider.channels)
	require.Empty(t, provider.payloads)
}

func TestTextStreamPublisherSegmentsLongLines(t *testing.T) {
	provider := &stubProvider{}
	publisher := NewTextStreamPublisher(provider)
	longLine := strings.Repeat("a", maxSegmentRunes*2+50)
	output := core.Output{"response": longLine}
	state := &task.State{TaskExecID: core.MustNewID(), Output: &output}
	publisher.Publish(context.Background(), &task.Config{}, state)
	require.Len(t, provider.payloads, 3)
	require.Equal(t, maxSegmentRunes, utf8.RuneCountInString(provider.payloads[0]))
	require.Equal(t, maxSegmentRunes, utf8.RuneCountInString(provider.payloads[1]))
	require.Equal(t, 50, utf8.RuneCountInString(provider.payloads[2]))
}

func TestTextStreamPublisherHonorsChunkLimit(t *testing.T) {
	provider := &stubProvider{}
	publisher := NewTextStreamPublisher(provider)
	output := core.Output{"response": "first\nsecond\nthird"}
	state := &task.State{TaskExecID: core.MustNewID(), Output: &output}
	ctx := WithStreamChunkLimit(context.Background(), 2)
	publisher.Publish(ctx, &task.Config{}, state)
	require.Len(t, provider.payloads, 2)
}
