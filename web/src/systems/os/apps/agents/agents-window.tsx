import { useDesktop } from "../../hooks/use-desktop";
import { AgentCallLocation } from "./agent-call-location";
import { AgentDetailLocation } from "./agent-detail-location";
import { AgentSettingsLocation } from "./agent-settings-location";
import { parseAgentWindowLocation } from "./agent-window-location";
import { AgentsActivityLocation } from "./agents-activity-location";
import { AgentsCatalogLocation } from "./agents-catalog-location";

const DEFAULT_AGENTS_ROUTE = { pathname: "/agents", search: {} } as const;

/** Agents app controller driven exclusively by the logical window's WM location. */
export function AgentsWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_AGENTS_ROUTE);
  const parsed = parseAgentWindowLocation(location);

  if (parsed.kind === "activity") {
    return <AgentsActivityLocation windowId={windowId} search={parsed.search} />;
  }
  if (parsed.kind === "call") {
    return <AgentCallLocation key={parsed.callId} callId={parsed.callId} windowId={windowId} />;
  }
  if (parsed.kind === "settings") {
    return (
      <>
        <AgentDetailLocation name={parsed.name} rawSearch={{}} windowId={windowId} />
        <AgentSettingsLocation name={parsed.name} rawSearch={parsed.search} />
      </>
    );
  }
  if (parsed.kind === "detail") {
    return <AgentDetailLocation name={parsed.name} rawSearch={parsed.search} windowId={windowId} />;
  }
  return <AgentsCatalogLocation search={parsed.search} />;
}
