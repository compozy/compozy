// Components
export { HomeDashboard } from "./components/home-dashboard";
export { ActivityPage } from "./components/activity/activity-page";
// The loading shapes also live on their own light entry, `@/systems/dashboard/skeleton`.
export { DashboardWindowSkeleton } from "./components/dashboard-window-skeleton";

// Route preload — scope resolution, prefs snapshot, and warmed query options
export { homeScopeForActiveWorkspace, type HomeScope } from "./lib/home-scope";
export { homeWorkingNowSessionFilters } from "./lib/home-working-now-query";
export { homePrefsStore } from "./hooks/use-home-prefs-store";
export { homeActivityOptions, homeOverviewOptions } from "./lib/query-options";

export { dashboardKeys } from "./lib/query-keys";
export { HomePulseHeatmap } from "./components/home-pulse-heatmap";
export {
  HOME_OUTCOMES_LEGEND_CLASS,
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
  HOME_USAGE_HEADLINE_CLASS,
} from "./components/home-zone-shape";
