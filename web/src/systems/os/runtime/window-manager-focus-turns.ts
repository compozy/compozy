import type { WindowManagerCommandInput } from "../lib/window-manager-types";

/** One `window.focus` this client queued; the dispatch chain carries it to `takeoff` and `settle`. */
export interface FocusTicket {
  /** The absolute focuses dispatched up to and including this one. */
  readonly seq: number;
  /** Dispatched with `replaceable`: a newer focus may drop it or resolve it as replaced. */
  readonly replaceable: boolean;
  /** The binding and registration it was dispatched under (see `reset`). */
  readonly epoch: number;
  /** A newer absolute focus replaced this one before it was sent. */
  superseded: boolean;
}

interface PresentationView {
  presentationRevision: number;
  /** Present on a registration's view: it starts the revisions its later views count from. */
  attachmentToken?: string;
}

/**
 * How long a client view may wait behind this client's focuses. Once it has waited this long it
 * is adopted, whether or not a command settles then, so neither a stream of clicks faster than one
 * round trip nor one slow focus keeps another source's view off screen.
 */
export const FOCUS_HOLD_LIMIT_MS = 300;

/** A focus on a named window; a directional focus depends on the focus before it. */
function isAbsoluteFocus(command: WindowManagerCommandInput): boolean {
  return command.commandId === "window.focus" && command.payload.direction === "";
}

/**
 * Collapses this client's `window.focus` presentation commands to the newest intent.
 *
 * Focus is presentation: only the newest one the operator asked for matters. A `replaceable` focus
 * still waiting at the tail of the dispatch queue is not sent once a newer absolute focus queues
 * behind it, and the answer of a replaceable focus that a newer one has replaced does not move the
 * shown tab; its completion resolves `false` without counting as refused (see `replaced`). A focus
 * dispatched without `replaceable` (an open or a landing that acts on its result) always runs and
 * keeps its result. A replaced focus's client view is held, as is any client view the stream
 * delivers while a focus still waits for its answer (the stream reports each focus the daemon
 * applies, the replaced ones included). The newest held view is adopted once no focus is
 * outstanding, or once it has waited `FOCUS_HOLD_LIMIT_MS`, where an older presentation revision
 * is ignored; each held view that is adopted is reported to `onHeldViewAdopted` listeners. Any
 * other command between two focuses keeps the earlier one: layout commands are never dropped,
 * reordered or kept from observing the focus they were queued after.
 *
 * What is held belongs to one binding and one registration: `reset` ends both, and the view a
 * focus dispatched before it answers is dropped, never held or shown; its completion still
 * reports what the daemon answered.
 */
export class FocusTurns<View extends PresentationView> {
  private absolute = 0;
  private epoch = 0;
  /** Commands admitted and not yet settled, of every kind. */
  private running = 0;
  private queuedTail: FocusTicket | null = null;
  private readonly outstanding = new Set<FocusTicket>();
  private heldView: View | null = null;
  private holdTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly adoptedListeners = new Set<() => void>();

  /** `adopt` shows a released view and says whether it was adopted (not ignored as older). */
  constructor(private readonly adopt: (view: View) => boolean) {}

  /** Admits a dispatched command; returns its ticket when it is a focus. */
  admit(command: WindowManagerCommandInput): FocusTicket | null {
    // A command that enters an idle queue is sent at once; only one waiting behind another can
    // still be replaced.
    const waits = this.running > 0;
    this.running += 1;
    if (command.commandId !== "window.focus") {
      // Nothing queued before another command may be dropped any more.
      this.queuedTail = null;
      return null;
    }
    if (isAbsoluteFocus(command)) {
      this.absolute += 1;
      if (this.queuedTail !== null) this.queuedTail.superseded = true;
    }
    const replaceable = command.replaceable === true;
    const ticket: FocusTicket = {
      seq: this.absolute,
      replaceable,
      epoch: this.epoch,
      superseded: false,
    };
    this.outstanding.add(ticket);
    this.queuedTail = waits && replaceable ? ticket : null;
    return ticket;
  }

  /**
   * Whether the command whose turn came is still sent; a sent focus can no longer be replaced. A
   * focus queued before the last `reset` belonged to what ended and is not sent.
   */
  takeoff(ticket: FocusTicket | null): boolean {
    if (ticket === null) return true;
    if (this.queuedTail === ticket) this.queuedTail = null;
    return !ticket.superseded && ticket.epoch === this.epoch;
  }

  /**
   * Whether a newer absolute focus replaced this replaceable focus. A `reset` replaces nothing: a
   * focus the daemon answered still reports that answer, whichever binding is current by then.
   */
  replaced(ticket: FocusTicket | null): boolean {
    if (ticket === null) return false;
    return ticket.replaceable && this.absolute > ticket.seq;
  }

  /**
   * Keeps a focus answer's view off screen when it was dispatched before the last `reset`
   * (dropped) or a newer focus replaced it (held).
   */
  holdAnswer(ticket: FocusTicket | null, view: View): boolean {
    if (ticket === null) return false;
    if (ticket.epoch !== this.epoch) return true;
    if (!this.replaced(ticket)) return false;
    this.keep(view);
    return true;
  }

  /**
   * Whether a client view arriving from the stream waits behind a focus of this client. A view
   * that ends the registration (`null`) or starts another (a new attachment token) is never held,
   * and ends what was held for the last one.
   */
  offer(view: View | null, attachmentToken: string | null): boolean {
    if (view === null || (view.attachmentToken ?? attachmentToken) !== attachmentToken) {
      this.reset();
      return false;
    }
    if (view.attachmentToken !== undefined) return false;
    if (this.outstanding.size === 0) return false;
    this.keep(view);
    return true;
  }

  /** Ends a dispatched command's turn and adopts the newest held view once nothing holds it. */
  settle(ticket: FocusTicket | null): void {
    this.running -= 1;
    if (ticket !== null) this.outstanding.delete(ticket);
    if (this.outstanding.size === 0) this.release();
  }

  /** Calls `listener` after each held client view is adopted; returns the unsubscribe. */
  onHeldViewAdopted(listener: () => void): () => void {
    this.adoptedListeners.add(listener);
    return () => {
      this.adoptedListeners.delete(listener);
    };
  }

  /**
   * Ends the binding or registration the queued and held focus state belongs to: a focus queued
   * before it is not sent, the view a focus dispatched before it answers is dropped, and nothing
   * held for them is shown.
   */
  reset(): void {
    this.epoch += 1;
    this.queuedTail = null;
    this.outstanding.clear();
    this.heldView = null;
    this.stopHoldTimer();
  }

  private keep(view: View): void {
    if (this.heldView === null) {
      this.holdTimer = setTimeout(() => this.release(), FOCUS_HOLD_LIMIT_MS);
    }
    if (this.heldView === null || view.presentationRevision >= this.heldView.presentationRevision) {
      this.heldView = view;
    }
  }

  private release(): void {
    this.stopHoldTimer();
    const view = this.heldView;
    if (view === null) return;
    this.heldView = null;
    if (!this.adopt(view)) return;
    for (const listener of this.adoptedListeners) listener();
  }

  private stopHoldTimer(): void {
    if (this.holdTimer === null) return;
    clearTimeout(this.holdTimer);
    this.holdTimer = null;
  }
}
