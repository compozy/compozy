import { useEffect } from "react";

/** Clicks controls in order on mount, for the review states that show a menu or submenu open. */
export function useOpenOnMount(selector: string | readonly string[] | undefined) {
  const key = Array.isArray(selector) ? selector.join("|") : selector;
  useEffect(() => {
    if (!key) return;
    const timers = key.split("|").map((each, index) =>
      window.setTimeout(() => {
        const target = document.querySelector<HTMLElement>(each);
        if (index === 0) target?.click();
        // A submenu opens on pointer entry, like a person hovering it.
        else target?.dispatchEvent(new PointerEvent("pointermove", { bubbles: true }));
        target?.focus();
        if (index > 0)
          target?.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
      }, index * 350)
    );
    return () => timers.forEach(timer => window.clearTimeout(timer));
  }, [key]);
}
