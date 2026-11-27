import { createFileRoute } from "@tanstack/react-router";

import { AccountUpgradeSheetProvider } from "@/systems/account";
import { DesktopShell, DesktopShellSkeleton, OsRouteNotFound } from "@/systems/os";

import { preloadAppRoute, prepareAppProfile } from "./_app/-app-preload";

function AppShellWithBilling() {
  return (
    <AccountUpgradeSheetProvider>
      <DesktopShell />
    </AccountUpgradeSheetProvider>
  );
}

export const Route = createFileRoute("/_app")({
  beforeLoad: ({ context }) => prepareAppProfile(context.queryClient),
  loader: ({ context }) => preloadAppRoute(context.queryClient),
  component: AppShellWithBilling,
  // Shown only when identity and workspace reads outlast the router's pending threshold.
  pendingComponent: DesktopShellSkeleton,
  notFoundComponent: OsRouteNotFound,
});
