import { createStore } from "@xstate/store";

/** How long an activation's request waits for its session's composer before it lapses. */
export const SESSION_COMPOSER_FOCUS_LIMIT_MS = 3000;

export interface SessionComposerFocusRequest {
  key: number;
  sessionId: string;
  /** Epoch ms after which the request no longer applies. */
  until: number;
}

/**
 * The caret follows an explicit session activation into that session's composer (#1868). The
 * activation path records the request (`requestSessionComposerFocus`, from the routing
 * coordinator); the composer of that session claims it once it can take focus
 * (`claimSessionComposerFocus`). A newer request replaces an older one.
 */
export const sessionComposerFocusStore = createStore({
  context: { key: 0, request: null as SessionComposerFocusRequest | null },
  on: {
    requested: (context, event: { sessionId: string; until: number }) => {
      const key = context.key + 1;
      return { key, request: { key, sessionId: event.sessionId, until: event.until } };
    },
    settled: (context, event: { key: number }) =>
      context.request?.key === event.key ? { ...context, request: null } : context,
  },
});

let passiveDepth = 0;
/** Removes the pointer watch of the request in flight, if any. */
let releasePointerWatch: (() => void) | null = null;

/** Ends the request `key` (landed, lapsed, cancelled); a newer request is left alone. */
function settleRequest(key: number): void {
  if (sessionComposerFocusStore.getSnapshot().context.request?.key !== key) return;
  releasePointerWatch?.();
  sessionComposerFocusStore.trigger.settled({ key });
}

/** Ends whatever request is in flight. */
function settleCurrentRequest(): void {
  const request = sessionComposerFocusStore.getSnapshot().context.request;
  if (request !== null) settleRequest(request.key);
}

/**
 * A pointer press after the activation is the operator placing focus: the request ends, whether
 * or not its composer has rendered yet. The press that caused the activation came before it.
 */
function watchPointer(key: number): void {
  releasePointerWatch?.();
  if (typeof document === "undefined") return;
  const cancel = () => settleRequest(key);
  document.addEventListener("pointerdown", cancel, true);
  releasePointerWatch = () => {
    document.removeEventListener("pointerdown", cancel, true);
    releasePointerWatch = null;
  };
}

/** Records an explicit activation of `sessionId`; a no-op inside `withoutSessionComposerFocus`. */
export function requestSessionComposerFocus(sessionId: string, now = Date.now()): void {
  const id = sessionId.trim();
  if (id === "" || passiveDepth > 0) return;
  sessionComposerFocusStore.trigger.requested({
    sessionId: id,
    until: now + SESSION_COMPOSER_FOCUS_LIMIT_MS,
  });
  const request = sessionComposerFocusStore.getSnapshot().context.request;
  if (request !== null) watchPointer(request.key);
}

/**
 * Runs a passive activation: the session list's keyboard walk (#1832) and the board drawer's
 * step activate sessions the way a click does, but the keyboard stays where the walk keeps it.
 * Covers the synchronous part of `run`, which is where the activation path records requests. A
 * request still in flight ends too, so walking back to its session never lands it later.
 */
export function withoutSessionComposerFocus<T>(run: () => T): T {
  settleCurrentRequest();
  passiveDepth += 1;
  try {
    return run();
  } finally {
    passiveDepth -= 1;
  }
}

/** Overlays and regions whose focus the composer never takes. */
const FOCUS_OWNER_SELECTOR = [
  '[role="dialog"]',
  '[role="alertdialog"]',
  '[aria-modal="true"]',
  '[role="menu"]',
  '[role="listbox"]',
].join(", ");
const OS_WINDOW_SELECTOR = '[data-slot="os-window-surface"]';
const COMPOSER_SURFACE_SELECTOR = '[data-testid="composer-shell"]';
const NON_TEXT_INPUTS = new Set(["button", "checkbox", "radio", "reset", "submit"]);

function isEditable(element: Element): boolean {
  if (element instanceof HTMLTextAreaElement || element instanceof HTMLSelectElement) return true;
  if (element instanceof HTMLInputElement) return !NON_TEXT_INPUTS.has(element.type);
  return element instanceof HTMLElement && element.isContentEditable;
}

/**
 * Whether the composer `editable` may take the caret now: its window is the focused one (or it is
 * outside the OS shell), and nothing else owns focus. An editable field (a find bar, a terminal,
 * another composer), an open overlay, and this composer's own surface (its decision dock,
 * permission prompt, queue editing and controls) all keep it.
 */
export function composerFocusAllowed(editable: HTMLElement): boolean {
  const frame = editable.closest(OS_WINDOW_SELECTOR);
  if (frame !== null && !frame.hasAttribute("data-focused")) return false;
  const active = editable.ownerDocument.activeElement;
  if (active === null || active === editable.ownerDocument.body) return true;
  if (active.closest(FOCUS_OWNER_SELECTOR) !== null) return false;
  if (isEditable(active)) return false;
  return active.closest(COMPOSER_SURFACE_SELECTOR) !== editable.closest(COMPOSER_SURFACE_SELECTOR);
}

/**
 * Lands the pending request for `sessionId` on `editable` when it applies now. Returns whether the
 * request is finished (landed, lapsed, replaced, or already in place); `false` keeps it waiting.
 * A composer not rendered yet (`null`) keeps it waiting until it lapses.
 */
export function claimSessionComposerFocus(
  sessionId: string,
  editable: HTMLElement | null,
  now = Date.now()
): boolean {
  const request = sessionComposerFocusStore.getSnapshot().context.request;
  if (request === null || request.sessionId !== sessionId) return true;
  const settle = () => {
    settleRequest(request.key);
    return true;
  };
  if (now > request.until) return settle();
  if (editable === null) return false;
  if (editable.contains(editable.ownerDocument.activeElement)) return settle();
  if (!composerFocusAllowed(editable)) return false;
  editable.focus({ preventScroll: true });
  return editable.ownerDocument.activeElement === editable ? settle() : false;
}
