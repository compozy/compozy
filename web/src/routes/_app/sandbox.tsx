import { createFileRoute } from "@tanstack/react-router";

import { validateSandboxSearch } from "@/systems/sandbox";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSandboxRoute } from "./-settings-preload";

export const Route = createFileRoute("/_app/sandbox")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Sandbox", to: "/sandbox" } },
  }),
  validateSearch: validateSandboxSearch,
  loader: ({ context }) => preloadSandboxRoute(context.queryClient),
  component: createOsRouteSync("sandbox"),
});
