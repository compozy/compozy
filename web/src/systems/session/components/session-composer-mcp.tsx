import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { MCPAuthorizeDialog } from "@/systems/settings";

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
 * Feeds the composer's MCP control. The session's `mcp_summary` drives the trigger at no cost;
 * the session MCP route is read only while the popover is open, and the stream's
 * `session.mcp_changed` / `session.mcp_reloaded` events reread both.
 */
export function SessionComposerMCP({ session }: SessionComposerMCPProps) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const summary = sessionMCPSummary(session);
  const workspaceId = session.workspace_id ?? "";
  const model = useSessionMCP(
    workspaceId,
    session.id,
    session.profile_name,
    open && summary !== null
  );
  const manage = () => void navigate({ to: MCP_SETTINGS_PATH });
  const hasActions =
    model.data?.compozy.some(row => sessionMCPStateView(row.state, "").action !== undefined) ??
    false;
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
        actionsDisabled={actions.resolving}
        onManage={manage}
        onOpenChange={setOpen}
      />
      <MCPAuthorizeDialog
        authorize={actions.authorization.authorize}
        scope={actions.authorization.scope}
        server={actions.authorization.server}
      />
    </>
  );
}
