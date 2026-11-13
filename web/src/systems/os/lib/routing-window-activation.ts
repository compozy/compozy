import type { OsDesktopRuntimeStore, OsWindowRoute } from "./os-types";
import { sameOsWindowRoute } from "./window-manager-route";

type ActivationState = Pick<OsDesktopRuntimeStore, "windows" | "focusedId">;

/** What a window activation started from: the intent it carries and the focus it found. */
export interface WindowActivationStart {
  readonly windowId: string;
  readonly intent: number;
  readonly startedOn: string | null;
  /** The requested window was already focused and shown when the activation began. */
  readonly alreadyShown: boolean;
  /** The workspace cycle it began in (`WindowActivations.abandon`). */
  readonly generation: number;
}

/**
 * The window activations and open-or-focus commands in flight. Each writes its own entry once it
 * settles; reports meanwhile defer to them, so a frame that shows the new focus first cannot
 * replace the entry the operator leaves (#1655). An activation a workspace switch abandoned writes nothing: the state it
 * settles into belongs to the workspace switched to, whose one entry the switch writes itself.
 */
export class WindowActivations {
  private inFlight = 0;
  private generation = 0;

  /** Whether an activation's focus is still in flight. */
  get settling(): boolean {
    return this.inFlight > 0;
  }

  begin(state: ActivationState, windowId: string, intent: number): WindowActivationStart {
    this.inFlight += 1;
    const startedOn = state.focusedId;
    const alreadyShown = startedOn === windowId && state.windows[windowId]?.shown === true;
    return { windowId, intent, startedOn, alreadyShown, generation: this.generation };
  }

  /** Ends an activation once its focus settled; returns the entry it writes, if any. */
  end(
    start: WindowActivationStart,
    state: ActivationState,
    currentRoute: OsWindowRoute | null,
    newestIntent: number
  ): OsWindowRoute | null {
    if (!this.finish(start)) return null;
    return windowActivationEntry(start, state, currentRoute, newestIntent);
  }

  /**
   * Ends an open-or-focus once its command settled; returns the entry it writes, if any. One the
   * daemon did not apply writes nothing. `requested` is the route it asked for, written while the
   * projection does not show the window yet; `landed` is the window the open settled on, which a
   * conflict recovery can move to an existing instance.
   */
  endOpen(
    start: WindowActivationStart,
    state: ActivationState,
    currentRoute: OsWindowRoute | null,
    newestIntent: number,
    outcome: { applied: boolean; requested: OsWindowRoute; landed: string }
  ): OsWindowRoute | null {
    if (!this.finish(start) || !outcome.applied) return null;
    return windowOpenEntry(start, state, currentRoute, newestIntent, outcome);
  }

  /** Counts one activation or open out; whether it still belongs to the current workspace cycle. */
  private finish(start: WindowActivationStart): boolean {
    this.inFlight -= 1;
    return start.generation === this.generation;
  }

  /** A workspace switch began: no activation in flight writes into what follows. */
  abandon(): void {
    this.generation += 1;
  }
}

/**
 * The one history entry a settled activation writes (Routing Model rule 5), or `null` for none.
 *
 * Only the newest intent writes, and it names the window focused once its focus settled, whatever
 * the daemon answered: a focus a newer one replaced, or a repeat the daemon answers unapplied
 * because the window is already focused, still ends on its window. Focus that ended elsewhere is
 * written only when it moved (another source moved this client during the activation). An
 * activation that starts and ends on its own window, already shown, with the URL naming it,
 * changed nothing and writes nothing; another window that shares the route still writes.
 */
export function windowActivationEntry(
  start: WindowActivationStart,
  state: ActivationState,
  currentRoute: OsWindowRoute | null,
  newestIntent: number
): OsWindowRoute | null {
  if (start.intent !== newestIntent) return null;
  const focusedId = state.focusedId;
  const focused = focusedId === null ? undefined : state.windows[focusedId];
  if (!focused) return null;
  if (focusedId !== start.windowId) return focusedId !== start.startedOn ? focused.route : null;
  const unchanged = start.alreadyShown && sameOsWindowRoute(currentRoute, focused.route);
  return unchanged ? null : focused.route;
}

/**
 * The one history entry a settled open-or-focus writes (#1655), or `null` for none: the opened
 * window's route (an intent it consumed meanwhile included), else the route it was asked for. Only
 * the newest intent writes; an open of the window that was already focused, shown and named by the
 * URL changed nothing and writes nothing.
 */
export function windowOpenEntry(
  start: WindowActivationStart,
  state: ActivationState,
  currentRoute: OsWindowRoute | null,
  newestIntent: number,
  opened: { requested: OsWindowRoute; landed: string }
): OsWindowRoute | null {
  if (start.intent !== newestIntent) return null;
  const route = state.windows[opened.landed]?.route ?? opened.requested;
  const unchanged =
    start.alreadyShown &&
    opened.landed === start.windowId &&
    state.focusedId === start.windowId &&
    sameOsWindowRoute(currentRoute, route);
  return unchanged ? null : route;
}
