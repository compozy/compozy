// Types
export type {
  CreateNetworkChannelRequest,
  CreateNetworkChannelResponse,
  NetworkCapability,
  NetworkCapabilityBrief,
  NetworkCapabilityCatalog,
  NetworkChannel,
  NetworkChannelDetailResponse,
  NetworkChannelUpdateRequest,
  NetworkChannelUpdateResponse,
  NetworkChannelsResponse,
  NetworkChannelsQuery,
  NetworkChannelSummary,
  NetworkConversationMessage,
  NetworkConversationMessageFilters,
  NetworkConversationMessagesResponse,
  NetworkConversationMessagesQuery,
  NetworkCursorPageParam,
  NetworkCreateChannelDraft,
  NetworkDirectRoomDetail,
  NetworkDirectRoomDetailResponse,
  NetworkDirectRoomMessage,
  NetworkDirectRoomMessagesResponse,
  NetworkDirectRoomSummary,
  NetworkDirectRoomsResponse,
  NetworkDirectsListQuery,
  NetworkKindFilter,
  NetworkMessagePageParam,
  NetworkFanoutPolicy,
  NetworkPeerCard,
  NetworkPeerDetail,
  NetworkPeerDetailResponse,
  NetworkPeerSummary,
  NetworkPeersResponse,
  NetworkPresence,
  NetworkPresenceState,
  NetworkRecentEntry,
  NetworkResolveDirectRoomRequest,
  NetworkResolveDirectRoomResponse,
  NetworkRouteSurface,
  NetworkSendRequest,
  NetworkSendResponse,
  NetworkSignalTone,
  NetworkStatus,
  NetworkStatusResponse,
  NetworkSubscription,
  NetworkSubscriptionMode,
  NetworkSubscriptionRequest,
  NetworkSubscriptionResponse,
  NetworkSubscriptionsResponse,
  NetworkSurface,
  NetworkThreadDetail,
  NetworkThreadDetailResponse,
  NetworkThreadTaskLink,
  NetworkThreadMessage,
  NetworkThreadMessagesResponse,
  NetworkThreadsResponse,
  NetworkThreadsListQuery,
  NetworkThreadSummary,
  NetworkUsageFilters,
  NetworkUsageQuery,
  NetworkWorkDetail,
  NetworkWorkResponse,
  TaskRunNetworkProjection,
  TaskRunNetworkUsage,
} from "./types";

// Adapters
export {
  createNetworkChannel,
  getNetworkChannel,
  getNetworkDirectRoom,
  getNetworkPeer,
  getNetworkStatus,
  getNetworkThread,
  getNetworkWork,
  deleteNetworkSubscription,
  listNetworkChannels,
  listNetworkDirectRoomMessages,
  listNetworkDirectRooms,
  listNetworkPeers,
  listNetworkSubscriptions,
  listNetworkThreadMessages,
  listNetworkThreads,
  NetworkApiError,
  promoteNetworkThreadTask,
  resolveNetworkDirectRoom,
  sendNetworkMessage,
  updateNetworkChannel,
  upsertNetworkSubscription,
} from "./adapters/network-api";
export type { NetworkSubscriptionsListQuery } from "./adapters/network-api";

// Query infrastructure
export { networkKeys } from "./lib/query-keys";
export {
  networkChannelDetailOptions,
  networkChannelsOptions,
  networkDirectDetailOptions,
  networkDirectMessagesOptions,
  networkDirectsOptions,
  networkPeersOptions,
  networkStatusOptions,
  networkSubscriptionsOptions,
  networkThreadDetailOptions,
  networkThreadMessagesOptions,
  networkThreadsOptions,
} from "./lib/query-options";

// Lib
export {
  createNetworkChannelDraft,
  formatNetworkKindLabel,
  formatNetworkPresenceLabel,
  formatNetworkRelativeTime,
  formatNetworkWorkStateLabel,
  getMessageAuthorInitial,
  getMostRecentTimestamp,
  getNetworkKindTone,
  getNetworkStatusTone,
  getPeerDisplayName,
  getPeerRecencyAt,
  isNetworkWorkState,
  isTerminalNetworkWorkState,
  shouldRenderNetworkWorkChip,
  sortAgentsForNetwork,
  toNetworkKindFilter,
  toNetworkPresenceState,
  toggleDraftAgent,
} from "./lib/network-formatters";
export type { NetworkWorkState } from "./lib/network-formatters";
export { formatElapsedSeconds, useElapsedSeconds } from "./lib/use-elapsed";
export type { UseElapsedOptions } from "./lib/use-elapsed";
export { formatTaskRunBounds } from "./lib/task-run-network";
export {
  NETWORK_IDENTITY_PALETTE,
  getIdentityInitial,
  pickIdentityPaletteColors,
  pickIdentityPaletteIndex,
} from "./lib/palette";

// Hooks
export { useChannelMembers } from "./hooks/use-channel-members";
export type {
  ChannelMember,
  ChannelMemberRole,
  UseChannelMembersResult,
} from "./hooks/use-channel-members";
export { useInspectorState } from "./hooks/use-inspector-state";
export type { InspectorTab, UseInspectorStateResult } from "./hooks/use-inspector-state";
export { useNetworkInspectorView } from "./hooks/use-network-inspector-view";
export type {
  UseNetworkInspectorViewArgs,
  UseNetworkInspectorViewResult,
} from "./hooks/use-network-inspector-view";
export { useNetworkCreateChannelAction } from "./hooks/use-network-create-channel-action";
export { useNetworkChannelDirectsRoute } from "./hooks/use-network-channel-directs-route";
export type { UseNetworkChannelDirectsRouteResult } from "./hooks/use-network-channel-directs-route";
export { useNetworkRailView } from "./hooks/use-network-rail-view";
export type {
  UseNetworkRailViewArgs,
  UseNetworkRailViewResult,
} from "./hooks/use-network-rail-view";
export {
  createNetworkChipFilter,
  NETWORK_FILTER_KEYS,
  useNetworkListFilters,
} from "./hooks/use-network-list-filters";
export type {
  NetworkChipFilter,
  NetworkFilterKey,
  NetworkListSort,
  UseNetworkListFiltersArgs,
  UseNetworkListFiltersResult,
} from "./hooks/use-network-list-filters";
export { NetworkListFiltersProvider } from "./contexts/network-list-filters-context";
export { useNetworkListFiltersContext } from "./hooks/use-network-list-filters-context";
export { useNetworkChannels } from "./hooks/use-channels";
export { useLastRead, buildLastReadStorageKey } from "./hooks/use-last-read";
export type { NetworkLastReadKey, UseLastReadResult } from "./hooks/use-last-read";
export { useNetworkDirects, useNetworkDirectDetail } from "./hooks/use-directs";
export type {
  UseNetworkDirectsOptions,
  UseNetworkDirectsResult,
  UseNetworkDirectDetailResult,
} from "./hooks/use-directs";
export { useNetworkMessages } from "./hooks/use-messages";
export type { UseNetworkMessagesArgs, UseNetworkMessagesResult } from "./hooks/use-messages";
export { useTaskRunConversation } from "./hooks/use-task-run-conversation";
export type { UseTaskRunConversationResult } from "./hooks/use-task-run-conversation";
export { useNetworkPage } from "./hooks/use-network-page";
export type { UseNetworkPageResult } from "./hooks/use-network-page";
export { useDirectRoom } from "./hooks/use-direct-room";
export type { UseDirectRoomArgs, UseDirectRoomResult } from "./hooks/use-direct-room";
export { useThreadOverlay } from "./hooks/use-thread-overlay";
export type { UseThreadOverlayArgs, UseThreadOverlayResult } from "./hooks/use-thread-overlay";
export { useNetworkRecents } from "./hooks/use-recents";
export type { UseNetworkRecentsResult } from "./hooks/use-recents";
export { useNetworkRouteShell } from "./hooks/use-network-route-shell";
export type {
  NetworkRouteShellResult,
  UseNetworkRouteShellArgs,
} from "./hooks/use-network-route-shell";
export { useNetworkRouteView } from "./hooks/use-network-route-view";
export { useNetworkThreads, useNetworkThreadDetail } from "./hooks/use-threads";
export type {
  UseNetworkThreadsOptions,
  UseNetworkThreadsResult,
  UseNetworkThreadDetailResult,
} from "./hooks/use-threads";
export { THREAD_OVERLAY_BREAKPOINT_PX, useThreadViewMode } from "./hooks/use-thread-view-mode";
export type { ThreadViewMode } from "./hooks/use-thread-view-mode";
export { useNetworkChannelThreadsRoute } from "./hooks/use-network-channel-threads-route";
export type {
  UseNetworkChannelThreadsRouteArgs,
  UseNetworkChannelThreadsRouteResult,
} from "./hooks/use-network-channel-threads-route";
export {
  isOptimisticMessage,
  THREAD_COLLISION_TOAST,
  useCreateNetworkChannel,
  useCreateNetworkThread,
  useDeleteNetworkSubscription,
  usePromoteNetworkThreadTask,
  useResolveNetworkDirectRoom,
  useSendNetworkMessage,
  useUpdateNetworkChannel,
  useUpsertNetworkSubscription,
} from "./hooks/use-network-actions";
export type {
  CreateNetworkThreadInput,
  CreateNetworkThreadResult,
  DeleteNetworkSubscriptionInput,
  OptimisticConversationMessage,
  PromoteNetworkThreadTaskInput,
  ResolveNetworkDirectRoomInput,
  SendNetworkMessageDirectInput,
  SendNetworkMessageInput,
  SendNetworkMessageResult,
  SendNetworkMessageThreadInput,
  UpdateNetworkChannelInput,
  UpsertNetworkSubscriptionInput,
  UseCreateNetworkThreadResult,
  UseResolveNetworkDirectRoomResult,
  UseSendNetworkMessageResult,
} from "./hooks/use-network-actions";
export { useActiveNetworkSession } from "./hooks/use-active-session";
export type {
  ActiveNetworkSession,
  UseActiveNetworkSessionResult,
} from "./hooks/use-active-session";
export { useOpenWork } from "./hooks/use-work";
export type { OpenWorkEntry, UseOpenWorkArgs, UseOpenWorkResult } from "./hooks/use-work";

// Lib (timeline composition + formatters)
export { buildTimelineEntries, isSystemKind, SYSTEM_KINDS } from "./lib/group-messages";
export type {
  TimelineDatePillEntry,
  TimelineEntry,
  TimelineMessageEntry,
  TimelineNewDividerEntry,
  TimelineRowVariant,
} from "./lib/group-messages";
export {
  TIMELINE_GROUP_WINDOW_SECONDS,
  formatDatePill,
  formatTimelineClock,
  formatTimelineClockWithSeconds,
  formatTimelineIso,
  isSameCalendarDay,
  isWithinSeconds,
} from "./lib/format-timestamp";

// Components — re-export from @agh/ui after kit promotion. Network surface
// continues to export KindChip from its barrel as a convenience for callers
// reaching for the network grammar; the canonical home is the shared kit.
export { KindChip } from "@agh/ui";
export type { KindChipProps } from "@agh/ui";
export { NetworkCreateChannelDialog } from "./components/network-create-channel-dialog";
export { NetworkCoordinationInvitation } from "./components/coordination-invitation";
export type { NetworkCoordinationInvitationProps } from "./components/coordination-invitation";
export { NetworkParticipationFields } from "./components/network-participation-fields";
export type { NetworkParticipationFieldsProps } from "./components/network-participation-fields";
export { TaskRunCoordinationInvitationHost } from "./components/task-run-coordination-invitation-host";
export type { TaskRunCoordinationInvitationHostProps } from "./components/task-run-coordination-invitation-host";
export { TaskRunConversationPanel } from "./components/task-run-conversation-panel";
export type { TaskRunConversationPanelProps } from "./components/task-run-conversation-panel";
export {
  DEFAULT_NETWORK_PARTICIPATION_DRAFT,
  NETWORK_PARTICIPATION_STRATEGIES,
  isNetworkParticipationDraftValid,
  isNetworkParticipationStrategy,
  networkParticipationDraftFromPayload,
  networkParticipationDraftFromValues,
  networkParticipationValidationMessage,
  serializeNetworkParticipation,
} from "./lib/network-participation";
export type {
  NetworkParticipationDraft,
  NetworkParticipationMode,
  NetworkParticipationPayload,
  NetworkParticipationStrategy,
} from "./lib/network-participation";
export {
  useAcceptNetworkCoordinationInvitation,
  useDismissNetworkCoordinationInvitation,
  useNetworkCoordination,
  useNetworkUsage,
} from "./hooks/use-network-coordination";
export { networkCoordinationOptions, networkUsageOptions } from "./lib/query-options";
export {
  getNetworkCoordination,
  getNetworkUsage,
  putNetworkCoordination,
  putNetworkCoordinationInvitation,
} from "./adapters/network-coordination-api";
export type {
  NetworkCoordinationRef,
  NetworkCoordinationResponse,
  NetworkUsageResponse,
} from "./adapters/network-coordination-api";

// Components — timeline subtree
export {
  DatePill,
  HoverToolbar,
  MessageAvatar,
  MessageBodyText,
  MessageRow,
  MessageRowCollapsed,
  MessageRowSystem,
  MessageTimeline,
  NewDivider,
  readMessageBody,
} from "./components/timeline";
export type {
  DatePillProps,
  HoverToolbarHandlers,
  HoverToolbarProps,
  MessageAvatarProps,
  MessageRowCollapsedProps,
  MessageRowDensity,
  MessageRowOptimisticHandlers,
  MessageRowProps,
  MessageRowSystemProps,
  MessageTimelineProps,
  NewDividerProps,
} from "./components/timeline";

// Components — thread overlay subtree
export {
  ThreadOverlay,
  ThreadOverlayHeader,
  ThreadOverlayReplies,
  ThreadOverlayRoot,
} from "./components/thread-overlay";
export type {
  ThreadOverlayHeaderProps,
  ThreadOverlayProps,
  ThreadOverlayRepliesProps,
  ThreadOverlayRootProps,
} from "./components/thread-overlay";

// Components — list views
export { ThreadsList } from "./components/threads";
export type { ThreadsListProps } from "./components/threads";
export { DirectRoom, DirectsList, NewDirectDialog } from "./components/directs";
export type { DirectRoomProps, DirectsListProps, NewDirectDialogProps } from "./components/directs";
export { ActivityFeed } from "./components/activity";
export type { ActivityFeedProps } from "./components/activity";

// Components — composer subtree
export {
  ChannelThreadComposer,
  Composer,
  ComposerSlashPopover,
  ComposerToolbar,
  DetailComposer,
} from "./components/composer";
export type {
  ChannelThreadComposerProps,
  ComposerProps,
  ComposerSlashPopoverProps,
  ComposerSubmitArgs,
  ComposerToolbarProps,
  DetailComposerDirectProps,
  DetailComposerProps,
  DetailComposerThreadProps,
  SlashCommandEntry,
} from "./components/composer";

// Components — work surfacing subtree
export { WorkBanner, WorkChip, WorkInspector, WorkInspectorRow } from "./components/work";
export type {
  WorkBannerProps,
  WorkChipProps,
  WorkInspectorProps,
  WorkInspectorRowProps,
} from "./components/work";

// Components — empty / disabled / error states
export {
  DaemonDown,
  DirectEmpty,
  DirectsEmpty,
  NetworkEmpty,
  ThreadEmpty,
  ThreadsEmpty,
} from "./components/empty-states";

// Window controller seam — Network deliberately owns its nested location
// parser because this is the domain whose composition was router-coupled.
export { NetworkWindowController } from "./routes/network-window-controller";
export type { NetworkWindowControllerProps } from "./routes/network-window-controller";
export type { NetworkWindowNavigation } from "./hooks/use-network-route-shell";
export { networkThreadsLocation, parseNetworkWindowLocation } from "./lib/network-window-location";
export type {
  NetworkWindowLocation,
  ParsedNetworkWindowLocation,
} from "./lib/network-window-location";
export type {
  DaemonDownProps,
  DirectEmptyProps,
  DirectsEmptyProps,
  NetworkEmptyProps,
  ThreadEmptyProps,
  ThreadsEmptyProps,
} from "./components/empty-states";
