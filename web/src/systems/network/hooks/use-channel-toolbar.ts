import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { useChannelMembers, type UseChannelMembersResult } from "./use-channel-members";
import { useNetworkListFiltersContext } from "./use-network-list-filters-context";
import { createNetworkChipFilter } from "./use-network-list-filters";
import { useUpdateNetworkChannel } from "./use-network-actions";
import { networkKeys } from "../lib/query-keys";
import type { ChannelTab } from "../components/shell/channel-tabs-types";
import type { UpdateNetworkChannelInput } from "./use-network-actions";

export interface UseChannelToolbarArgs {
  workspaceId: string;
  channel: string;
  tabTargets: ReadonlyArray<{ value: ChannelTab; to: string }>;
}

export interface UseChannelToolbarResult {
  searchQuery: string;
  setSearchQuery: (next: string) => void;
  hasWorkActive: boolean;
  onHasWorkToggle: () => void;
  markLoadedRead: () => void;
  isMarkLoadedReadDisabled: boolean;
  overflowOpen: boolean;
  setOverflowOpen: (open: boolean) => void;
  policyOpen: boolean;
  setPolicyOpen: (open: boolean) => void;
  isPolicySubmitting: boolean;
  submitPolicy: (data: UpdateNetworkChannelInput["data"]) => Promise<void>;
  members: UseChannelMembersResult;
  onTabChange: (next: ChannelTab) => void;
  onRefresh: () => void;
}

/** View-model for the channel toolbar row: list filters, tab routing, and channel commands. */
export function useChannelToolbar({
  workspaceId,
  channel,
  tabTargets,
}: UseChannelToolbarArgs): UseChannelToolbarResult {
  const {
    filters,
    setFilters,
    searchQuery,
    setSearchQuery,
    markLoadedRead,
    isMarkLoadedReadDisabled,
  } = useNetworkListFiltersContext();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [overflowOpen, setOverflowOpen] = useState(false);
  const [policyOpen, setPolicyOpen] = useState(false);
  const updateChannel = useUpdateNetworkChannel();
  const members = useChannelMembers(channel, { workspaceId });
  const hasWorkActive = filters.some(filter => filter.field === "has_work");

  const onHasWorkToggle = () => {
    setFilters(
      hasWorkActive
        ? filters.filter(filter => filter.field !== "has_work")
        : [...filters, createNetworkChipFilter("has_work")]
    );
  };

  const onTabChange = (next: ChannelTab) => {
    const target = tabTargets.find(tab => tab.value === next);
    if (!target || !workspaceId) return;
    void navigate({ params: { workspaceId, channel }, to: target.to });
  };

  const onRefresh = () => {
    if (workspaceId) {
      void queryClient.invalidateQueries({
        queryKey: networkKeys.channelScope(workspaceId, channel),
      });
    }
    setOverflowOpen(false);
  };

  const submitPolicy = async (data: UpdateNetworkChannelInput["data"]) => {
    await updateChannel.mutateAsync({ workspaceId, channel, data });
  };

  return {
    searchQuery,
    setSearchQuery,
    hasWorkActive,
    onHasWorkToggle,
    markLoadedRead,
    isMarkLoadedReadDisabled,
    overflowOpen,
    setOverflowOpen,
    policyOpen,
    setPolicyOpen,
    isPolicySubmitting: updateChannel.isPending,
    submitPolicy,
    members,
    onTabChange,
    onRefresh,
  };
}
