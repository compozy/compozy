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
export { Dock, type DockProps, type DockStatusProps } from "../components/custom/dock";
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
export { ThemeToggle, type ThemeToggleProps } from "../components/custom/theme-toggle";
export { ResizableDrawer, type ResizableDrawerProps } from "../components/custom/resizable-drawer";
