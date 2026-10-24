/**
 * The Home dashboard's loading shapes, on their own entry so a caller that shows them before the
 * dashboard code has loaded (the Home window's loading fallback) does not pull the hooks, the
 * queries or the live zones in with the barrel.
 */
export { DashboardWindowSkeleton } from "./components/dashboard-window-skeleton";
export { HomeDashboardSkeleton } from "./components/home-dashboard-skeleton";
