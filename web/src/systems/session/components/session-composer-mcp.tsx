import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { MCPSignInPopover } from "@/systems/settings";

import { useSessionMCP } from "../hooks/use-session-mcp";
import { useSessionMCPActions } from "../hooks/use-session-mcp-actions";
import { sessionMCPStateView, sessionMCPSummary } from "../lib/session-mcp-view";
import type { SessionPayload } from "../types";
import { MCPComposerControl } from "./mcp-composer-control";

const MCP_SETTINGS_PATH = "/settings/mcp";

export interface SessionComposerMCPProps {
  session: SessionPayload;
}

/**
 * Feeds the composer's MCP control. The session's `mcp_summary` drives the trigger; the session MCP
 * route is read while the popover is open or a change is pending (where a failed reload shows), and
 * the stream's `session.mcp_changed` / `session.mcp_reloaded` events reread both.
 */
export function SessionComposerMCP({ session }: SessionComposerMCPProps) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const summary = sessionMCPSummary(session);
  const workspaceId = session.workspace_id ?? "";
  const model = useSessionMCP(workspaceId, session.id, session.profile_name, {
    open: open && summary !== null,
    pendingReload: summary?.pending_reload === true,
  });
  const manage = () => void navigate({ to: MCP_SETTINGS_PATH });
  const hasActions =
    model.data?.compozy_servers.some(
      row => sessionMCPStateView(row.state, "").action !== undefined
    ) ?? false;
  const actions = useSessionMCPActions({
    workspaceId,
    sessionId: session.id,
    profile: session.profile_name,
    enabled: open && hasActions,
    onUnresolved: manage,
  });
  if (summary === null) return null;
  return (
    <>
      <MCPComposerControl
        summary={summary}
        data={model.data}
        loading={model.loading}
        failed={model.failed}
        onRetry={model.retry}
        onAction={actions.act}
        actingName={actions.actingName}
        actionsDisabled={actions.resolving || actions.allowing}
        onManage={manage}
        rowPanel={
          <MCPSignInPopover
            authorize={actions.authorization.authorize}
            server={actions.authorization.server}
            appliesOn="next-message"
            side="right"
            align="start"
          />
        }
        onOpenChange={next => {
          setOpen(next);
          // The sign-in panel lives inside the popover: closing the popover ends the attempt.
          if (!next) actions.authorization.authorize.cancel();
          // A pending change keeps the read running while closed, so opening alone would not
          // reread an answer that has gone out of date.
          if (next && summary.pending_reload) model.refresh();
        }}
      />
    </>
  );
}
