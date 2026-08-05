/**
 * Demo flow — <what this demo shows, one line>.
 *
 * Copy this template into the project workdir (a directory whose package
 * resolves `@playwright/test`) and replace every <placeholder>. The recorder
 * lib path is absolute so the flow runs from anywhere inside the project.
 */
import { chromium } from "@playwright/test";
import { openDemo } from "<abs-path-to-skill>/scripts/recorder.mjs";

const APP_URL = "<app url from .demo-video.yaml>";

// ---------------------------------------------------------------- state prep
// Drive the app/API into the exact state the opening shot needs BEFORE the
// browser opens: seed or delete data, close leftover windows, log in. This is
// per-demo work — follow the `state` notes in .demo-video.yaml.
async function prepare() {
  // e.g. const res = await fetch(`${APP_URL}/api/...`, { method: "DELETE" });
}

await prepare();

const demo = await openDemo(chromium, {
  url: APP_URL,
  outDir: new URL(".", import.meta.url).pathname,
  viewport: { width: 1920, height: 1080 },   // from .demo-video.yaml capture.viewport
  surface: { width: 2880, height: 1620 },    // from .demo-video.yaml capture.surface
  readySelector: "<selector that marks the app as settled>",
});
const { page, mouse, beat, park, startCapture, finish } = demo;

// ---------------------------------------------------------------- choreography
await startCapture();
await beat(null, 1, { ease: 0, label: "establish" });
await park();                       // cursor fades in at a neutral spot
await page.waitForTimeout(700);

// One gesture per intent: frame the target with beat(), then act on it.
// const button = page.getByRole("button", { name: "<label>" });
// await beat(button, 1.4, { ease: 900, label: "<moment>" });
// await mouse.clickLocator(button, { dwell: 420 });
//
// const dialog = page.getByTestId("<testid>");
// await dialog.waitFor({ state: "visible" });
// await beat(dialog, 1.25, { ease: 800, label: "dialog" });
// await page.waitForTimeout(1100);          // let the viewer read what opened
//
// await page.keyboard.type("<text>", { delay: 130 });

// Pull back to full frame so the result reads as a whole, then hold.
await beat(null, 1, { ease: 1000, label: "reveal" });
await page.waitForTimeout(2400);

await finish();
