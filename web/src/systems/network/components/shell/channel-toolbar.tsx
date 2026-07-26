import { Briefcase, MoreHorizontal, PanelRight, RefreshCw, Settings2 } from "lucide-react";

import {
  Button,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  LaneTabs,
  SearchInput,
  type LaneTabsItem,
} from "@agh/ui";

import { useChannelToolbar } from "../../hooks/use-channel-toolbar";
import type { NetworkChannel, NetworkChannelSummary } from "../../types";
import { ChannelPolicyDialog } from "./channel-policy-dialog";
import type { ChannelTab } from "./channel-tabs-types";

interface TabItem extends LaneTabsItem<ChannelTab> {
  to:
    | "/network/$workspaceId/$channel/threads"
    | "/network/$workspaceId/$channel/directs"
    | "/network/$workspaceId/$channel/activity";
}

function buildTabs({
  threadCount,
  directCount,
}: {
  threadCount: number | null;
  directCount: number | null;
}): ReadonlyArray<TabItem> {
  return [
    {
      value: "threads",
      label: "Threads",
      count: threadCount ?? undefined,
      to: "/network/$workspaceId/$channel/threads",
      testId: "network-tab-threads",
    },
    {
      value: "directs",
      label: "Directs",
      count: directCount ?? undefined,
      to: "/network/$workspaceId/$channel/directs",
      testId: "network-tab-directs",
    },
    {
      value: "activity",
      label: "Activity",
      to: "/network/$workspaceId/$channel/activity",
      testId: "network-tab-activity",
    },
  ];
}

export interface ChannelToolbarProps {
  workspaceId: string;
  channel: NetworkChannelSummary;
  detail: NetworkChannel | null;
  activeTab: ChannelTab;
  threadCount: number | null;
  directCount: number | null;
  inspectorOpen: boolean;
  onInspectorToggle: () => void;
}

/**
 * Lane tabs on their own ruled row, then the list toolbar: search, the direct
 * "Has work" toggle, and the channel commands (mark read, inspector, kebab).
 */
export function ChannelToolbar({
  workspaceId,
  channel,
  detail,
  activeTab,
  threadCount,
  directCount,
  inspectorOpen,
  onInspectorToggle,
}: ChannelToolbarProps) {
  const tabs = buildTabs({ threadCount, directCount });
  const toolbar = useChannelToolbar({
    workspaceId,
    channel: channel.channel,
    tabTargets: tabs,
  });

  return (
    <div data-testid="network-channel-toolbar">
      <div className="mt-4 px-5">
        <LaneTabs<ChannelTab>
          ariaLabel={`Surfaces for #${channel.channel}`}
          data-testid="network-channel-tabs"
          items={tabs}
          listClassName="w-full"
          onChange={toolbar.onTabChange}
          value={activeTab}
        />
      </div>

      <div className="flex items-center gap-2 px-5 py-3">
        <SearchInput
          className="h-8 w-56"
          data-testid="network-list-search"
          onChange={toolbar.setSearchQuery}
          placeholder={`Search #${channel.channel}…`}
          value={toolbar.searchQuery}
        />
        <Button
          aria-pressed={toolbar.hasWorkActive}
          className={cn(toolbar.hasWorkActive ? "bg-elevated text-fg" : null)}
          data-testid="network-toolbar-has-work"
          onClick={toolbar.onHasWorkToggle}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Briefcase aria-hidden="true" className="size-3" />
          Has work
        </Button>

        <div className="ml-auto flex items-center gap-1.5">
          <Button
            aria-label="Mark loaded conversations as read"
            data-testid="network-list-mark-all-read"
            disabled={toolbar.isMarkLoadedReadDisabled}
            onClick={toolbar.markLoadedRead}
            size="sm"
            type="button"
            variant="ghost"
          >
            Mark read
          </Button>
          <Button
            aria-label={inspectorOpen ? "Close channel inspector" : "Open channel inspector"}
            aria-pressed={inspectorOpen}
            className={cn(inspectorOpen ? "bg-elevated text-fg" : null)}
            data-state={inspectorOpen ? "open" : "closed"}
            data-testid="network-channel-inspector-toggle"
            onClick={onInspectorToggle}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <PanelRight aria-hidden="true" className="size-4" />
          </Button>

          <DropdownMenu onOpenChange={toolbar.setOverflowOpen} open={toolbar.overflowOpen}>
            <DropdownMenuTrigger
              render={
                <Button
                  aria-label="Channel actions"
                  data-testid="network-channel-kebab"
                  size="icon-sm"
                  type="button"
                  variant="ghost"
                />
              }
            >
              <MoreHorizontal aria-hidden="true" className="size-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                data-testid="network-channel-policy"
                onSelect={event => {
                  event.preventDefault();
                  toolbar.setPolicyOpen(true);
                  toolbar.setOverflowOpen(false);
                }}
              >
                <Settings2 aria-hidden="true" className="size-3" />
                Delivery policy
              </DropdownMenuItem>
              <DropdownMenuItem
                data-testid="network-channel-refresh"
                onSelect={event => {
                  event.preventDefault();
                  toolbar.onRefresh();
                }}
              >
                <RefreshCw aria-hidden="true" className="size-3" />
                Refresh data
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <ChannelPolicyDialog
        channel={channel}
        detail={detail}
        isSubmitting={toolbar.isPolicySubmitting}
        members={toolbar.members.members}
        onOpenChange={toolbar.setPolicyOpen}
        onSubmit={toolbar.submitPolicy}
        open={toolbar.policyOpen}
      />
    </div>
  );
}
