import { readFile } from "node:fs/promises";
import path from "node:path";

type FontWeight = 400 | 500 | 600 | 700;

export interface OGFont {
  name: string;
  data: ArrayBuffer;
  weight: FontWeight;
  style: "normal";
}

// Static latin WOFF cuts from @fontsource (SIL OFL 1.1): Satori reads TTF/OTF/WOFF, not the
// variable WOFF2 builds the site and product load.
const FONT_MANIFEST: ReadonlyArray<{ file: string; name: string; weight: FontWeight }> = [
  { file: "figtree-latin-400-normal.woff", name: "Figtree", weight: 400 },
  { file: "figtree-latin-500-normal.woff", name: "Figtree", weight: 500 },
  { file: "figtree-latin-600-normal.woff", name: "Figtree", weight: 600 },
  { file: "instrument-sans-latin-400-normal.woff", name: "Instrument Sans", weight: 400 },
  { file: "geist-mono-latin-500-normal.woff", name: "Geist Mono", weight: 500 },
];

let cached: ReadonlyArray<OGFont> | null = null;

function fontPath(file: string): string {
  const siteRoot = process.env.COMPOZY_SITE_ROOT ?? process.cwd();
  return path.join(siteRoot, "lib", "og", "fonts", file);
}

export async function loadOGFonts(): Promise<ReadonlyArray<OGFont>> {
  if (cached) return cached;
  const loaded = await Promise.all(
    FONT_MANIFEST.map(async entry => {
      const buf = await readFile(fontPath(entry.file));
      const data = buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength) as ArrayBuffer;
      return { name: entry.name, weight: entry.weight, style: "normal" as const, data };
    })
  );
  cached = loaded;
  return loaded;
}
