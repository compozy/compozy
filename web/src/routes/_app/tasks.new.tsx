import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import { validateTaskCreateSearch } from "@/systems/tasks";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/tasks/new")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "New task" } },
  }),
  validateSearch: validateTaskCreateSearch,
  component: createOsRouteSync("tasks"),
});
