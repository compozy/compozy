import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/files")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Files", to: "/files" } },
  }),
  component: createOsRouteSync("files"),
});
