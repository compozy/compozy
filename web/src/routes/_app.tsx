import { createFileRoute } from "@tanstack/react-router";

import { preloadAppRoute, prepareAppProfile } from "./_app/-app-preload";
import { DesktopShell, OsRouteNotFound } from "@/systems/os";

export const Route = createFileRoute("/_app")({
  beforeLoad: ({ context }) => prepareAppProfile(context.queryClient),
  loader: ({ context }) => preloadAppRoute(context.queryClient),
  component: DesktopShell,
  notFoundComponent: OsRouteNotFound,
});
