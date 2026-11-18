import { useEffect } from "react";

import { useTypeAppearance } from "../hooks/use-type-appearance";
import { applyTypeAppearance, writeTypeAppearanceCache } from "../lib/type-appearance";

/**
 * Paints the daemon's Appearance text size and letter spacing on <html> for
 * every surface of this client. Renders nothing; it waits for the daemon's
 * answer, so the boot script's cached values stand until then. A change made
 * on another client lands through the shell section's refetch.
 */
export function TypeAppearanceSync() {
  const { type, loaded } = useTypeAppearance();
  const { size, spacing } = type;

  useEffect(() => {
    if (!loaded) return;
    const next = { size, spacing };
    applyTypeAppearance(document.documentElement, next);
    writeTypeAppearanceCache(next);
  }, [loaded, size, spacing]);

  return null;
}
