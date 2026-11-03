import { useQueryClient } from "@tanstack/react-query";
import { useAtom } from "@xstate/store-react";
import { useEffect, useRef, useState } from "react";

import {
  ownerFromRow,
  profileViewStore,
  useProfileReadScope,
  useProfileSwitchEpoch,
} from "@/systems/profiles";
import { sessionAcrossProfilesOptions } from "@/systems/session";

import type { OsShellHandle } from "../contexts/os-shell-context";
import { foreignProfileSessionWindows, shownSessionId } from "../lib/foreign-profile-windows";
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
      const foreign = await foreignProfileSessionWindows(
        manager.getState().windows,
        request.profileId,
        async sessionId =>
          ownerFromRow(await queryClient.fetchQuery(sessionAcrossProfilesOptions(sessionId))).id
      );
      for (const { windowId, sessionId } of foreign) {
        // Any later view change, All profiles included, owns the desk now: stop closing.
        if (profileViewStore.getSnapshot().context.switchEpoch !== request.epoch) return;
        // The operator may have moved the window to another session while owners were read or
        // an earlier close was pending; only the session that was inspected licenses the close.
        if (shownSessionId(manager.getState().windows[windowId]) !== sessionId) continue;
        // react-doctor-disable-next-line react-doctor/async-await-in-loop -- one close at a time is the point: the runtime's close guard admits a single close and refuses the rest while it is pending.
        await coordinator.userClose(windowId);
      }
    })();
  }, [live, queryClient, request, shell, targetProfileId, workspaceId]);
}
