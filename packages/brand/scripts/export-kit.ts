// export:kit — every kit template at native size (_dx.md, US-003).
//   bun run export:kit [-- --only id,id] [-- --set field=value ...]
// Reads the template list from $BRAND_BASE_URL/kit/index.json, screenshots each /kit/<id>
// [data-kit] once it sets data-kit-ready → $BRAND_EXPORT_DIR/kit/<id>.png; print templates also
// → <id>.pdf at trim size; public digital assets are copied to $BRAND_EXPORT_DIR/digital/.
// A template that never gets ready is reported by id; the rest still export; exit 1.
import { copyFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";

import { kitSpecs } from "../src/manual/kit-specs";
import { baseUrlOf, errorText, fail, fetchJson, resolveExportDir } from "./export-env";

const READY_TIMEOUT_MS = 20_000;

/** template id → public file name under $BRAND_EXPORT_DIR/digital/ */
const DIGITAL: Record<string, string> = {
  "x-header": "x/compozy-x-header-1500x500.png",
  "x-avatar": "x/compozy-x-avatar-400.png",
  "linkedin-banner": "linkedin/compozy-linkedin-banner-1584x396.png",
  "linkedin-cover": "linkedin/compozy-linkedin-company-cover-1128x191.png",
  "linkedin-avatar": "linkedin/compozy-linkedin-avatar-400.png",
  "youtube-banner": "youtube/compozy-youtube-banner-2560x1440.png",
  "youtube-avatar": "youtube/compozy-youtube-avatar-800.png",
  "github-social-preview": "github/compozy-github-social-preview-1280x640.png",
  "github-avatar": "github/compozy-github-avatar-500.png",
  "readme-header": "github/compozy-readme-header.png",
  "discord-banner": "discord/compozy-discord-banner-960x540.png",
  "discord-icon": "discord/compozy-discord-icon-512.png",
  "og-default": "og/compozy-og-default-1200x630.png",
  "og-blog": "og/compozy-og-blog-1200x630.png",
  "og-docs": "og/compozy-og-docs-1200x630.png",
  "og-changelog": "og/compozy-og-changelog-1200x630.png",
  "desktop-wallpaper": "wallpapers/compozy-wallpaper-2880x1800.png",
  wallpaper: "wallpapers/compozy-wallpaper-2560x1440.png",
  "email-signature": "email/compozy-email-signature-600x160.png",
  "newsletter-header": "email/compozy-newsletter-header-1200x400.png",
};

interface IndexRow {
  id: string;
  w: number;
  h: number;
  group: string;
}

function parseArgs(argv: string[]) {
  let only: Set<string> | undefined;
  const set: string[] = [];
  for (let i = 0; i < argv.length; i++) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (flag === "--only" && value) only = new Set(value.split(","));
    else if (flag === "--set" && value?.includes("=")) set.push(value);
    else fail(`unknown argument ${flag}`);
    i++;
  }
  const query = new URLSearchParams(
    set.map(s => [s.slice(0, s.indexOf("=")), s.slice(s.indexOf("=") + 1)])
  );
  return { only, query: set.length ? `?${query}` : "" };
}

const { only, query } = parseArgs(process.argv.slice(2));
let outDir: string;
try {
  outDir = resolveExportDir(process.env);
} catch (e) {
  fail(errorText(e));
}
const base = baseUrlOf(process.env);
const list = await fetchJson<IndexRow[]>(base, "/kit/index.json").catch(e => fail(errorText(e)));
const todo = list.filter(t => !only || only.has(t.id));
if (only) {
  const unknown = [...only].filter(id => !list.some(t => t.id === id));
  if (unknown.length) fail(`unknown kit id ${unknown.join(", ")}`);
}

const { chromium } = await import("playwright");
const browser = await chromium.launch({ channel: "chrome" });
const page = await browser.newPage({ deviceScaleFactor: 1 });
const failures: string[] = [];
mkdirSync(join(outDir, "kit"), { recursive: true });

const printById = new Map(kitSpecs.map(s => [s.id, s.print]));
for (const t of todo) {
  const out = join(outDir, "kit", `${t.id}.png`);
  try {
    await page.setViewportSize({ width: t.w, height: t.h });
    await page.goto(`${base}/kit/${t.id}${query}`, { waitUntil: "load" });
    const root = page.locator(`[data-kit="${t.id}"][data-kit-ready]`);
    await root.waitFor({ timeout: READY_TIMEOUT_MS }).catch(() => {
      throw new Error(`never set data-kit-ready within ${READY_TIMEOUT_MS / 1000} s`);
    });
    await root.screenshot({ path: out });
    const print = printById.get(t.id);
    if (print) {
      // PDF at trim size, vector type; the art reaches the trim edge and the printer adds bleed.
      const scale = Math.min(2, Math.max(0.1, ((print.wMm / 25.4) * 96) / t.w));
      await page.pdf({
        path: out.replace(/\.png$/, ".pdf"),
        width: `${print.wMm}mm`,
        height: `${print.hMm}mm`,
        scale,
        printBackground: true,
        pageRanges: "1",
        margin: { top: "0", right: "0", bottom: "0", left: "0" },
      });
    }
    const digital = DIGITAL[t.id];
    if (digital) {
      const copy = join(outDir, "digital", digital);
      mkdirSync(dirname(copy), { recursive: true });
      copyFileSync(out, copy);
    }
    console.log(`✓ ${t.id} ${t.w}×${t.h}`);
  } catch (e) {
    failures.push(`${t.id}: ${errorText(e)}`);
    console.error(`✗ ${t.id}: ${errorText(e)}`);
  }
}
await browser.close();

console.log(
  `${todo.length - failures.length}/${todo.length} templates exported to ${join(outDir, "kit")}`
);
if (failures.length > 0) fail(`${failures.length} template(s) failed:\n- ${failures.join("\n- ")}`);
