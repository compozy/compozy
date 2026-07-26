import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadKnowledgeRoute } from "./-knowledge-preload";

export const Route = createFileRoute("/_app/knowledge")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Knowledge", to: "/knowledge" } },
  }),
  loader: ({ context }) => preloadKnowledgeRoute(context.queryClient),
  component: createOsRouteSync("knowledge"),
});
