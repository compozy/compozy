import type { ReactNode } from "react";

import { SessionContinueContext } from "../contexts/session-continue-context-value";
import type { SessionContinueHostState } from "../hooks/use-session-continue-host";
import type { SessionDerivePlacementHandlers } from "../hooks/use-session-derive";
import { SessionContinueDialog } from "./session-continue-dialog";

export interface SessionContinueHostProps extends SessionDerivePlacementHandlers {
  host: SessionContinueHostState;
  children?: ReactNode;
}

/**
 * Mounts the Continue dialog once for a surface and hands its entry points
 * (row menus, the provider-error marker) the request function through context.
 * Placement stays with the host: only it knows which window "this" one is.
 */
export function SessionContinueHost({
  host,
  openInNewWindow,
  openInThisWindow,
  children,
}: SessionContinueHostProps) {
  const { target } = host;
  return (
    <SessionContinueContext value={host.request}>
      {children}
      {target ? (
        <SessionContinueDialog
          key={target.nonce}
          onOpenChange={host.setOpen}
          open={host.open}
          placement={{ openInNewWindow, openInThisWindow }}
          source={target.source}
          workspaceId={target.source.workspace_id?.trim() || host.workspaceId}
        />
      ) : null}
    </SessionContinueContext>
  );
}
