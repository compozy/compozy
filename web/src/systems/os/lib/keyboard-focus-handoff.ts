/** How long a handoff waits for its window to render before it lapses. */
export const KEYBOARD_HANDOFF_LIMIT_MS = 1000;

/** Takes the keyboard into `windowId` when it can now; `false` keeps the handoff waiting. */
export type KeyboardFocusTarget = (windowId: string) => boolean;

/**
 * Carries the keyboard into the window a focus change of this client's own asked for (#1655): a
 * directional focus, a tab or window activation from a chord, the palette, the dock or a menu, an
 * open-or-focus, Back/Forward. The window manager's focus moves first; the handoff then waits for
 * the window's pane to render (a tab switch shows it a frame later) and the pane hosts move DOM
 * focus into it (`WindowPaneHosts`).
 *
 * Only this client's own actions request one. A focus another client or the CLI sends reaches this
 * client as a client view and requests nothing, so the caret stays where the operator types. A
 * pointer activation requests nothing either: the click already placed focus, and a pointer press
 * anywhere drops a handoff still waiting (`cancel`). A handoff that cannot land within
 * `KEYBOARD_HANDOFF_LIMIT_MS` lapses, so a later foreign focus of that window never takes it.
 */
export class KeyboardFocusHandoff {
  private pending: { windowId: string; until: number } | null = null;
  private readonly targets = new Set<KeyboardFocusTarget>();
  /** Pointer presses so far (`cancel`): an action compares it with what it saw when it began. */
  private presses = 0;

  /**
   * Marks the start of an own focus change that settles later; pass the mark to `request`. A
   * pointer press in between (a click into a field while the daemon answers) wins: the handoff
   * the action asks for then does nothing.
   */
  begin(): number {
    return this.presses;
  }

  /**
   * Asks for the keyboard to follow focus into `windowId`; a newer request replaces it. `since`
   * is the action's `begin` mark.
   */
  request(windowId: string, since: number): void {
    if (since !== this.presses) return;
    this.pending = { windowId, until: Date.now() + KEYBOARD_HANDOFF_LIMIT_MS };
    this.offer();
  }

  /**
   * Offers the waiting handoff to the targets, or only when it names `windowId` if given; the
   * first that takes it ends it.
   */
  offer(windowId?: string): void {
    const pending = this.pending;
    if (pending === null || (windowId !== undefined && windowId !== pending.windowId)) return;
    // Cleared first: the focus a target moves reports back here (`focusin`) before it returns.
    this.pending = null;
    if (Date.now() > pending.until) return;
    for (const target of this.targets) {
      if (target(pending.windowId)) return;
    }
    if (this.pending === null) this.pending = pending;
  }

  /**
   * The operator placed focus themselves with a pointer: a handoff still waiting no longer
   * applies, nor does one an action in flight would ask for.
   */
  cancel(): void {
    this.presses += 1;
    this.pending = null;
  }

  /** Adds a target the handoffs are offered to; returns its removal. */
  register(target: KeyboardFocusTarget): () => void {
    this.targets.add(target);
    return () => {
      this.targets.delete(target);
    };
  }
}
