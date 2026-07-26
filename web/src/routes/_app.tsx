import { createFileRoute } from "@tanstack/react-router";

import { DesktopShell, OsRouteNotFound } from "@/systems/os";
import { preloadAppRoute } from "./_app/-app-preload";

export const Route = createFileRoute("/_app")({
  loader: ({ context }) => preloadAppRoute(context.queryClient),
  component: DesktopShell,
  notFoundComponent: OsRouteNotFound,
});
