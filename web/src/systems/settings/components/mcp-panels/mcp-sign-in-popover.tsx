import type { MCPAuthorizePhase, UseMCPAuthorizeReturn } from "../../hooks/use-mcp-authorize";
import { reopenMCPSignInPage } from "../../lib/mcp-sign-in-window";
import type { SettingsMCPServerEntry } from "../../types";

import { MCPAnchoredPanel, type MCPAnchoredPanelProps } from "./mcp-anchored-panel";
import { MCPSignInPanel, type MCPSignInPhase } from "./mcp-sign-in-panel";

const PANEL_PHASE: Record<Exclude<MCPAuthorizePhase, "idle">, MCPSignInPhase> = {
  reviewing_scopes: "scopes",
  beginning: "starting",
  waiting: "waiting",
  manual: "paste",
  exchanging: "finishing",
  confirmed: "connected",
  failed: "failed",
};

function hostOf(...candidates: Array<string | null | undefined>): string | undefined {
  for (const candidate of candidates) {
    if (!candidate) continue;
    try {
      return new URL(candidate).host;
    } catch {
      // Not an absolute address; try the next one.
    }
  }
  return undefined;
}

export interface MCPSignInPopoverProps {
  authorize: UseMCPAuthorizeReturn;
  /** The live entry for the server signing in: its address and, once connected, its tools. */
  server: SettingsMCPServerEntry | null;
  appliesOn?: "now" | "next-message";
  side?: MCPAnchoredPanelProps["side"];
  align?: MCPAnchoredPanelProps["align"];
}

/**
 * The sign-in panel beside the control that started it, driven by `useMCPAuthorize`. Closing it
 * cancels the attempt; credentials change only on a confirmed sign-in.
 */
export function MCPSignInPopover({
  authorize,
  server,
  appliesOn = "now",
  side,
  align,
}: MCPSignInPopoverProps) {
  const phase = authorize.phase;
  const url = authorize.begin?.authorization_url ?? undefined;
  const name = authorize.server ?? server?.name ?? "";
  const toolCount = server?.runtime_status?.tool_count;
  return (
    <MCPAnchoredPanel
      anchor={authorize.anchor}
      open={phase !== "idle"}
      onClose={authorize.cancel}
      side={side}
      align={align}
    >
      {phase === "idle" ? null : (
        <MCPSignInPanel
          key={name}
          server={name}
          phase={PANEL_PHASE[phase]}
          host={hostOf(url, server?.url)}
          url={url}
          manual={authorize.mode === "manual"}
          canFinishByPaste={phase === "failed" && authorize.begin !== null}
          error={authorize.error}
          scopes={authorize.approvedScopes}
          toolCount={toolCount ?? undefined}
          appliesOn={appliesOn}
          onClose={authorize.cancel}
          onContinue={authorize.confirmScopeEscalation}
          onRetry={authorize.retryBegin}
          onOpenAgain={() => {
            if (url) reopenMCPSignInPage(url);
          }}
          onRequestPaste={authorize.enterManual}
          onSubmitPaste={authorize.submitManual}
        />
      )}
    </MCPAnchoredPanel>
  );
}
