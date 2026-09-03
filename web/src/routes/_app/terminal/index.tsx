import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/terminal/")({
  component: createOsRouteSync("terminal"),
});
