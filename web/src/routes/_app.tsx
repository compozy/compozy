import { createFileRoute } from "@tanstack/react-router";

import { preloadAppRoute } from "./_app/-app-preload";
import { DesktopShell, OsRouteNotFound } from "@/systems/os";

export const Route = createFileRoute("/_app")({
  loader: ({ context }) => preloadAppRoute(context.queryClient),
  component: DesktopShell,
  notFoundComponent: OsRouteNotFound,
});
