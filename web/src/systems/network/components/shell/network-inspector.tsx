import {
  Eyebrow,
  MetadataListRoot,
  MetadataListRow,
  MetadataListTerm,
  MetadataListValue,
  PillGroup,
  Time,
} from "@agh/ui";

import { cn } from "@/lib/utils";

import type { ChannelMember } from "../../hooks/use-channel-members";
import type { InspectorTab } from "../../hooks/use-inspector-state";
import type { OpenWorkEntry } from "../../hooks/use-work";
import type { NetworkChannelSummary } from "../../types";
import { WorkInspector } from "../work/work-inspector";
import { InspectorMembersList } from "./inspector-members-list";

export interface NetworkInspectorProps {
  channel: NetworkChannelSummary;
  activeTab: InspectorTab;
  onTabChange: (tab: InspectorTab) => void;
  members: ReadonlyArray<ChannelMember>;
  isMembersLoading: boolean;
  workEntries: ReadonlyArray<OpenWorkEntry>;
  isWorkLoading: boolean;
  workCount: number;
  onWorkJump?: (entry: OpenWorkEntry) => void;
  className?: string;
}

/** Channel About block pinned above the inspector tabs: purpose + fixed KV rows. */
function InspectorAbout({ channel }: { channel: NetworkChannelSummary }) {
  const purpose = channel.purpose?.trim() || null;

  return (
    <div className="flex flex-col gap-2.5 border-b border-line px-4 pt-4 pb-3.5">
      <Eyebrow>About</Eyebrow>
      {purpose ? (
        <p className="text-small-body text-muted" data-testid="network-inspector-purpose">
          {purpose}
        </p>
      ) : null}
      <MetadataListRoot className="gap-1" data-testid="network-inspector-about">
        {channel.fanout_policy ? (
          <MetadataListRow>
            <MetadataListTerm>Fanout</MetadataListTerm>
            <MetadataListValue className="font-mono text-mono-id">
              {channel.fanout_policy}
            </MetadataListValue>
          </MetadataListRow>
        ) : null}
        {channel.coordinator_peer_id ? (
          <MetadataListRow>
            <MetadataListTerm>Coordinator</MetadataListTerm>
            <MetadataListValue className="font-mono text-mono-id">
              {channel.coordinator_peer_id}
            </MetadataListValue>
          </MetadataListRow>
        ) : null}
        {channel.created_by ? (
          <MetadataListRow>
            <MetadataListTerm>Created by</MetadataListTerm>
            <MetadataListValue className="font-mono text-mono-id">
              {channel.created_by}
            </MetadataListValue>
          </MetadataListRow>
        ) : null}
        {channel.created_at ? (
          <MetadataListRow>
            <MetadataListTerm>Created</MetadataListTerm>
            <MetadataListValue>
              <Time iso={channel.created_at} mode="relative" />
            </MetadataListValue>
          </MetadataListRow>
        ) : null}
      </MetadataListRoot>
    </div>
  );
}

/**
 * Channel inspector: About pinned on top, then a two-segment Members | Work
 * toggle over the matching list. Opening and closing belongs to the toolbar
 * toggle — the panel carries no chrome of its own.
 */
export function NetworkInspector({
  channel,
  activeTab,
  onTabChange,
  members,
  isMembersLoading,
  workEntries,
  isWorkLoading,
  workCount,
  onWorkJump,
  className,
}: NetworkInspectorProps) {
  return (
    <section
      aria-label="Channel inspector"
      className={cn("flex min-h-0 flex-1 flex-col", className)}
      data-testid="network-inspector"
    >
      <InspectorAbout channel={channel} />

      <div className="px-4 pt-3 pb-1">
        <PillGroup<InspectorTab>
          aria-label="Inspector sections"
          data-testid="network-inspector-tabs"
          items={[
            { value: "members", label: "Members", testId: "network-inspector-tab-members" },
            {
              value: "work",
              label: "Work",
              badge: workCount > 0 ? workCount : undefined,
              testId: "network-inspector-tab-work",
            },
          ]}
          onChange={onTabChange}
          size="sm"
          value={activeTab}
        />
      </div>

      <div
        className="flex min-h-0 flex-1 flex-col"
        data-testid={`network-inspector-panel-${activeTab}`}
      >
        {activeTab === "members" ? (
          <InspectorMembersList isLoading={isMembersLoading} members={members} />
        ) : (
          <WorkInspector
            chromeless
            entries={workEntries}
            isLoading={isWorkLoading}
            onJump={onWorkJump}
            totalCount={workCount}
          />
        )}
      </div>
    </section>
  );
}
