import { useEffect, type RefObject } from "react";

/**
 * When the desktop window is shown again (⌘H and back, reopened from the Dock),
 * Chromium hands the page's first tabbable element initial focus as if it were
 * reached by keyboard — that is the skip link, which then pops up during plain
 * mouse use. Focus that lands on the link as the first move after the window
 * activates, before any key or pointer input, is that native hand-off: release
 * it back to the document. A link the user had focused before the window lost
 * focus is a restoration and stays.
 */
export function useReleaseActivationFocus(linkRef: RefObject<HTMLAnchorElement | null>) {
  useEffect(() => {
    const link = linkRef.current;
    if (!link) return;
    let activating = false;
    let heldBeforeDeactivation = false;
    // Focus events do not bubble, so these bubble-phase window listeners hear
    // only the window itself losing and regaining focus.
    const onWindowBlur = () => {
      heldBeforeDeactivation = document.activeElement === link;
    };
    const onWindowFocus = () => {
      activating = true;
    };
    const onInput = () => {
      activating = false;
    };
    const onFocusIn = (event: FocusEvent) => {
      const handOff = activating && event.target === link && !heldBeforeDeactivation;
      activating = false;
      if (!handOff) return;
      // Settle the focus change first; blurring mid-dispatch can be dropped.
      queueMicrotask(() => {
        if (document.activeElement === link) link.blur();
      });
    };
    window.addEventListener("blur", onWindowBlur);
    window.addEventListener("focus", onWindowFocus);
    document.addEventListener("keydown", onInput, true);
    document.addEventListener("pointerdown", onInput, true);
    document.addEventListener("focusin", onFocusIn, true);
    return () => {
      window.removeEventListener("blur", onWindowBlur);
      window.removeEventListener("focus", onWindowFocus);
      document.removeEventListener("keydown", onInput, true);
      document.removeEventListener("pointerdown", onInput, true);
      document.removeEventListener("focusin", onFocusIn, true);
    };
  }, [linkRef]);
}
