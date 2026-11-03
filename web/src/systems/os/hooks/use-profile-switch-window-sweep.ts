import { useQueryClient } from "@tanstack/react-query";
import { useAtom } from "@xstate/store-react";
import { useEffect, useRef, useState } from "react";

import { profileViewStore, useProfileReadScope, useProfileSwitchEpoch } from "@/systems/profiles";

import type { OsShellHandle } from "../contexts/os-shell-context";
import {
  foreignProfileWindows,
  sameShownItem,
  shownProfileItem,
} from "../lib/foreign-profile-windows";
import type { OsDesktopRuntimeStore } from "../lib/os-types";
import { profileItemOwnerLookup } from "../lib/profile-item-owner";

const CLOSE_RETRY_FIRST_DELAY_MS = 250;
const CLOSE_RETRY_MAX_DELAY_MS = 5_000;

/**
 * Closes one window while it is still the foreign one the sweep inspected. The runtime refuses a
 * close while another one is pending, and a terminal's close confirmation can hold that for a
 * while, so a refused close is asked again until it lands or the window stops being foreign
 * (closed, moved to another session, the view changed, or the shell went away). A refusal can
 * also be a failed command, so the wait doubles up to a cap rather than hammering the daemon.
 */
async function closeWhileForeign(close: () => Promise<boolean>, stillForeign: () => boolean) {
  let delay = CLOSE_RETRY_FIRST_DELAY_MS;
  while (stillForeign()) {
    // react-doctor-disable-next-line react-doctor/async-await-in-loop -- each attempt depends on the previous one being refused.
    if (await close()) return;
    const wait = delay;
    // react-doctor-disable-next-line react-doctor/async-await-in-loop -- the retry waits for the refusing close to settle before asking again.
    await new Promise(resolve => setTimeout(resolve, wait));
    delay = Math.min(delay * 2, CLOSE_RETRY_MAX_DELAY_MS);
  }
}

interface SweepRequest {
  epoch: number;
  workspaceId: string;
  profileId: string;
  profileName: string;
}

/** The desk is live for its binding: hydrated, attached, and fenced by its stream. */
function selectDeskLive(state: OsDesktopRuntimeStore): boolean {
  return (
    state.hydration === "live" && state.client !== null && state.connectionStatus === "connected"
  );
}

/**
 * Closes another profile's windows when the operator switches into one profile.
 *
 * Desks belong to one profile, and under All profiles the client works on the destination's desk,
 * so sessions, terminals, pinned agents and automations of every profile opened there are stored
 * in it. A switch to that profile would leave them on screen, resolving under the wrong profile.
 * The switch is the moment they stop belonging: once the target desk is live, each window whose
 * item another profile owns closes through the normal close command, so it stays reopenable and
 * the other profile's own desk is untouched. Only an explicit switch triggers this; a deep link
 * opened while a profile is selected keeps its owner banner.
 */
export function useProfileSwitchWindowSweep(shell: OsShellHandle, workspaceId: string | null) {
  const queryClient = useQueryClient();
  const scope = useProfileReadScope();
  const epoch = useProfileSwitchEpoch();
  const targetProfileId = scope.aggregate ? null : (scope.destinationOwner?.id ?? null);
  const targetProfileName = scope.destination;
  const [seenEpoch, setSeenEpoch] = useState(epoch);
  const [request, setRequest] = useState<SweepRequest | null>(null);
  if (epoch !== seenEpoch) {
    setSeenEpoch(epoch);
    setRequest(
      scope.aggregate || workspaceId === null
        ? null
        : { epoch, workspaceId, profileId: targetProfileId ?? "", profileName: targetProfileName }
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
      const foreign = await foreignProfileWindows(
        manager.getState().windows,
        request.workspaceId,
        { id: request.profileId, name: request.profileName },
        profileItemOwnerLookup(queryClient)
      );
      for (const { windowId, item } of foreign) {
        // react-doctor-disable-next-line react-doctor/async-await-in-loop -- one close at a time is the point: the runtime's close guard admits a single close and refuses the rest while it is pending.
        await closeWhileForeign(
          () => coordinator.userClose(windowId),
          // Any later view change, All profiles included, owns the desk now; the operator may have
          // moved the window to another item, so only the inspected one licenses a close; and
          // nothing retries after the shell unmounts.
          () =>
            mounted.current &&
            profileViewStore.getSnapshot().context.switchEpoch === request.epoch &&
            sameShownItem(
              shownProfileItem(manager.getState().windows[windowId], request.workspaceId),
              item
            )
        );
      }
    })();
  }, [live, queryClient, request, shell, targetProfileId, workspaceId]);
}
