import {
  MCPAllowPanel,
  MCPAnchoredPanel,
  MCPIssuePanel,
  MCPSignInPopover,
  mcpAllowPanelLaunch,
} from "@/systems/settings";

import type { SessionMCPActions } from "../hooks/use-session-mcp-actions";
import { sessionMCPServerTarget } from "../lib/session-mcp-view";

export interface SessionComposerMCPPanelsProps {
  actions: SessionMCPActions;
  onOpenSettings: () => void;
}

/**
 * The composer's row panels, each beside the row button that opened it and inside the popover so
 * pressing them keeps it open: sign in, review a project server before allowing it, and why a
 * server is not working. The allow and issue panels show the server's live row from the session's
 * MCP read, so a check's answer shows in the open panel.
 */
export function SessionComposerMCPPanels({
  actions,
  onOpenSettings,
}: SessionComposerMCPPanelsProps) {
  const panel = actions.panel;
  const row = panel?.row;
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
        open={panel !== null}
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
            allowing={actions.panelAllowing}
            onAllow={() => actions.allow(row)}
            onClose={actions.closePanel}
          />
        ) : null}
        {panel?.kind === "check" && row ? (
          <MCPIssuePanel
            server={row.name}
            reason={row.reason}
            local={row.transport === "stdio"}
            checking={actions.panelChecking}
            stillFailing={actions.panelStillFailing}
            onCheck={() => actions.check(sessionMCPServerTarget(row))}
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
