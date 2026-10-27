/**
 * Fonts installed on this device, through the Local Font Access API
 * (`window.queryLocalFonts`). Chromium-only: the desktop app grants the
 * `local-fonts` permission to the product origin; browsers without the API, or
 * a denied prompt, leave only the curated catalog. Queried once per page, on
 * the picker's first open — the call needs a user gesture.
 */

export interface InstalledFonts {
  families: readonly string[];
  /** Families whose glyph advances match, i.e. usable as a Code font. */
  monospace: ReadonlySet<string>;
}

interface LocalFontData {
  family: string;
}

type QueryLocalFonts = () => Promise<LocalFontData[]>;

let pending: Promise<InstalledFonts | null> | null = null;

export function supportsInstalledFonts(): boolean {
  return typeof window !== "undefined" && typeof readQueryLocalFonts() === "function";
}

/** Resolves `null` when the API is missing or the permission was refused. */
export function queryInstalledFonts(): Promise<InstalledFonts | null> {
  pending ??= readInstalledFonts().then(fonts => {
    // A refusal is not cached: the operator may grant it on a later open.
    if (fonts === null) pending = null;
    return fonts;
  });
  return pending;
}

async function readInstalledFonts(): Promise<InstalledFonts | null> {
  const query = readQueryLocalFonts();
  if (typeof query !== "function") return null;
  let fonts: LocalFontData[];
  try {
    fonts = await query.call(window);
  } catch {
    return null;
  }
  // macOS lists private system faces as ".SF NS…"; they are not selectable.
  const families = [...new Set(fonts.map(font => font.family.trim()))]
    .filter(family => family !== "" && !family.startsWith("."))
    .sort((a, b) => a.localeCompare(b));
  if (families.length === 0) return null;
  return { families, monospace: monospaceFamilies(families) };
}

function readQueryLocalFonts(): QueryLocalFonts | undefined {
  return (window as Window & { queryLocalFonts?: QueryLocalFonts }).queryLocalFonts;
}

/**
 * Narrow (`i`) and wide (`W`) runs render at the same width only in a
 * monospace face. Measured on a detached canvas against each installed family.
 */
function monospaceFamilies(families: readonly string[]): Set<string> {
  const context = document.createElement("canvas").getContext("2d");
  if (!context) return new Set();
  const mono = new Set<string>();
  for (const family of families) {
    context.font = `16px "${family.replace(/["\\]/g, "")}", serif`;
    const narrow = context.measureText("iiiiiiii").width;
    const wide = context.measureText("WWWWWWWW").width;
    if (narrow > 0 && Math.abs(narrow - wide) < 0.5) mono.add(family);
  }
  return mono;
}
