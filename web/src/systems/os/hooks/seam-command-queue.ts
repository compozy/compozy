import type { OsDesktopRuntime } from "../lib/os-types";
import type { PixelRect } from "../lib/window-manager-types";
import { WINDOW_MANAGER_COMMAND_DEADLINE_MS } from "../runtime/window-manager-runtime-helpers";
import type { WindowManagerBinding } from "../stores/window-manager-store-types";
import { windowManagerStore } from "../stores/window-manager-store";

/**
 * What a seam input is drawn against: the client binding, the shown desktop and tab, and the
 * work area. Hidden tabs stay mounted, so a tab or desktop switch must end a seam input explicitly.
 */
export function formatSeamFrame(
  binding: WindowManagerBinding | null,
  shown: { activeDesktopId: string | null; activeTabIds: Readonly<Record<string, string>> },
  rect: PixelRect | undefined
): string {
  const bound =
    binding === null ? "" : `${binding.workspaceId}/${binding.profileId}/${binding.clientId}`;
  const desktopId = shown.activeDesktopId ?? "";
  const area = rect === undefined ? "" : `${rect.x},${rect.y},${rect.w},${rect.h}`;
  return `${bound}|${desktopId}/${shown.activeTabIds[desktopId] ?? ""}|${area}`;
}

/** The frame a seam input made now would carry. */
export function currentSeamFrame(manager: OsDesktopRuntime): string {
  const { binding, workArea } = windowManagerStore.getSnapshot().context;
  return formatSeamFrame(binding, manager.getState(), workArea?.rect);
}

/**
 * Where and when a seam input was made: the client binding and the shown frame it was made in,
 * the time of the input, and whether the seam that took it is still mounted. A queued command
 * runs only while all of it still holds.
 */
export interface SeamInputScope {
  readonly frame: string;
  readonly at: number;
  readonly mounted: () => boolean;
}

/** A seam command waiting behind this client's commands in flight on its split. */
export interface QueuedSeamCommand {
  readonly scope: SeamInputScope;
  /** Authors and sends the command against the layout as it is now. */
  readonly run: () => void;
  /** Gives up without sending: the input's scope or age no longer holds. */
  readonly drop?: () => void;
}

/**
 * This client's own resizes and balances in flight on one split, and the seam commands queued
 * behind them. Kept per manager (one per client) and split, so a sibling seam's command counts
 * too, and only while something is in flight or queued: the map never outgrows the splits this
 * client is resizing right now. The runtime publishes a command's snapshot before its completion
 * settles, so its result always lands while it is still counted.
 */
interface OwnSplitCommands {
  pending: number;
  queued: QueuedSeamCommand[];
}

const ownCommands = new WeakMap<object, Map<string, OwnSplitCommands>>();

export function ownPending(manager: object, splitId: string): number {
  return ownCommands.get(manager)?.get(splitId)?.pending ?? 0;
}

/** Whether a command of this client is in flight on the split, or queued behind one. */
export function ownBusy(manager: object, splitId: string): boolean {
  const own = ownCommands.get(manager)?.get(splitId);
  return own !== undefined && (own.pending > 0 || own.queued.length > 0);
}

/**
 * Whether an input made in `scope` may still be sent from `frame`: same binding and shown frame,
 * its seam still mounted, and younger than the runtime's command deadline, counted from the input.
 */
export function seamInputHolds(scope: SeamInputScope, frame: string, now: number): boolean {
  return (
    scope.frame === frame && scope.mounted() && now - scope.at <= WINDOW_MANAGER_COMMAND_DEADLINE_MS
  );
}

/** Runs one queued command, or drops it when its input no longer holds; never throws. */
function resume(manager: OsDesktopRuntime, command: QueuedSeamCommand): void {
  try {
    if (seamInputHolds(command.scope, currentSeamFrame(manager), Date.now())) command.run();
    else command.drop?.();
  } catch (error) {
    // One failing command must not leave the split busy behind it forever.
    console.error("A queued seam command failed.", error);
  }
}

/**
 * Counts one of this client's commands on a split until it settles, then resumes what is queued
 * behind it strictly one at a time: a resumed command that sends puts the rest behind its own
 * result, so each one is authored against the layout the one before it leaves.
 */
export function trackOwnCommand(
  manager: OsDesktopRuntime,
  splitId: string,
  completion: Promise<boolean>
): Promise<boolean> {
  let splits = ownCommands.get(manager);
  if (splits === undefined) {
    splits = new Map();
    ownCommands.set(manager, splits);
  }
  let own = splits.get(splitId);
  if (own === undefined) {
    own = { pending: 0, queued: [] };
    splits.set(splitId, own);
  }
  const tracked = own;
  const owner = splits;
  tracked.pending += 1;
  return completion.finally(() => {
    tracked.pending -= 1;
    while (tracked.pending === 0) {
      const next = tracked.queued.shift();
      if (next === undefined) break;
      resume(manager, next);
    }
    if (tracked.pending === 0 && owner.get(splitId) === tracked) owner.delete(splitId);
  });
}

/** Runs `command` now when nothing of this client is in flight or queued on the split, else queues it. */
export function afterOwnCommands(
  manager: OsDesktopRuntime,
  splitId: string,
  command: QueuedSeamCommand
): void {
  const own = ownCommands.get(manager)?.get(splitId);
  if (own === undefined || (own.pending === 0 && own.queued.length === 0)) resume(manager, command);
  else own.queued.push(command);
}
