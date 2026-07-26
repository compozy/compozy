import { readFile } from "node:fs/promises";
import path from "node:path";

type FontWeight = 400 | 500 | 600 | 700;

export interface OGFont {
  name: string;
  data: ArrayBuffer;
  weight: FontWeight;
  style: "normal";
}

const FONT_MANIFEST: ReadonlyArray<{ file: string; name: string; weight: FontWeight }> = [
  { file: "Inter-Regular.ttf", name: "Inter", weight: 400 },
  { file: "Inter-Medium.ttf", name: "Inter", weight: 500 },
  { file: "Inter-SemiBold.ttf", name: "Inter", weight: 600 },
  { file: "PlayfairDisplay-Regular.ttf", name: "Playfair Display", weight: 400 },
  { file: "JetBrainsMono-Medium.ttf", name: "JetBrains Mono", weight: 500 },
];

let cached: ReadonlyArray<OGFont> | null = null;

function fontPath(file: string): string {
  const siteRoot = process.env.AGH_SITE_ROOT ?? process.cwd();
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
