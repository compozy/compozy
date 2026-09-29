import type { ReactNode } from "react";

import { SessionContinueContext } from "../contexts/session-continue-context-value";
import { SessionForkContext } from "../contexts/session-fork-context-value";
import type { SessionContinueHostState } from "../hooks/use-session-continue-host";
import type { SessionDerivePlacementHandlers } from "../hooks/use-session-derive";
import { SessionContinueDialog } from "./session-continue-dialog";
import { SessionForkDialog } from "./session-fork-dialog";

export interface SessionContinueHostProps extends SessionDerivePlacementHandlers {
  host: SessionContinueHostState;
  children?: ReactNode;
}

/**
 * Mounts the Continue and Fork dialogs once for a surface and hands their entry
 * points (row menus, the provider-error marker, "Fork from here") the request
 * functions through context. Placement stays with the host: only it knows
 * which window "this" one is.
 */
export function SessionContinueHost({
  host,
  openInNewWindow,
  openInThisWindow,
  children,
}: SessionContinueHostProps) {
  const { target } = host;
  const placement = { openInNewWindow, openInThisWindow };
  const workspaceId = target?.source.workspace_id?.trim() || host.workspaceId;
  return (
    <SessionContinueContext value={host.request}>
      <SessionForkContext value={host.requestFork}>
        {children}
        {target?.kind === "continue" ? (
          <SessionContinueDialog
            key={target.nonce}
            onOpenChange={host.setOpen}
            open={host.open}
            placement={placement}
            source={target.source}
            workspaceId={workspaceId}
          />
        ) : null}
        {target?.kind === "fork" ? (
          <SessionForkDialog
            key={target.nonce}
            onOpenChange={host.setOpen}
            open={host.open}
            placement={placement}
            point={target.point ?? null}
            source={target.source}
            workspaceId={workspaceId}
          />
        ) : null}
      </SessionForkContext>
    </SessionContinueContext>
  );
}
