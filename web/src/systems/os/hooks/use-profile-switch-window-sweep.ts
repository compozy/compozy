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

const CLOSE_RETRY_DELAY_MS = 250;

/**
 * Closes one window while it is still the foreign one the sweep inspected. The runtime refuses a
 * close while another one is pending, and a terminal's close confirmation can hold that for a
 * while, so a refused close is asked again until it lands or the window stops being foreign
 * (closed, moved to another session, the view changed, or the shell went away).
 */
async function closeWhileForeign(close: () => Promise<boolean>, stillForeign: () => boolean) {
  while (stillForeign()) {
    // react-doctor-disable-next-line react-doctor/async-await-in-loop -- each attempt depends on the previous one being refused.
    if (await close()) return;
    // react-doctor-disable-next-line react-doctor/async-await-in-loop -- the retry waits for the refusing close to settle before asking again.
    await new Promise(resolve => setTimeout(resolve, CLOSE_RETRY_DELAY_MS));
  }
}

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
  const mounted = useRef(false);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

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
        // react-doctor-disable-next-line react-doctor/async-await-in-loop -- one close at a time is the point: the runtime's close guard admits a single close and refuses the rest while it is pending.
        await closeWhileForeign(
          () => coordinator.userClose(windowId),
          // Any later view change, All profiles included, owns the desk now; the operator may have
          // moved the window to another session, so only the inspected one licenses a close; and
          // nothing retries after the shell unmounts.
          () =>
            mounted.current &&
            profileViewStore.getSnapshot().context.switchEpoch === request.epoch &&
            shownSessionId(manager.getState().windows[windowId]) === sessionId
        );
      }
    })();
  }, [live, queryClient, request, shell, targetProfileId, workspaceId]);
}
