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
  /** The profile switched into, by name (the view, pinned agents) and id (labeled owners). */
  profile: string;
  profileId: string;
}

/** The partitions (projects, All projects, User) already swept for one switch. */
interface SweptPartitions {
  epoch: number;
  partitions: Set<string>;
}

/** The desk is live for its binding: hydrated, attached, and fenced by its stream. */
function selectDeskLive(state: OsDesktopRuntimeStore): boolean {
  return (
    state.hydration === "live" && state.client !== null && state.connectionStatus === "connected"
  );
}

/** The switch is still the operator's current answer: no later switch, same profile in view. */
function switchStillCurrent(request: SweepRequest): boolean {
  const { switchEpoch, view } = profileViewStore.getSnapshot().context;
  return (
    switchEpoch === request.epoch && view?.kind === "profile" && view.profile === request.profile
  );
}

/**
 * Closes another profile's windows when the operator switches into one profile.
 *
 * Desks belong to one profile, and under All profiles the client works on the destination's desk,
 * so sessions, terminals, pinned agents and automations of every profile opened there are stored
 * in it. A switch to that profile would leave them on screen, resolving under the wrong profile.
 * The switch is the moment they stop belonging: each desk of the profile (every project, All
 * projects, User) is swept the first time it is live after the switch, and each window whose item
 * another profile owns closes through the normal close command, so it stays reopenable and the
 * other profile's own desk is untouched. The close is view-only: the operator's close guard never
 * runs, so another profile's running terminal leaves this desk without being terminated. Only an
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
      scope.view.kind === "aggregate"
        ? null
        : { epoch, profile: scope.view.profile, profileId: targetProfileId ?? "" }
    );
  }
  // The profile list can resolve the target's id after the switch commits.
  if (request !== null && request.profileId === "" && targetProfileId !== null) {
    setRequest({ ...request, profileId: targetProfileId });
  }
  const live = useAtom(shell.projection, selectDeskLive);
  const swept = useRef<SweptPartitions | null>(null);
  const mounted = useRef(false);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  useEffect(() => {
    if (request === null || request.profileId === "" || !live || workspaceId === null) return;
    if (request.profileId !== targetProfileId) return;
    const done = swept.current;
    if (done?.epoch === request.epoch && done.partitions.has(workspaceId)) return;
    const { coordinator, manager } = shell;
    // The render saw the previous binding; the rebind in this commit is what has to be live.
    if (!selectDeskLive(manager.getState())) return;
    if (done?.epoch === request.epoch) done.partitions.add(workspaceId);
    else swept.current = { epoch: request.epoch, partitions: new Set([workspaceId]) };
    const partition = workspaceId;
    void (async () => {
      const foreign = await foreignProfileWindows(
        manager.getState().windows,
        partition,
        { id: request.profileId, name: request.profile },
        profileItemOwnerLookup(queryClient)
      );
      for (const { windowId, item } of foreign) {
        // react-doctor-disable-next-line react-doctor/async-await-in-loop -- one close at a time is the point: the runtime's close guard admits a single close and refuses the rest while it is pending.
        await closeWhileForeign(
          () => coordinator.userClose(windowId, true),
          // A later switch or a rolled-back one owns the desk now; the operator may have moved
          // the window to another item (or to another desk, where its id is gone), so only the
          // inspected one licenses a close; and nothing retries after the shell unmounts.
          () =>
            mounted.current &&
            switchStillCurrent(request) &&
            sameShownItem(shownProfileItem(manager.getState().windows[windowId], partition), item)
        );
      }
    })();
  }, [live, queryClient, request, shell, targetProfileId, workspaceId]);
}
