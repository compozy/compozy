// Shell chrome: topbar publication API, navigation, rails, dock, status chrome.
export {
  Topbar,
  TopbarOverflowIcon,
  TopbarSlotProvider,
  type TopbarProps,
  type TopbarSlotProviderProps,
} from "../components/custom/topbar";
export {
  createTopbarSlotStore,
  useTopbarSlot,
  useTopbarSlotValue,
  type TopbarCrumb,
  type TopbarSlotStore,
  type TopbarSlotValue,
} from "../components/custom/hooks/use-topbar-slot";
export { RouteNav } from "../components/custom/route-nav";
export { KindChip, type KindChipProps } from "../components/custom/kind-chip";
export {
  RightRail,
  type RightRailMode,
  type RightRailProps,
} from "../components/custom/right-rail";
export { Dock, type DockProps, type DockStatusProps } from "../components/custom/dock";
export {
  PageActionsTopbarSlot,
  type PageActionsTopbarSlotProps,
} from "../components/custom/page-actions-topbar-slot";
export {
  RestartBanner,
  type RestartBannerProps,
  type RestartBannerTone,
} from "../components/custom/restart-banner";
export {
  ConnectionIndicator,
  type ConnectionIndicatorDotProps,
  type ConnectionIndicatorLabelProps,
  type ConnectionIndicatorProps,
  type ConnectionStatus,
  type ConnectionVariant,
} from "../components/custom/connection-indicator";
export { LiveBadge, type LiveBadgeProps } from "../components/custom/live-badge";
