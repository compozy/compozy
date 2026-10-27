import { useEffect } from "react";

import { useFontAppearance } from "../hooks/use-font-appearance";
import {
  applyFontAppearance,
  loadBundledFonts,
  writeFontAppearanceCache,
} from "../lib/font-appearance";

/**
 * Paints the daemon's Appearance fonts on <html> for every surface of this
 * client. Renders nothing; it waits for the daemon's answer before touching
 * the variables, so the boot script's cached values stand until then. A change
 * made on another client lands through the shell section's refetch.
 */
export function FontAppearanceSync() {
  const { appearance, loaded } = useFontAppearance();
  const { sans, mono } = appearance;

  useEffect(() => {
    if (!loaded) return;
    const next = { sans, mono };
    applyFontAppearance(document.documentElement, next);
    writeFontAppearanceCache(next);
    void loadBundledFonts(next);
  }, [loaded, sans, mono]);

  return null;
}
