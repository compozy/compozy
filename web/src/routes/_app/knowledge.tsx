import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/knowledge")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Knowledge", to: "/knowledge" } },
  }),
  loader: async ({ context }) =>
    (await import("./-knowledge-preload")).preloadKnowledgeRoute(context.queryClient),
  component: createOsRouteSync("knowledge"),
});
