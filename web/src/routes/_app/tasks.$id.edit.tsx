import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import { validateTaskDetailSearch } from "@/systems/tasks";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/tasks/$id/edit")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Edit" } },
  }),
  // The editor is a dialog over the task detail, so the detail's own tab has to
  // survive the location or dismissal would drop the operator on Overview.
  validateSearch: validateTaskDetailSearch,
  component: createOsRouteSync("tasks"),
});
