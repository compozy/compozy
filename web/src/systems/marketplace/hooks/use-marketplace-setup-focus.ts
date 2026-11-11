import { useEffect, type RefObject } from "react";

const SETUP_TARGETS = ['[data-setup-target="inputs"]', '[data-setup-target="authorize"]'];

/**
 * Moves focus to the first setup control once it renders. The controls arrive with the live
 * server read, so this watches the detail subtree until one exists, then stops.
 */
export function useMarketplaceSetupFocus(
  container: RefObject<HTMLDivElement | null>,
  enabled: boolean
) {
  useEffect(() => {
    const root = container.current;
    if (!enabled || !root) return;
    const focusTarget = () => {
      for (const selector of SETUP_TARGETS) {
        const target = root.querySelector<HTMLElement>(selector);
        if (target) {
          target.focus();
          return true;
        }
      }
      return false;
    };
    if (focusTarget()) return;
    const observer = new MutationObserver(() => {
      if (focusTarget()) observer.disconnect();
    });
    observer.observe(root, { childList: true, subtree: true, attributes: true });
    return () => observer.disconnect();
  }, [container, enabled]);
}
