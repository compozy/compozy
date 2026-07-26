// Mutating helper — writes PNGs to disk via headless Chrome + CDP.
// Boots one Chrome instance, drives it through chrome-remote-interface, and emits one PNG per --shot pair.
// Required deps in the working dir: `chrome-launcher`, `chrome-remote-interface`.
// Usage:
//   bun run cap.mjs --out <dir> --width 1440 --height 900 --wait 2200 \
//     --shot <name> <url> [--shot <name> <url> ...]
// Output:
//   stdout: "chrome port <N>", then "saved <name>" per successful shot.
//   stderr: chrome-launcher / CDP errors and per-shot "FAIL <name> <msg>" lines.
//   exit 1 when any requested shot fails after attempting the complete set.

import { launch } from "chrome-launcher";
import CDP from "chrome-remote-interface";
import { writeFileSync } from "node:fs";

const argv = process.argv.slice(2);
let outDir = "./out";
let width = 1440;
let height = 900;
let waitMs = 1500;
const targets = [];
let failedShots = 0;
for (let i = 0; i < argv.length; i++) {
  const flag = argv[i];
  if (flag === "--out") outDir = argv[++i];
  else if (flag === "--width") width = Number(argv[++i]);
  else if (flag === "--height") height = Number(argv[++i]);
  else if (flag === "--wait") waitMs = Number(argv[++i]);
  else if (flag === "--shot") {
    const name = argv[++i];
    const url = argv[++i];
    if (!name || !url) {
      console.error("USAGE ERROR: --shot expects <name> <url>");
      process.exit(2);
    }
    targets.push({ name, url });
  } else {
    console.error(`USAGE ERROR: unknown flag ${flag}`);
    process.exit(2);
  }
}
if (targets.length === 0) {
  console.error("USAGE ERROR: at least one --shot <name> <url> pair required");
  process.exit(2);
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const chrome = await launch({
  chromeFlags: [
    "--headless=new",
    "--disable-gpu",
    "--hide-scrollbars",
    "--no-sandbox",
    "--disable-dev-shm-usage",
    "--mute-audio",
    `--window-size=${width},${height}`,
  ],
});
console.log("chrome port", chrome.port);

let client;
try {
  client = await CDP({ port: chrome.port });
  const { Page, Emulation, Network, Runtime, DOM } = client;
  await Promise.all([
    Page.enable(),
    Network.enable(),
    Runtime.enable(),
    DOM.enable(),
  ]);
  await Emulation.setDeviceMetricsOverride({
    width,
    height,
    deviceScaleFactor: 1,
    mobile: false,
  });

  for (const target of targets) {
    try {
      await Page.navigate({ url: target.url });
      await Page.loadEventFired();
      await sleep(waitMs);
      try {
        await Runtime.evaluate({
          expression:
            "document.fonts && document.fonts.ready && document.fonts.ready.then(() => 1)",
          awaitPromise: true,
          timeout: 5000,
        });
      } catch (fontErr) {
        console.warn(`WARN ${target.name} fonts.ready timeout: ${fontErr.message}`);
      }
      const { data } = await Page.captureScreenshot({ format: "png" });
      writeFileSync(`${outDir}/${target.name}.png`, Buffer.from(data, "base64"));
      console.log("saved", target.name);
    } catch (shotErr) {
      failedShots += 1;
      console.error("FAIL", target.name, shotErr.message);
    }
  }
} finally {
  try {
    if (client) await client.close();
  } finally {
    await chrome.kill();
  }
}

if (failedShots > 0) {
  process.exitCode = 1;
}
