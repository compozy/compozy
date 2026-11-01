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

  if (parsed.kind === "page") {
    return <AgentPageLocation name={parsed.name} page={parsed.page} />;
  }
  if (parsed.kind === "detail") {
    return <AgentDetailLocation name={parsed.name} search={parsed.search} />;
  }
  return <AgentsCatalogLocation search={parsed.search} />;
}
