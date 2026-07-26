import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import { toast } from "sonner";

import {
  redirectSessionPermalinkRoute,
  type SessionPermalinkRouteContext,
} from "./-session-permalink-route";

/**
 * Permalink-by-id redirect. Resolves the agent name for a session and
 * forwards to the canonical `/agents/$name/sessions/$id` route. Used by
 * external surfaces (automation history, task tree) that hold a session id
 * without the originating agent in scope. On resolution failure the desktop
 * stays up; the error surfaces as a toast.
 */
export const Route = createFileRoute("/_app/session/$id")({
  beforeLoad: redirectSessionPermalinkRoute,
  component: SessionPermalinkFallback,
});

function SessionPermalinkFallback() {
  const routeContext = Route.useRouteContext() as SessionPermalinkRouteContext;
  const message = routeContext.permalinkError ?? "Session not found";
  useEffect(() => {
    toast.error(message);
  }, [message]);
  return null;
}
