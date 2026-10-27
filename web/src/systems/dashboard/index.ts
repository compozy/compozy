// Components
export { HomeDashboard } from "./components/home-dashboard";
// The loading shapes also live on their own light entry, `@/systems/dashboard/skeleton`.
export { DashboardWindowSkeleton } from "./components/dashboard-window-skeleton";

// Route preload — scope resolution, prefs snapshot, and warmed query options
export { homeScopeForActiveWorkspace, type HomeScope } from "./lib/home-scope";
export { homeWorkingNowSessionFilters } from "./lib/home-working-now-query";
export { homePrefsStore } from "./hooks/use-home-prefs-store";
export { homeActivityOptions, homeOverviewOptions } from "./lib/query-options";

export { dashboardKeys } from "./lib/query-keys";
