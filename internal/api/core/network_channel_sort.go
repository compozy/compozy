package core

import (
	"sort"
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

func sortedNetworkChannelPayloads(
	aggregates map[string]*networkChannelAggregate,
) []contract.NetworkChannelPayload {
	type sortableChannel struct {
		payload  contract.NetworkChannelPayload
		sequence int64
	}
	sortable := make([]sortableChannel, 0, len(aggregates))
	for _, aggregate := range aggregates {
		if aggregate == nil {
			continue
		}
		sortable = append(sortable, sortableChannel{
			payload:  networkChannelPayloadFromAggregate(aggregate),
			sequence: max(aggregate.lastActivitySequence, aggregate.lastPresenceSequence),
		})
	}
	sort.Slice(sortable, func(i int, j int) bool {
		leftSequence := sortable[i].sequence
		rightSequence := sortable[j].sequence
		if leftSequence != rightSequence && (leftSequence > 0 || rightSequence > 0) {
			return leftSequence > rightSequence
		}
		left := networkChannelSortTimestamp(sortable[i].payload)
		right := networkChannelSortTimestamp(sortable[j].payload)
		switch {
		case left != nil && right != nil && !left.Equal(*right):
			return left.After(*right)
		case left != nil && right == nil:
			return true
		case left == nil && right != nil:
			return false
		case sortable[i].payload.MessageCount != sortable[j].payload.MessageCount:
			return sortable[i].payload.MessageCount > sortable[j].payload.MessageCount
		default:
			return sortable[i].payload.Channel < sortable[j].payload.Channel
		}
	})
	channels := make([]contract.NetworkChannelPayload, 0, len(sortable))
	for _, item := range sortable {
		channels = append(channels, item.payload)
	}
	return channels
}

func networkChannelSortTimestamp(channel contract.NetworkChannelPayload) *time.Time {
	switch {
	case channel.LastActivityAt == nil:
		return channel.LastPresenceAt
	case channel.LastPresenceAt == nil:
		return channel.LastActivityAt
	case channel.LastPresenceAt.After(*channel.LastActivityAt):
		return channel.LastPresenceAt
	default:
		return channel.LastActivityAt
	}
}
