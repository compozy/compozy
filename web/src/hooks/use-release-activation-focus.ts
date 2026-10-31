import { useEffect } from "react";

/** The native hand-off lands within a few milliseconds of the window's focus event. */
const ACTIVATION_HANDOFF_MS = 100;

/**
 * When the desktop window is shown again (⌘H and back, reopened from the Dock),
 * Chromium may hand initial focus to the page's first tabbable control as if it
 * were reached by keyboard, so it paints a focus ring during plain mouse use.
 * Focus that lands right after the window activates, before any key or pointer
 * input, on a control other than the one focused when the window lost focus is
 * that hand-off: release it back to the document. Restoring the control that
 * held focus before the window deactivated is left alone.
 */
export function useReleaseActivationFocus(): void {
  useEffect(() => {
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
      heldBeforeDeactivation = document.activeElement;
    };
    const onWindowFocus = () => {
      activating = true;
      clearTimeout(activationTimer);
      activationTimer = setTimeout(settle, ACTIVATION_HANDOFF_MS);
    };
    const onFocusIn = (event: FocusEvent) => {
      const target = event.target;
      const handOff =
        activating && target instanceof HTMLElement && target !== heldBeforeDeactivation;
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
