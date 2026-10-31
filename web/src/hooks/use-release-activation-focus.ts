import { useEffect } from "react";

/** The native hand-off lands within a few milliseconds of the window's focus event. */
const ACTIVATION_HANDOFF_MS = 100;

const TABBABLE =
  'a[href], button, input, select, textarea, summary, [contenteditable="true"], [tabindex]';

/** The first control in the document's sequential focus order — where Chromium's hand-off lands. */
function isFirstTabbable(target: HTMLElement): boolean {
  for (const element of document.querySelectorAll<HTMLElement>(TABBABLE)) {
    if (element.tabIndex < 0 || element.matches(":disabled") || element.closest("[inert]")) {
      continue;
    }
    if (typeof element.checkVisibility === "function" && !element.checkVisibility()) continue;
    return element === target;
  }
  return false;
}

/**
 * When the desktop window is shown again (⌘H and back, reopened from the Dock),
 * Chromium may hand initial focus to the page's first tabbable control as if it
 * were reached by keyboard, so it paints a focus ring during plain mouse use.
 * That hand-off is released back to the document only when every signal agrees:
 * the window is regaining focus it had lost, the focus lands within moments of
 * that, no key or pointer input came first, the target is the document's first
 * tabbable control, and it is not the control that held focus before the
 * window deactivated. Focus the app sets on mount (an autofocused input, a
 * dialog title) or that assistive technology moves elsewhere is left alone.
 */
export function useReleaseActivationFocus(): void {
  useEffect(() => {
    let deactivated = false;
    let activating = false;
    let activationTimer: ReturnType<typeof setTimeout> | undefined;
    let heldBeforeDeactivation: Element | null = null;
    const settle = () => {
      activating = false;
      clearTimeout(activationTimer);
    };
    // Focus events do not bubble, so these bubble-phase window listeners hear
    // only the window itself losing and regaining focus.
    const onWindowBlur = () => {
      settle();
      deactivated = true;
      heldBeforeDeactivation = document.activeElement;
    };
    const onWindowFocus = () => {
      if (!deactivated) return;
      activating = true;
      clearTimeout(activationTimer);
      activationTimer = setTimeout(settle, ACTIVATION_HANDOFF_MS);
    };
    const onFocusIn = (event: FocusEvent) => {
      const target = event.target;
      const handOff =
        activating &&
        target instanceof HTMLElement &&
        target !== heldBeforeDeactivation &&
        isFirstTabbable(target);
      settle();
      if (!handOff) return;
      // Settle the focus change first; blurring mid-dispatch can be dropped.
      queueMicrotask(() => {
        if (document.activeElement === target) target.blur();
      });
    };
    window.addEventListener("blur", onWindowBlur);
    window.addEventListener("focus", onWindowFocus);
    document.addEventListener("keydown", settle, true);
    document.addEventListener("pointerdown", settle, true);
    document.addEventListener("focusin", onFocusIn, true);
    return () => {
      settle();
      window.removeEventListener("blur", onWindowBlur);
      window.removeEventListener("focus", onWindowFocus);
      document.removeEventListener("keydown", settle, true);
      document.removeEventListener("pointerdown", settle, true);
      document.removeEventListener("focusin", onFocusIn, true);
    };
  }, []);
}
