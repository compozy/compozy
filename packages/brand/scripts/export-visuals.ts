// export:visuals — every /visual/$id (Atmosphere crops, framed product shots, diagrams) in dark
// and light at 2x → $BRAND_EXPORT_DIR/visuals/<name>-<theme>.png (_dx.md, US-003.AC-2).
//   bun run export:visuals
import { mkdirSync } from "node:fs";
import { join } from "node:path";

import { visualExportIds } from "../src/manual/art/visuals";
import {
  assertReachable,
  baseUrlOf,
  errorText,
  fail,
  resolveExportDir,
  settleInPage,
} from "./export-env";

let outDir: string;
try {
  outDir = resolveExportDir(process.env);
} catch (e) {
  fail(errorText(e));
}
const base = baseUrlOf(process.env);
await assertReachable(base).catch(e => fail(errorText(e)));

const out = join(outDir, "visuals");
mkdirSync(out, { recursive: true });
const { chromium } = await import("playwright");
const browser = await chromium.launch({ channel: "chrome" });
const page = await browser.newPage({
  viewport: { width: 1600, height: 900 },
  deviceScaleFactor: 2,
});
const ids = visualExportIds();
const failures: string[] = [];

for (const id of ids) {
  try {
    await page.goto(`${base}/visual/${id}`, { waitUntil: "load" });
    await page.evaluate(settleInPage, "[data-export]");
    await page.locator("[data-export]").screenshot({ path: join(out, `${id}.png`) });
    console.log(`✓ ${id}`);
  } catch (e) {
    failures.push(`${id}: ${errorText(e)}`);
    console.error(`✗ ${id}: ${errorText(e)}`);
  }
}
await browser.close();

console.log(`${ids.length - failures.length}/${ids.length} visuals exported to ${out}`);
if (failures.length > 0) fail(`${failures.length} visual(s) failed:\n- ${failures.join("\n- ")}`);
