import type { ReactNode } from "react";

import { SessionDeriveContext } from "../contexts/session-derive-context-value";
import { SessionForkContext } from "../contexts/session-fork-context-value";
import type { SessionDeriveHostState } from "../hooks/use-session-derive-host";
import type { SessionDerivePlacementHandlers } from "../hooks/use-session-derive";
import { SessionContinueDialog } from "./session-continue-dialog";
import { SessionForkDialog } from "./session-fork-dialog";

export interface SessionDeriveHostProps extends SessionDerivePlacementHandlers {
  host: SessionDeriveHostState;
  children?: ReactNode;
}

/**
 * Mounts the Continue and Fork dialogs once for a surface and hands their entry
 * points (row menus, the provider-error marker, "Fork from here") the request
 * functions through context. Placement stays with the host: only it knows
 * which window "this" one is.
 */
export function SessionDeriveHost({
  host,
  openInNewWindow,
  openInThisWindow,
  children,
}: SessionDeriveHostProps) {
  const { target } = host;
  const placement = { openInNewWindow, openInThisWindow };
  const workspaceId = target?.source.workspace_id?.trim() || host.workspaceId;
  return (
    <SessionDeriveContext value={host.request}>
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
    </SessionDeriveContext>
  );
}
