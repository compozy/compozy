import { AlertCircle, Plug } from "@compozy/ui/icons";

import { Marker } from "@compozy/ui";

import { mcpReloadDetail, type SessionMCPReloadView } from "../lib/session-mcp-reload-event";

export interface SessionMCPReloadMarkerProps {
  reload: SessionMCPReloadView;
}

/**
 * One quiet transcript line per MCP reload at a message boundary (`_uiux.md` S4). A failed reload
 * says the message wasn't sent: it stays in the composer, and sending it again retries.
 */
export function SessionMCPReloadMarker({ reload }: SessionMCPReloadMarkerProps) {
  if (reload.outcome === "failed") {
    return (
      <Marker icon={<AlertCircle />} role="alert" tone="warning" data-testid="session-mcp-reload">
        <b>Couldn't start {reload.runtime ?? "the coding agent"} with the new MCP servers</b> · your
        message wasn't sent
      </Marker>
    );
  }
  return (
    <Marker icon={<Plug />} role="status" tone="neutral" data-testid="session-mcp-reload">
      <b>MCP servers updated</b> · {mcpReloadDetail(reload)}
    </Marker>
  );
}
