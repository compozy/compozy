package streaming

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/core"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisPublisherPublishAndReplay(t *testing.T) {
	t.Parallel()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	publisher, err := NewRedisPublisher(client, &RedisOptions{
		MaxEntries: 5,
		TTL:        time.Minute,
	})
	require.NoError(t, err)
	execID := core.MustNewID()
	ctx := context.Background()

	first, err := publisher.Publish(ctx, execID, Event{
		Type: EventTypeStatus,
		Data: map[string]any{"status": "started"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), first.ID)

	second, err := publisher.Publish(ctx, execID, Event{
		Type: EventTypeLLMChunk,
		Data: map[string]any{"content": "hello"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), second.ID)

	replayed, err := publisher.Replay(ctx, execID, 0, 10)
	require.NoError(t, err)
	require.Len(t, replayed, 2)
	require.Equal(t, first.ID, replayed[0].ID)
	require.Equal(t, second.ID, replayed[1].ID)

	sinceFirst, err := publisher.Replay(ctx, execID, first.ID, 10)
	require.NoError(t, err)
	require.Len(t, sinceFirst, 1)
	require.Equal(t, second.ID, sinceFirst[0].ID)

	logTTL := server.TTL(publisher.logKey(execID))
	seqTTL := server.TTL(publisher.seqKey(execID))
	require.True(t, logTTL > 0, "log key should have ttl")
	require.True(t, seqTTL > 0, "sequence key should have ttl")
}
