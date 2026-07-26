package cli

import (
	"strconv"

	"github.com/compozy/agh/internal/api/contract"
)

func networkThreadsBundle(response contract.NetworkThreadsResponse) outputBundle {
	threads := response.Threads
	return listBundle(
		response,
		threads,
		"Network Threads",
		[]string{
			taskThreadValue,
			"Root",
			networkOpenedByValue,
			networkMessagesValue,
			"Participants",
			networkOpenWorkValue,
			networkLastActivityValue,
			toolOperatorPreviewValue,
		},
		"network_threads",
		[]string{
			networkChannelKey,
			networkThreadIDKey,
			"root_message_id",
			networkOpenedByPeerIDKey,
			networkMessageCountKey,
			"participant_count",
			networkOpenWorkCountKey,
			networkLastActivityAtKey,
			networkLastMessagePreviewKey,
		},
		networkThreadHumanRow,
		networkThreadToonRow,
	)
}

func networkThreadBundle(thread NetworkThreadRecord) outputBundle {
	return outputBundle{
		jsonValue: contract.NetworkThreadResponse{Thread: thread},
		human: func() (string, error) {
			return renderHumanSection("Network Thread", networkThreadKeyValues(thread)), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"network_thread",
				[]string{
					networkChannelKey,
					networkThreadIDKey,
					"root_message_id",
					networkTitleKey,
					networkOpenedByPeerIDKey,
					"opened_session_id",
					networkOpenedAtKey,
					networkLastActivityAtKey,
					networkMessageCountKey,
					"participant_count",
					networkOpenWorkCountKey,
					networkLastMessagePreviewKey,
				},
				[]string{
					thread.Channel,
					thread.ThreadID,
					thread.RootMessageID,
					thread.Title,
					thread.OpenedByPeerID,
					thread.OpenedSessionID,
					formatTimePtr(thread.OpenedAt),
					formatTimePtr(thread.LastActivityAt),
					strconv.Itoa(thread.MessageCount),
					strconv.Itoa(thread.ParticipantCount),
					strconv.Itoa(thread.OpenWorkCount),
					thread.LastMessagePreview,
				},
			), nil
		},
	}
}

func networkThreadPromotionBundle(promoted *PromoteNetworkThreadTaskRecord) outputBundle {
	return outputBundle{
		jsonValue: promoted,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Promoted Task", []keyValue{
					{Label: "Task ID", Value: stringOrDash(promoted.Task.ID)},
					{Label: taskTitleValue, Value: stringOrDash(promoted.Task.Title)},
					{Label: networkChannelValue, Value: stringOrDash(promoted.Origin.Channel)},
					{Label: taskThreadValue, Value: stringOrDash(promoted.Origin.ThreadID)},
					{Label: networkMessageValue, Value: stringOrDash(promoted.Origin.OriginMessageID)},
				}),
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"network_thread_promotion",
				[]string{taskTaskIDKey, networkTitleKey, networkChannelKey, networkThreadIDKey, networkMessageIDKey},
				[]string{
					promoted.Task.ID,
					promoted.Task.Title,
					promoted.Origin.Channel,
					promoted.Origin.ThreadID,
					promoted.Origin.OriginMessageID,
				},
			), nil
		},
	}
}

func networkThreadMessagesBundle(response contract.NetworkThreadMessagesResponse) outputBundle {
	return networkMessagesBundle(response, response.Messages)
}

func networkDirectsBundle(response contract.NetworkDirectRoomsResponse) outputBundle {
	directs := response.Directs
	return listBundle(
		response,
		directs,
		"Network Direct Rooms",
		[]string{
			"Direct",
			"Session A",
			"Session B",
			networkMessagesValue,
			networkOpenWorkValue,
			networkLastActivityValue,
			toolOperatorPreviewValue,
		},
		"network_directs",
		[]string{
			networkChannelKey,
			networkDirectIDKey,
			"session_a",
			"session_b",
			networkMessageCountKey,
			networkOpenWorkCountKey,
			networkLastActivityAtKey,
			networkLastMessagePreviewKey,
		},
		networkDirectHumanRow,
		networkDirectToonRow,
	)
}

func networkDirectBundle(direct NetworkDirectRoomRecord) outputBundle {
	return outputBundle{
		jsonValue: contract.NetworkDirectRoomResponse{Direct: direct},
		human: func() (string, error) {
			return renderHumanSection("Network Direct Room", networkDirectKeyValues(direct)), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"network_direct",
				[]string{
					networkChannelKey,
					networkDirectIDKey,
					"session_a",
					"session_b",
					networkOpenedAtKey,
					networkLastActivityAtKey,
					networkMessageCountKey,
					networkOpenWorkCountKey,
					networkLastMessagePreviewKey,
				},
				[]string{
					direct.Channel,
					direct.DirectID,
					direct.SessionA,
					direct.SessionB,
					formatTimePtr(direct.OpenedAt),
					formatTimePtr(direct.LastActivityAt),
					strconv.Itoa(direct.MessageCount),
					strconv.Itoa(direct.OpenWorkCount),
					direct.LastMessagePreview,
				},
			), nil
		},
	}
}

func networkDirectMessagesBundle(response contract.NetworkDirectRoomMessagesResponse) outputBundle {
	return networkMessagesBundle(response, response.Messages)
}

func networkMessagesBundle(jsonValue any, messages []NetworkConversationMessageRecord) outputBundle {
	return listBundle(
		jsonValue,
		messages,
		"Network Messages",
		[]string{
			networkMessageValue,
			networkSurfaceValue,
			taskThreadValue,
			"Direct",
			networkKindValue,
			"Direction",
			networkFromValue,
			"To",
			"Work",
			networkTimestampValue,
			toolOperatorPreviewValue,
		},
		"network_messages",
		[]string{
			networkMessageIDKey,
			networkChannelKey,
			networkSurfaceKey,
			networkThreadIDKey,
			networkDirectIDKey,
			networkKindKey,
			"direction",
			"peer_from",
			"peer_to",
			networkWorkIDKey,
			networkTimestampKey,
			"preview_text",
		},
		networkMessageHumanRow,
		networkMessageToonRow,
	)
}
