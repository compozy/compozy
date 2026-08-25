import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";

/**
 * One call's record.
 *
 * `/agents/calls/…` is a static prefix, so it wins over `/agents/$name` and a
 * call id can never be mistaken for an agent name.
 */
export const Route = createFileRoute("/_app/agents/calls/$callId")({
  component: createOsRouteSync("agents"),
});
