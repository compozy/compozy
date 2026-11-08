// QA screenshot of a manual page with Playwright (Chrome):
//   bun run shot <url> <out.png> [width=1440] [height=900] [full=1]
// Use this, not Chrome --headless --screenshot: the CLI skips decoding raster visuals.
import { chromium } from "playwright";

const [url, out, w = "1440", h = "900", full = "1"] = process.argv.slice(2);
if (!url || !out) {
  console.error("usage: bun run shot <url> <out.png> [width] [height] [full 1|0]");
  process.exit(1);
}
const browser = await chromium.launch({ channel: "chrome" });
const page = await browser.newPage({ viewport: { width: Number(w), height: Number(h) } });
await page.goto(url, { waitUntil: "networkidle" });
await page.evaluate(() => document.fonts.ready);
if (full === "1") {
  // walk the page so offscreen rasters decode and reveal animations fire before the capture
  const height = await page.evaluate(() => document.body.scrollHeight);
  for (let y = 0; y < height; y += 700) {
    await page.evaluate(top => window.scrollTo(0, top), y);
    await page.waitForTimeout(120);
  }
  await page.evaluate(() => window.scrollTo(0, 0));
}
await page.waitForTimeout(500);
await page.screenshot({ path: out, fullPage: full === "1" });
await browser.close();
