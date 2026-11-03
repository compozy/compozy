import { useQueryClient } from "@tanstack/react-query";
import { useAtom } from "@xstate/store-react";
import { useEffect, useRef, useState } from "react";

import { ownerFromRow, useProfileReadScope, useProfileSwitchEpoch } from "@/systems/profiles";
import { sessionAcrossProfilesOptions } from "@/systems/session";

import type { OsShellHandle } from "../contexts/os-shell-context";
import { foreignProfileSessionWindowIds } from "../lib/foreign-profile-windows";
import type { OsDesktopRuntimeStore } from "../lib/os-types";

interface SweepRequest {
  epoch: number;
  workspaceId: string;
  profileId: string;
}

/** The desk is live for its binding: hydrated, attached, and fenced by its stream. */
function selectDeskLive(state: OsDesktopRuntimeStore): boolean {
  return (
    state.hydration === "live" && state.client !== null && state.connectionStatus === "connected"
  );
}

/**
 * Closes another profile's session windows when the operator switches into one profile.
 *
 * Desks belong to one profile, and under All profiles the client works on the destination's desk,
 * so sessions of every profile opened there are stored in it. A switch to that profile would leave
 * them on screen under an owner banner. The switch is the moment they stop belonging: once the
 * target desk is live, each window whose session another profile owns closes through the normal
 * close command, so it stays reopenable and the other profile's own desk is untouched. Only an
 * explicit switch triggers this; a deep link opened while a profile is selected keeps its banner.
 */
export function useProfileSwitchWindowSweep(shell: OsShellHandle, workspaceId: string | null) {
  const queryClient = useQueryClient();
  const scope = useProfileReadScope();
  const epoch = useProfileSwitchEpoch();
  const targetProfileId = scope.aggregate ? null : (scope.destinationOwner?.id ?? null);
  const [seenEpoch, setSeenEpoch] = useState(epoch);
  const [request, setRequest] = useState<SweepRequest | null>(null);
  if (epoch !== seenEpoch) {
    setSeenEpoch(epoch);
    setRequest(
      scope.aggregate || workspaceId === null
        ? null
        : { epoch, workspaceId, profileId: targetProfileId ?? "" }
    );
  }
  // The profile list can resolve the target's id after the switch commits.
  if (request !== null && request.profileId === "" && targetProfileId !== null) {
    setRequest({ ...request, profileId: targetProfileId });
  }
  const live = useAtom(shell.projection, selectDeskLive);
  const started = useRef<SweepRequest | null>(null);

  useEffect(() => {
    if (request === null || request.profileId === "" || !live) return;
    if (started.current === request || request.workspaceId !== workspaceId) return;
    if (request.profileId !== targetProfileId) return;
    const { coordinator, manager } = shell;
    // The render saw the previous binding; the rebind in this commit is what has to be live.
    if (!selectDeskLive(manager.getState())) return;
    started.current = request;
    void (async () => {
      const foreign = await foreignProfileSessionWindowIds(
        manager.getState().windows,
        request.profileId,
        async sessionId =>
          ownerFromRow(await queryClient.fetchQuery(sessionAcrossProfilesOptions(sessionId))).id
      );
      // A newer switch owns the desk now; its own sweep reads it again.
      if (started.current !== request) return;
      // The runtime queues commands and each reads the revision current when it runs, and a
      // rebind resolves the queued ones unapplied, so the closes can be issued together.
      await Promise.all(foreign.map(windowId => coordinator.userClose(windowId)));
    })();
  }, [live, queryClient, request, shell, targetProfileId, workspaceId]);
}
