import { mkdirSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { chromium } from "@playwright/test";

import { repoRoot } from "../../node/paths.ts";
import { maxWidth, minWidth } from "../../shell/view.ts";
import { stringFlag, UsageError, type ParsedArgs } from "../args.ts";
import { ensureGallery } from "../server.ts";
import { prototypeUrl, resolveTarget } from "./url.ts";

function widthOf(value: string | undefined) {
  if (value === undefined) return 1440;
  const width = Number(value);
  if (!Number.isInteger(width) || width < minWidth || width > maxWidth) {
    throw new UsageError(`--width must be an integer from ${minWidth} to ${maxWidth}`);
  }
  return width;
}

/**
 * `proto screenshot <slug> [--state] [--theme] [--width] [--out]`: a full-page PNG of the
 * standalone prototype (no gallery chrome) in Playwright's Chromium, against a running
 * `proto dev` or one started (and stopped) for this capture.
 */
export async function screenshot(args: ParsedArgs) {
  const { slug, entry, state, theme } = resolveTarget(args);
  const width = widthOf(stringFlag(args, "width"));
  const resolvedTheme = theme ?? "dark";
  const resolvedState = state ?? entry.meta?.states[0]?.id ?? "default";
  const out = resolve(
    stringFlag(args, "out") ??
      resolve(repoRoot, ".tmp/proto", `${slug}--${resolvedState}--${resolvedTheme}--${width}.png`)
  );
  const gallery = await ensureGallery();
  // The owned server stops even when the browser never starts.
  try {
    const browser = await chromium.launch();
    try {
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        deviceScaleFactor: 2,
      });
      await page.goto(
        prototypeUrl(slug, { state: resolvedState, theme: resolvedTheme }, gallery.origin).href,
        // A cold server optimizes dependencies on the first visit.
        { timeout: 120_000 }
      );
      await page.waitForFunction(() => document.documentElement.dataset.prototypeStatus, null, {
        timeout: 60_000,
      });
      const status = await page.evaluate(() => document.documentElement.dataset.prototypeStatus);
      if (status !== "ready") throw new Error(`${slug} failed to render (see proto dev)`);
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      mkdirSync(dirname(out), { recursive: true });
      await page.screenshot({ path: out, fullPage: true, animations: "disabled" });
      console.log(relative(process.cwd(), out) || out);
    } finally {
      await browser.close();
    }
  } finally {
    await gallery.stop();
  }
}
