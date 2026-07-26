import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import { validateTaskDetailSearch } from "@/systems/tasks";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/tasks/$id")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: {
      crumb:
        params.id === "network"
          ? { label: "Network wakes" }
          : { label: `Task ${params.id}`, params: { id: params.id }, to: "/tasks/$id" },
    },
  }),
  validateSearch: validateTaskDetailSearch,
  component: createOsRouteSync("tasks"),
});
