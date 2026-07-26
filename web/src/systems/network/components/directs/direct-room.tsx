import { Eyebrow } from "@agh/ui";

import { formatNetworkPresenceLabel } from "../../lib/network-formatters";
import type { NetworkPresence } from "../../types";
import { DetailComposer } from "../composer/detail-composer";
import { ConversationError } from "../empty-states/conversation-error";
import { DirectEmpty } from "../empty-states/direct-empty";
import { MessageTimeline } from "../timeline/timeline";
import { MessageAvatar } from "../timeline/message-avatar";
import { useMessageCopyActions } from "../timeline/use-message-copy-actions";
import { WorkBanner } from "../work/work-banner";
import { useDirectRoomView } from "./use-direct-room-view";

export interface DirectRoomProps {
  workspaceId: string;
  channel: string;
  directId: string;
  /** Used to render the *other* party's identity at the top per `_design.md` §5.6. */
  selfSessionId?: string;
}

interface PresenceBadgeProps {
  presence: NetworkPresence;
}

function PresenceBadge({ presence }: PresenceBadgeProps) {
  const label = formatNetworkPresenceLabel(presence.state);
  return (
    <span
      aria-label={`peer presence ${label}`}
      className="inline-flex min-w-0 items-center gap-1 text-form-label text-muted"
      data-state={presence.state}
      data-testid="network-direct-presence"
      title="This peer is joined to the local daemon."
    >
      <span
        aria-hidden="true"
        className="inline-block size-1.5 shrink-0 rounded-full bg-info"
        data-testid="network-direct-presence-dot"
      />
      <span className="min-w-0 truncate">{label}</span>
    </span>
  );
}

export function DirectRoom({ workspaceId, channel, directId, selfSessionId }: DirectRoomProps) {
  const view = useDirectRoomView({ workspaceId, channel, directId, selfSessionId });
  const { room, session, disabledReason, openWork, handleRetry, handleDiscard } = view;
  const otherPeerId = room.otherPeerId;
  const detailError = room.detailError;
  const isResolvingDetail = !detailError && !room.detail;
  const toolbarHandlers = useMessageCopyActions({
    surface: "direct",
    workspaceId,
    channel,
    conversationId: directId,
  });

  return (
    <section
      aria-label={`Direct room with @${otherPeerId || "peer"}`}
      className="flex min-h-0 flex-1 flex-col"
      data-testid="network-direct-room"
    >
      <header
        className="flex flex-wrap items-center gap-3 border-b border-line px-5 py-3"
        data-slot="direct-room-identity"
        data-testid="network-direct-identity-row"
      >
        {otherPeerId ? (
          <MessageAvatar
            initialFrom={otherPeerId}
            name={otherPeerId}
            ownerRole="agent"
            seed={otherPeerId}
            sizePx={32}
          />
        ) : null}
        <h2 className="min-w-0 flex-1 truncate text-compact-h1 font-semibold tracking-compact-h1 text-fg-strong">
          {otherPeerId ? `@${otherPeerId}` : "Direct room"}
        </h2>
        <div
          className="ml-auto flex items-center gap-2 text-small-body text-muted"
          data-slot="direct-room-meta"
          data-testid="network-direct-identity-row-meta"
        >
          <Eyebrow>agent</Eyebrow>
          <PresenceBadge presence={room.presence} />
        </div>
      </header>

      {detailError ? (
        <div className="flex flex-1 items-center justify-center px-5 py-10" role="alert">
          <ConversationError
            description={`Could not load direct room ${directId}. Choose an existing direct room from #${channel}.`}
            testId="network-direct-room-error"
            title="Direct room unavailable"
          />
        </div>
      ) : isResolvingDetail ? (
        <MessageTimeline
          ariaLabel={`Direct messages with @${otherPeerId || "peer"}`}
          density="channel"
          isLoading
          messages={[]}
        />
      ) : (
        <>
          <WorkBanner hasNeedsInput={false} openCount={openWork.openCount} />

          <MessageTimeline
            ariaLabel={`Direct messages with @${otherPeerId || "peer"}`}
            density="channel"
            emptyState={<DirectEmpty />}
            isLoading={room.isDetailLoading || room.isMessagesLoading}
            lastReadAt={room.lastReadIso}
            hasOlder={room.hasOlder}
            isLoadingOlder={room.isLoadingOlder}
            onLoadOlder={room.loadOlder}
            messages={room.messages}
            onDiscardOptimistic={handleDiscard}
            onRetryOptimistic={handleRetry}
            toolbarHandlers={toolbarHandlers}
          />

          <DetailComposer
            workspaceId={workspaceId}
            channel={channel}
            directId={directId}
            disabledReason={disabledReason ?? undefined}
            displayName={session?.displayName}
            peerFrom={session?.peerId ?? ""}
            peerLabel={otherPeerId ? `@${otherPeerId}` : "@peer"}
            peerTo={otherPeerId || undefined}
            sessionId={session?.sessionId ?? ""}
            surface="direct"
          />
        </>
      )}
    </section>
  );
}
