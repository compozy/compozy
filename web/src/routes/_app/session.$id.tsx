import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import { toast } from "sonner";

import { validateSessionDeepLinkSearch } from "@/systems/session";
import {
  redirectSessionPermalinkRoute,
  type SessionPermalinkRouteContext,
} from "./-session-permalink-route";
import { SessionWorkspaceSwitchRouteDecision } from "./-session-workspace-switch";

/**
 * Permalink-by-id redirect. Resolves the agent name for a session and
 * forwards to the canonical `/agents/$name/sessions/$id` route. Used by
 * external surfaces (automation history, task tree) that hold a session id
 * without the originating agent in scope. A session owned by another workspace
 * offers the routed switch confirmation; every other resolution failure keeps
 * the desktop up and surfaces as a toast.
 */
export const Route = createFileRoute("/_app/session/$id")({
  validateSearch: validateSessionDeepLinkSearch,
  beforeLoad: redirectSessionPermalinkRoute,
  component: SessionPermalinkFallback,
});

function SessionPermalinkFallback() {
  const routeContext = Route.useRouteContext() as SessionPermalinkRouteContext;
  const { workspaceSwitch } = Route.useSearch();
  const params = Route.useParams();
  const navigate = Route.useNavigate();
  const owner = routeContext.sessionOwner;
  const message = owner ? null : (routeContext.permalinkError ?? "Session not found");
  useEffect(() => {
    if (message) toast.error(message);
  }, [message]);

  if (!owner) return null;

  return (
    <SessionWorkspaceSwitchRouteDecision
      open={workspaceSwitch === "confirm"}
      owner={owner}
      onReenter={() => {
        void navigate({ to: "/session/$id", params, search: {}, replace: true });
      }}
      onDecline={() => {
        void navigate({ search: { workspaceSwitch: "declined" }, replace: true });
      }}
    />
  );
}
