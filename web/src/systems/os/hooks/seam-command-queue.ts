import type { WindowManagerController } from "../lib/os-types";
import type { PixelRect } from "../lib/window-manager-types";
import { commandClockNow, commandTimeLeft } from "../runtime/window-manager-runtime-helpers";
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

/**
 * What the seam queue reads of the runtime: the shown frame, and this client's own commands in
 * flight that can move the layout, seam command or not.
 */
export type SeamCommandRuntime = Pick<
  WindowManagerController,
  "getState" | "commandsInFlight" | "commandsSettled"
>;

/** The frame a seam input made now would carry. */
export function currentSeamFrame(manager: SeamCommandRuntime): string {
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

/**
 * An arrow-key step on one seam, in weight space. The command that carries it reads `weight` when
 * it runs, so a later step merged into it while it waits is sent with it.
 */
export interface SeamKeyStep {
  readonly seamId: string;
  weight: number;
}

/** A seam command waiting behind this client's seam commands in flight. */
export interface QueuedSeamCommand {
  readonly scope: SeamInputScope;
  /** Authors and sends the command against the layout as it is now. */
  readonly run: () => void;
  /** Gives up without sending: the input's scope or age no longer holds. */
  readonly drop?: () => void;
  /** Set on an arrow-key step: queued steps on one seam merge into one command. */
  readonly step?: SeamKeyStep;
}

/**
 * This client's own seam resizes and balances in flight, and the seam commands queued behind
 * them. Kept per manager (one per client), not per split: a seam's geometry also depends on every
 * split around it, so a command on an ancestor, a descendant or a sibling moves the layout the
 * next one is authored against. Only the shown tab's seams take input, and a frame change drops
 * what waits. The runtime publishes a command's snapshot before its completion settles, so its
 * result always lands while it is still counted.
 */
interface OwnCommands {
  pending: number;
  queued: QueuedSeamCommand[];
}

const ownCommands = new WeakMap<object, OwnCommands>();

function ownCommandsOf(manager: object): OwnCommands {
  let own = ownCommands.get(manager);
  if (own === undefined) {
    own = { pending: 0, queued: [] };
    ownCommands.set(manager, own);
  }
  return own;
}

export function ownPending(manager: object): number {
  return ownCommands.get(manager)?.pending ?? 0;
}

/** Whether a seam command of this client is in flight, or queued behind one. */
export function ownBusy(manager: object): boolean {
  const own = ownCommands.get(manager);
  return own !== undefined && (own.pending > 0 || own.queued.length > 0);
}

/**
 * Whether an input made in `scope` may still be sent from `frame`: same binding and shown frame,
 * its seam still mounted, and time left of the runtime's command deadline, counted from the input.
 */
export function seamInputHolds(scope: SeamInputScope, frame: string, now: number): boolean {
  return scope.frame === frame && scope.mounted() && commandTimeLeft(scope.at, now) > 0;
}

/** Runs one queued command, or drops it when its input no longer holds; never throws. */
function resume(manager: SeamCommandRuntime, command: QueuedSeamCommand): void {
  try {
    if (seamInputHolds(command.scope, currentSeamFrame(manager), commandClockNow())) command.run();
    else command.drop?.();
  } catch (error) {
    // One failing command must not leave the client's seams busy behind it forever.
    console.error("A queued seam command failed.", error);
  }
}

/**
 * Counts one of this client's seam commands until it settles, then resumes what is queued behind
 * it strictly one at a time: a resumed command that sends puts the rest behind its own result, so
 * each one is authored against the layout the one before it leaves.
 */
export function trackOwnCommand(
  manager: SeamCommandRuntime,
  completion: Promise<boolean>
): Promise<boolean> {
  const own = ownCommandsOf(manager);
  own.pending += 1;
  return completion.finally(() => {
    own.pending -= 1;
    resumeQueued(manager, own);
  });
}

/**
 * Resumes queued seam commands one at a time while none of this client's is in flight. While an
 * own non-seam command (a palette balance, a move) is in flight, its result may move the layout
 * the next one would be authored against: the queue waits for it, counted as an own command.
 */
function resumeQueued(manager: SeamCommandRuntime, own: OwnCommands): void {
  while (own.pending === 0 && own.queued.length > 0) {
    if (manager.commandsInFlight()) {
      void trackOwnCommand(
        manager,
        manager.commandsSettled().then(() => true)
      );
      return;
    }
    const next = own.queued.shift();
    if (next !== undefined) resume(manager, next);
  }
}

/**
 * Runs `command` now when no command of this client that can move the layout is in flight and no
 * seam command is queued, else queues it.
 * An arrow-key step queued right behind a waiting step on the same seam is added to that step
 * instead, so key repeat against a slow daemon sends one command for what is held, not one per
 * keystroke that keeps resizing after the key is released.
 */
export function afterOwnCommands(manager: SeamCommandRuntime, command: QueuedSeamCommand): void {
  if (!ownBusy(manager) && !manager.commandsInFlight()) {
    resume(manager, command);
    return;
  }
  const own = ownCommandsOf(manager);
  if (!ownBusy(manager)) {
    // Only an own non-seam command is in flight: the command waits for it in the queue.
    own.queued.push(command);
    resumeQueued(manager, own);
    return;
  }
  const queued = own.queued;
  // Only into the newest queued command, so every command stays in input order. The merged step
  // keeps the earlier input's scope and time, so it still expires with that input.
  const lastStep = queued.at(-1)?.step;
  const lastFrame = queued.at(-1)?.scope.frame;
  const step = command.step;
  if (
    step !== undefined &&
    lastStep !== undefined &&
    lastStep.seamId === step.seamId &&
    lastFrame === command.scope.frame
  ) {
    lastStep.weight += step.weight;
    return;
  }
  queued.push(command);
}
