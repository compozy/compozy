import { ProfilePin } from "@/systems/profiles";

import { useDesktop } from "../../hooks/use-desktop";
import { AgentDetailLocation } from "./agent-detail-location";
import { AgentPageLocation } from "./agent-page-location";
import { parseAgentWindowLocation } from "./agent-window-location";
import { AgentsCatalogLocation } from "./agents-catalog-location";

const DEFAULT_AGENTS_ROUTE = { pathname: "/agents", search: {} } as const;

/** Agents app controller driven exclusively by the logical window's WM location. */
export function AgentsWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_AGENTS_ROUTE);
  const parsed = parseAgentWindowLocation(location);

  // An agent window reads in the agent's own profile when it carries one (#1183).
  if (parsed.kind === "page") {
    return (
      <ProfilePin profile={parsed.search.profile}>
        <AgentPageLocation name={parsed.name} page={parsed.page} />
      </ProfilePin>
    );
  }
  if (parsed.kind === "detail") {
    return (
      <ProfilePin profile={parsed.search.profile}>
        <AgentDetailLocation name={parsed.name} search={parsed.search} />
      </ProfilePin>
    );
  }
  return <AgentsCatalogLocation search={parsed.search} />;
}
