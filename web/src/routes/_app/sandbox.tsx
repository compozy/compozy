import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";
import { validateSandboxSearch } from "@/systems/sandbox";

export const Route = createFileRoute("/_app/sandbox")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Sandbox", to: "/sandbox" } },
  }),
  validateSearch: validateSandboxSearch,
  loader: async ({ context }) =>
    (await import("./-settings-preload")).preloadSandboxRoute(context.queryClient),
  component: createOsRouteSync("sandbox"),
});
