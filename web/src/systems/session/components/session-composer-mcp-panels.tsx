import {
  MCPAllowPanel,
  MCPAnchoredPanel,
  MCPIssuePanel,
  MCPSignInPopover,
  mcpAllowPanelLaunch,
} from "@/systems/settings";

import type { SessionMCPResponse } from "../adapters/session-mcp-api";
import type { SessionMCPActions } from "../hooks/use-session-mcp-actions";
import { sessionMCPCheckedRow } from "../hooks/use-session-mcp-check";

export interface SessionComposerMCPPanelsProps {
  data: SessionMCPResponse | null;
  actions: SessionMCPActions;
  onOpenSettings: () => void;
}

/**
 * The composer's row panels, each beside the row button that opened it and inside the popover so
 * pressing them keeps it open: sign in, review a project server before allowing it, and why a
 * server is not working. The allow and issue panels read the row live from the session's MCP read,
 * so a check's answer shows in the open panel.
 */
export function SessionComposerMCPPanels({
  data,
  actions,
  onOpenSettings,
}: SessionComposerMCPPanelsProps) {
  const panel = actions.panel;
  const row = panel && data ? sessionMCPCheckedRow(data, panel.target) : undefined;
  const source = row?.source.path?.trim() || undefined;
  return (
    <>
      <MCPSignInPopover
        authorize={actions.authorization.authorize}
        server={actions.authorization.server}
        appliesOn="next-message"
        side="right"
        align="start"
      />
      <MCPAnchoredPanel
        anchor={panel?.anchor ?? null}
        open={panel !== null && row !== undefined}
        onClose={actions.closePanel}
        side="right"
        align="start"
      >
        {panel?.kind === "allow" && row ? (
          <MCPAllowPanel
            server={row.name}
            reason={row.approval?.reason === "changed" ? "changed" : "added"}
            source={source}
            launch={mcpAllowPanelLaunch(row)}
            appliesOn="next-message"
            allowing={actions.allowing}
            onAllow={() => actions.allow(row)}
            onClose={actions.closePanel}
          />
        ) : null}
        {panel?.kind === "check" && row ? (
          <MCPIssuePanel
            server={row.name}
            reason={row.reason}
            checking={actions.checking}
            stillFailing={actions.stillFailing}
            onCheck={() => actions.check(panel.target)}
            onOpenSettings={() => {
              actions.closePanel();
              onOpenSettings();
            }}
            onClose={actions.closePanel}
          />
        ) : null}
      </MCPAnchoredPanel>
    </>
  );
}
