import type { ReactNode } from "react";

import { useProfiles } from "@/systems/profiles";

import { SessionDeriveContext } from "../contexts/session-derive-context-value";
import { SessionForkContext } from "../contexts/session-fork-context-value";
import { SessionMenuSourceContext } from "../contexts/session-menu-source-context-value";
import {
  SessionProfileMoveContext,
  type SessionProfileMoveHandle,
  type SessionProfileMoveTarget,
} from "../contexts/session-profile-move-context-value";
import type { SessionDeriveHostState } from "../hooks/use-session-derive-host";
import type { SessionDerivePlacementHandlers } from "../hooks/use-session-derive";
import { SessionContinueDialog } from "./session-continue-dialog";
import { SessionForkDialog } from "./session-fork-dialog";
import { SessionMenuSource } from "./session-menu-source";
import { SessionProfileMoveDialog } from "./session-profile-move-dialog";

export interface SessionDeriveHostProps extends SessionDerivePlacementHandlers {
  host: SessionDeriveHostState;
  children?: ReactNode;
}

/**
 * Mounts the Continue, Fork, and Move to profile dialogs once for a surface and
 * hands their entry points (row menus, the provider-error marker, "Fork from
 * here") the request functions through context. Placement stays with the host:
 * only it knows which window "this" one is. Session menus under it read their
 * linked work and folder layout through `SessionMenuSource`.
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
  const profiles = useProfiles();
  const targets: SessionProfileMoveTarget[] = [];
  for (const profile of profiles.data ?? []) {
    if (profile.state === "active") targets.push({ id: profile.id, name: profile.name });
  }
  const profileMove: SessionProfileMoveHandle = { targets, request: host.requestProfileMove };
  return (
    <SessionDeriveContext value={host.request}>
      <SessionForkContext value={host.requestFork}>
        <SessionProfileMoveContext value={profileMove}>
          <SessionMenuSourceContext value={SessionMenuSource}>{children}</SessionMenuSourceContext>
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
          {target?.kind === "profile-move" && target.profile ? (
            <SessionProfileMoveDialog
              key={target.nonce}
              onOpenChange={host.setOpen}
              open={host.open}
              source={target.source}
              target={target.profile}
            />
          ) : null}
        </SessionProfileMoveContext>
      </SessionForkContext>
    </SessionDeriveContext>
  );
}
