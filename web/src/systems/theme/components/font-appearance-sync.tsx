import { useEffect, useRef } from "react";

import { useFontAppearance } from "../hooks/use-font-appearance";
import {
  DEFAULT_FONT_APPEARANCE,
  applyFontAppearance,
  applyFontFamily,
  loadBundledFonts,
  readBundledFontCache,
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
  const daemonPublished = useRef(false);

  // The boot script leaves a cached bundled face unpublished (its stylesheet is
  // lazy). Fetch it now and publish it once resident, unless the daemon's
  // answer got there first — that value is authoritative.
  useEffect(() => {
    const cached = readBundledFontCache();
    const roles = (["sans", "mono"] as const).filter(role => cached[role] !== undefined);
    if (roles.length === 0) return;
    void loadBundledFonts({ ...DEFAULT_FONT_APPEARANCE, ...cached }).then(() => {
      if (daemonPublished.current) return;
      for (const role of roles) applyFontFamily(document.documentElement, role, cached[role] ?? "");
    });
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
      daemonPublished.current = true;
      applyFontAppearance(document.documentElement, next);
      writeFontAppearanceCache(next);
    });
    return () => {
      current = false;
    };
  }, [loaded, sans, mono]);

  return null;
}
