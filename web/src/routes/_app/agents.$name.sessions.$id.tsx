import { createFileRoute, redirect } from "@tanstack/react-router";
import { toast } from "sonner";

import { SessionWorkspaceSwitchRouteDecision } from "./-session-workspace-switch";
import { createOsRouteSync } from "@/systems/os";
import { validateSessionDeepLinkSearch } from "@/systems/session";

const SessionRouteSync = createOsRouteSync("session");

export const Route = createFileRoute("/_app/agents/$name/sessions/$id")({
  beforeLoad: () => ({
    topbar: { crumb: { label: "Session" } },
  }),
  validateSearch: validateSessionDeepLinkSearch,
  loaderDeps: ({ search }) => ({ workspaceSwitch: search.workspaceSwitch }),
  loader: async ({ context, params, deps, preload }) => {
    const { prefetchAgentSessionRoute } = await import("./-agent-session-route-loader");
    const data = await prefetchAgentSessionRoute({
      queryClient: context.queryClient,
      sessionId: params.id,
    });
    if (preload || data.status === "loaded") {
      return data;
    }
    // The URL owns whether the confirmation is open, so a foreign deep link canonicalizes into it.
    if (data.status === "foreign" && deps.workspaceSwitch !== "declined") {
      if (deps.workspaceSwitch !== "confirm") {
        throw redirect({
          to: "/agents/$name/sessions/$id",
          params,
          search: { workspaceSwitch: "confirm" },
          replace: true,
        });
      }
      return data;
    }
    toast.error("Session not found");
    throw redirect({
      to: "/agents/$name",
      params: { name: params.name },
      replace: true,
    });
  },
  component: AgentSessionRoute,
});

function AgentSessionRoute() {
  const data = Route.useLoaderData();
  const { workspaceSwitch } = Route.useSearch();
  const params = Route.useParams();
  const navigate = Route.useNavigate();

  // The session window must not open on a foreign id: the route stays unreported until the
  // session belongs to the active workspace.
  if (data.status !== "foreign") {
    return <SessionRouteSync />;
  }

  return (
    <SessionWorkspaceSwitchRouteDecision
      open={workspaceSwitch === "confirm"}
      owner={data.owner}
      onReenter={() => {
        void navigate({
          to: "/agents/$name/sessions/$id",
          params,
          search: {},
          replace: true,
        });
      }}
      onDecline={() => {
        void navigate({ search: { workspaceSwitch: "declined" }, replace: true });
      }}
    />
  );
}
