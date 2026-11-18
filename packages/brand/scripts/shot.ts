// QA screenshot of a manual page with Playwright (Chrome), written under the export dir:
//   bun run shot <url> <name.png> [width=1440] [height=900] [full=1]
//   → $BRAND_EXPORT_DIR/shots/<name.png> (an absolute <name.png> must sit inside BRAND_EXPORT_DIR)
// Use this, not Chrome --headless --screenshot: the CLI skips decoding raster visuals.
import { mkdirSync } from "node:fs";
import { dirname } from "node:path";

import { errorText, fail, resolveExportDir, resolveExportFile } from "./export-env";

const [url, name, w = "1440", h = "900", full = "1"] = process.argv.slice(2);
if (!url || !name) fail("usage: bun run shot <url> <name.png> [width] [height] [full 1|0]");

let out: string;
try {
  out = resolveExportFile(resolveExportDir(process.env), name, "shots");
} catch (e) {
  fail(errorText(e));
}
mkdirSync(dirname(out), { recursive: true });

const { chromium } = await import("playwright");
const browser = await chromium.launch({ channel: "chrome" });
const page = await browser.newPage({ viewport: { width: Number(w), height: Number(h) } });
await page.goto(url, { waitUntil: "networkidle" });
await page.evaluate(() => document.fonts.ready);
if (full === "1") {
  // Walk the page so offscreen rasters decode and reveal animations fire before the capture. The
  // steps must run in order, so the walk is one in-page chain rather than a round trip per step.
  await page.evaluate(async () => {
    const walk = (top: number): Promise<void> =>
      top >= document.body.scrollHeight
        ? Promise.resolve()
        : new Promise(done => {
            window.scrollTo(0, top);
            setTimeout(() => done(walk(top + 700)), 120);
          });
    await walk(0);
    window.scrollTo(0, 0);
  });
}
await page.waitForTimeout(500);
await page.screenshot({ path: out, fullPage: full === "1" });
await browser.close();
console.log(`shot → ${out}`);
