import { useEffect } from "react";

import { useFontAppearance } from "../hooks/use-font-appearance";
import {
  DEFAULT_FONT_APPEARANCE,
  applyFontAppearance,
  bootCachedBundledFonts,
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

  // The boot script may already name a bundled face; fetch its stylesheet now.
  useEffect(() => {
    const cached = bootCachedBundledFonts(document.documentElement);
    if (cached.length > 0) void loadBundledFonts(DEFAULT_FONT_APPEARANCE, cached);
  }, []);

  useEffect(() => {
    if (!loaded) return;
    const next = { sans, mono };
    let current = true;
    // A bundled face only gets its @font-face once its lazy stylesheet lands;
    // publishing the variables first would let open terminals measure the
    // fallback and never see a later root change.
    void loadBundledFonts(next).then(() => {
      if (!current) return;
      applyFontAppearance(document.documentElement, next);
      writeFontAppearanceCache(next);
    });
    return () => {
      current = false;
    };
  }, [loaded, sans, mono]);

  return null;
}
