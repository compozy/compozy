// Components
export { HomeDashboard } from "./components/home-dashboard";

// Route preload — scope resolution, prefs snapshot, and warmed query options
export { homeScopeForActiveWorkspace, type HomeScope } from "./lib/home-scope";
export { homeWorkingNowSessionFilters } from "./lib/home-working-now-query";
export { useHomePrefsStore } from "./hooks/use-home-prefs-store";
export { homeActivityOptions, homeOverviewOptions } from "./lib/query-options";
