import { Pill, Time } from "@agh/ui";

import { useChannelMembers } from "../../hooks/use-channel-members";
import type { NetworkChannel, NetworkChannelSummary } from "../../types";

export interface ChannelHeaderProps {
  workspaceId: string;
  channel: NetworkChannelSummary;
  detail: NetworkChannel | null;
  openWorkCount: number;
  threadCount: number | null;
}

function MetaDot() {
  return <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />;
}

interface MetaSegment {
  key: string;
  content: React.ReactNode;
}

function buildMetaSegments({
  channel,
  detail,
  threadCount,
  agentCount,
  humanCount,
}: {
  channel: NetworkChannelSummary;
  detail: NetworkChannel | null;
  threadCount: number | null;
  agentCount: number;
  humanCount: number;
}): MetaSegment[] {
  const segments: MetaSegment[] = [];
  const totalMembers = agentCount + humanCount;
  const fallbackPeerCount =
    detail?.peer_count ?? channel.peer_count ?? channel.local_peer_count ?? 0;

  if (totalMembers > 0) {
    if (agentCount > 0) {
      segments.push({
        key: "agents",
        content: `${agentCount} ${agentCount === 1 ? "agent" : "agents"}`,
      });
    }
    if (humanCount > 0) {
      segments.push({
        key: "humans",
        content: `${humanCount} ${humanCount === 1 ? "human" : "humans"}`,
      });
    }
  } else if (fallbackPeerCount > 0) {
    segments.push({
      key: "peers",
      content: `${fallbackPeerCount} ${fallbackPeerCount === 1 ? "peer" : "peers"}`,
    });
  } else {
    segments.push({ key: "peers", content: "no peers yet" });
  }

  if (threadCount !== null && threadCount > 0) {
    segments.push({
      key: "threads",
      content: `${threadCount} ${threadCount === 1 ? "thread" : "threads"}`,
    });
  }

  const messageCount = detail?.message_count ?? channel.message_count;
  if (typeof messageCount === "number" && messageCount > 0) {
    segments.push({
      key: "messages",
      content: `${messageCount.toLocaleString()} ${messageCount === 1 ? "message" : "messages"}`,
    });
  }

  const lastActivityAt = detail?.last_activity_at ?? channel.last_activity_at;
  if (lastActivityAt) {
    segments.push({
      key: "active",
      content: (
        <span className="inline-flex items-center gap-1">
          active <Time iso={lastActivityAt} mode="relative" />
        </span>
      ),
    });
  }

  const coordinator = detail?.coordinator_peer_id ?? channel.coordinator_peer_id;
  if (coordinator) {
    segments.push({
      key: "coordinator",
      content: (
        <span>
          coordinator <span className="font-mono text-mono-id text-muted">{coordinator}</span>
        </span>
      ),
    });
  }

  return segments;
}

/**
 * The channel content head (`chead` contract): name line with work + fanout
 * pills, purpose on its own line, then a dot-separated meta row. Route
 * identity and channel actions live in the window head and toolbar, so this
 * block carries no icon tile and no buttons.
 */
export function ChannelHeader({
  workspaceId,
  channel,
  detail,
  openWorkCount,
  threadCount,
}: ChannelHeaderProps) {
  const members = useChannelMembers(channel.channel, { workspaceId });
  const purpose = detail?.purpose?.trim() || channel.purpose?.trim() || null;
  const fanoutPolicy = detail?.fanout_policy || channel.fanout_policy || null;
  const metaSegments = buildMetaSegments({
    channel,
    detail,
    threadCount,
    agentCount: members.agentCount,
    humanCount: members.humanCount,
  });

  return (
    <header className="flex flex-col px-5 pt-4" data-testid="network-channel-header">
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <h1 className="flex min-w-0 items-baseline gap-1 text-compact-h1 font-semibold tracking-compact-h1 text-fg-strong">
          <span aria-hidden="true" className="font-medium text-subtle">
            #
          </span>
          <span className="truncate" data-testid="network-channel-title">
            {channel.channel}
          </span>
        </h1>
        {openWorkCount > 0 ? (
          <Pill data-testid="network-channel-work-pill" tone="warning">
            <Pill.Dot tone="warning" />
            {openWorkCount} work open
          </Pill>
        ) : null}
        {fanoutPolicy ? (
          <Pill data-testid="network-channel-fanout-pill" mono tone="neutral">
            {fanoutPolicy}
          </Pill>
        ) : null}
      </div>
      {purpose ? (
        <p
          className="mt-1.5 max-w-[64ch] text-small-body text-muted"
          data-testid="network-channel-purpose"
        >
          {purpose}
        </p>
      ) : null}
      <div
        className="mt-2 flex min-w-0 flex-wrap items-center gap-2 text-form-label text-subtle"
        data-testid="network-channel-meta"
      >
        {metaSegments.map((segment, index) => (
          <span className="inline-flex items-center gap-2" key={segment.key}>
            {index > 0 ? <MetaDot /> : null}
            <span data-testid={`network-channel-meta-${segment.key}`}>{segment.content}</span>
          </span>
        ))}
      </div>
    </header>
  );
}
