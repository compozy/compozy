import { runViewTransition } from "@compozy/ui";

import { useOsReducedMotion } from "../../hooks/use-os-reduced-motion";
import { useOsShell } from "../../hooks/use-os-shell";
import type { OsWindowRoute } from "../../lib/os-types";

/** Keeps section transitions and the shell's navigation owner together. */
export function useSettingsNavigation(shouldAnimate: (next: OsWindowRoute) => boolean) {
  const { coordinator } = useOsShell();
  const reducedMotion = useOsReducedMotion();

  return (next: OsWindowRoute) => {
    if (!shouldAnimate(next)) {
      coordinator.userNavigate(next);
      return;
    }
    void runViewTransition(() => coordinator.userNavigate(next), { reduced: reducedMotion });
  };
}
