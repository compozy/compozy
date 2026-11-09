// export:product-shots — captures every pinned product screen in the current @compozy/ui theme and
// writes web-size WebP files to public/brand/product/<shot>-<theme>.webp (ADR-005). App pins are
// real routes of the web app running against the demo seed; a story pin renders from web/ Storybook.
//
//   bun run export:product-shots                          seeds an isolated daemon, captures, stops it
//   bun run export:product-shots --base-url <url>         reuses an already running seeded app
//   bun run export:product-shots --web-dist <dir>         reuses a web/ build for the seeded daemon
//   bun run export:product-shots --storybook-dir <dir>    reuses a static web/ Storybook build
//   bun run export:product-shots --out <dir> --only desktop,board
//
// Every pin is resolved before anything is captured: a seeded workspace or pinned story that is
// missing exits 1 naming the pin (US-004.EC-1, UT-013); so does a route whose ready element never
// renders. Captures land in a staging dir and replace the published files only after every capture
// succeeded, so a failed run leaves them untouched.
import { spawnSync } from "node:child_process";
import {
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createServer, type Server } from "node:http";
import { tmpdir } from "node:os";
import { extname, join, normalize, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import {
  PRODUCT_SHOT_PINS,
  SHOT_THEMES,
  productShotFile,
  resolveAppPins,
  resolveStoryPins,
  type AppShotPin,
  type ProductShotPin,
  type ShotTheme,
} from "../src/manual/art/product-pins";
import { startSeededApp } from "./product-shots-lab";

const PKG = resolve(fileURLToPath(new URL("..", import.meta.url)));
const REPO = resolve(PKG, "../..");
const WEB = resolve(REPO, "web");
const MAX_BYTES = 400 * 1024;
const MAX_WIDTH = 2560;
const QUALITIES = [0.9, 0.84, 0.78, 0.7, 0.62, 0.54];
const SETTLE_MS = 1800;

interface Args {
  baseUrl?: string;
  webDist?: string;
  storybookDir?: string;
  out: string;
  only?: Set<string>;
}

function parseArgs(argv: string[]): Args {
  const args: Args = { out: join(PKG, "public/brand/product") };
  for (let i = 0; i < argv.length; i++) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (flag === "--base-url" && value) args.baseUrl = value.replace(/\/+$/, "");
    else if (flag === "--web-dist" && value) args.webDist = resolve(value);
    else if (flag === "--storybook-dir" && value) args.storybookDir = resolve(value);
    else if (flag === "--out" && value) args.out = resolve(value);
    else if (flag === "--only" && value) args.only = new Set(value.split(","));
    else fail(`unknown argument ${flag}`);
    i++;
  }
  return args;
}

function fail(message: string): never {
  console.error(`error: ${message}`);
  process.exit(1);
}

function buildStorybook(): string {
  const dir = mkdtempSync(join(tmpdir(), "brand-product-storybook-"));
  console.log(`building web/ Storybook → ${dir}`);
  const run = spawnSync("bunx", ["storybook", "build", "--quiet", "-o", dir], {
    cwd: WEB,
    stdio: "inherit",
  });
  if (run.status !== 0) fail(`web/ Storybook build failed (exit ${run.status})`);
  return dir;
}

const MIME: Record<string, string> = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".mjs": "text/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".woff2": "font/woff2",
  ".ico": "image/x-icon",
};

function serve(root: string): Promise<{ server: Server; base: string }> {
  const server = createServer((req, res) => {
    const path = normalize(decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname));
    const file = join(root, path.endsWith("/") ? `${path}index.html` : path);
    if (!file.startsWith(root) || !existsSync(file)) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, { "content-type": MIME[extname(file)] ?? "application/octet-stream" });
    res.end(readFileSync(file));
  });
  return new Promise(ok => {
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address();
      const port = typeof addr === "object" && addr ? addr.port : 0;
      ok({ server, base: `http://127.0.0.1:${port}` });
    });
  });
}

type Browser = import("playwright").Browser;
type Page = import("playwright").Page;

async function captureStoryPng(
  browser: Browser,
  base: string,
  pin: ProductShotPin,
  id: string,
  theme: ShotTheme
) {
  const context = await browser.newContext({
    viewport: pin.viewport,
    deviceScaleFactor: 2,
    reducedMotion: "reduce",
    colorScheme: theme,
  });
  try {
    const page = await context.newPage();
    const url = `${base}/iframe.html?id=${id}&viewMode=story&globals=theme:${theme}`;
    await page.goto(url, { waitUntil: "networkidle" });
    await page.waitForSelector("#storybook-root > *", { timeout: 20_000 });
    if (await page.locator(".sb-errordisplay:visible, .sb-nopreview:visible").count()) {
      throw new Error(`story ${id} rendered an error`);
    }
    return await settleAndShoot(page, pin);
  } finally {
    await context.close();
  }
}

const FREEZE =
  "*,*::before,*::after{animation-play-state:paused!important;transition:none!important;caret-color:transparent!important}";

async function settleAndShoot(page: Page, pin: ProductShotPin): Promise<Buffer> {
  await page.addStyleTag({ content: FREEZE });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(SETTLE_MS);
  if (!pin.selector) return await page.screenshot({ type: "png", animations: "disabled" });
  const box = await page.locator(pin.selector).first().boundingBox({ timeout: 10_000 });
  if (!box) throw new Error(`${pin.name}: selector ${pin.selector} is not visible`);
  const pad = pin.pad ?? 0;
  const x = Math.max(0, box.x - pad);
  const y = Math.max(0, box.y - pad);
  const clip = {
    x,
    y,
    width: Math.min(pin.viewport.width, box.x + box.width + pad) - x,
    height: Math.min(pin.viewport.height, box.y + box.height + pad) - y,
  };
  return await page.screenshot({ type: "png", animations: "disabled", clip });
}

// The window layout persists daemon-side (global scope, default profile), so every capture
// starts from an empty desktop: close each open window, then open the pinned route.
const WM_PATH = "/api/workspaces/global/window-manager";
const WM_CLIENT = "brand-product-shots";
/** web/src/systems/theme/lib/theme-preference.ts THEME_STORAGE_KEY */
const THEME_KEY = "compozy.theme";

async function closeAllWindows(base: string) {
  const json = { "content-type": "application/json" };
  await fetch(`${base}${WM_PATH}/clients?profile=default`, {
    method: "POST",
    headers: json,
    body: JSON.stringify({
      workspace_id: "global",
      client_id: WM_CLIENT,
      kind: "browser",
      context: { workspace_trusted: true },
    }),
  });
  for (let attempt = 0; attempt < 40; attempt++) {
    const state = (await (await fetch(`${base}${WM_PATH}?profile=default`)).json()) as {
      revision: number;
      windows?: Record<string, unknown>;
    };
    const [id] = Object.keys(state.windows ?? {});
    if (!id) return;
    await fetch(`${base}${WM_PATH}/commands?profile=default`, {
      method: "POST",
      headers: json,
      body: JSON.stringify({
        workspace_id: "global",
        command_id: "window.close",
        expected_revision: state.revision,
        client_id: WM_CLIENT,
        actor: { kind: "web", id: WM_CLIENT },
        origin: "web",
        payload: { window_id: id, minimize: false },
      }),
    });
  }
  throw new Error("could not close the open windows of the seeded desktop");
}

async function captureAppPng(
  browser: Browser,
  base: string,
  pin: AppShotPin,
  path: string,
  theme: ShotTheme
) {
  await closeAllWindows(base);
  const context = await browser.newContext({
    viewport: pin.viewport,
    deviceScaleFactor: 2,
    reducedMotion: "reduce",
    colorScheme: theme,
  });
  // The app defaults to dark and reads its theme from local storage, not the color scheme.
  await context.addInitScript(({ key, value }) => window.localStorage.setItem(key, value), {
    key: THEME_KEY,
    value: theme,
  });
  try {
    const page = await context.newPage();
    await page.goto(`${base}${path}`, { waitUntil: "load" });
    await page.keyboard.press("Escape");
    for (const step of pin.steps ?? []) {
      if ("click" in step) await page.locator(step.click).first().click({ timeout: 15_000 });
      else if ("press" in step) await page.keyboard.press(step.press);
      else await page.locator(step.waitFor).first().waitFor({ timeout: 15_000 });
    }
    try {
      await page.locator(pin.ready).first().waitFor({ state: "visible", timeout: 20_000 });
    } catch {
      throw new Error(`${pin.name}: ${path} never rendered ${pin.ready}`);
    }
    return await settleAndShoot(page, pin);
  } finally {
    await context.close();
  }
}

/** PNG → WebP through Chromium's encoder: cap the width, step quality down until ≤ MAX_BYTES. */
async function encodeWebp(page: Page, png: Buffer): Promise<Buffer> {
  const b64 = await page.evaluate(
    async ({ src, maxWidth, maxBytes, qualities }) => {
      const img = new Image();
      img.src = src;
      await img.decode();
      const scale = Math.min(1, maxWidth / img.naturalWidth);
      const canvas = document.createElement("canvas");
      canvas.width = Math.round(img.naturalWidth * scale);
      canvas.height = Math.round(img.naturalHeight * scale);
      const ctx = canvas.getContext("2d");
      if (!ctx) throw new Error("no 2d context");
      ctx.imageSmoothingQuality = "high";
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
      let out = "";
      for (const q of qualities) {
        out = canvas.toDataURL("image/webp", q).split(",")[1] ?? "";
        if ((out.length * 3) / 4 <= maxBytes) break;
      }
      return out;
    },
    {
      src: `data:image/png;base64,${png.toString("base64")}`,
      maxWidth: MAX_WIDTH,
      maxBytes: MAX_BYTES,
      qualities: QUALITIES,
    }
  );
  const webp = Buffer.from(b64, "base64");
  if (webp.subarray(8, 12).toString() !== "WEBP") throw new Error("encoder did not produce WebP");
  return webp;
}

async function seededWorkspaces(base: string): Promise<{ id: string; name: string }[]> {
  const response = await fetch(`${base}/api/workspaces`).catch(() => null);
  // Throws (not fail()) so the caller's finally still stops a seeded daemon.
  if (!response?.ok) throw new Error(`no seeded app answering on ${base}/api/workspaces`);
  const body = (await response.json()) as { workspaces?: { id: string; name: string }[] };
  return body.workspaces ?? [];
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const pins = PRODUCT_SHOT_PINS.filter(p => !args.only || args.only.has(p.name));
  if (args.only && pins.length !== args.only.size) fail(`--only names an unknown shot`);

  // Story pins resolve against a Storybook index first: no daemon starts for a broken pin.
  let storybook: string | undefined;
  let stories: ReturnType<typeof resolveStoryPins>["resolved"] = [];
  if (pins.some(p => p.source === "story")) {
    storybook = args.storybookDir ?? buildStorybook();
    const indexFile = join(storybook, "index.json");
    if (!existsSync(indexFile)) fail(`no Storybook index at ${indexFile}`);
    const story = resolveStoryPins(JSON.parse(readFileSync(indexFile, "utf8")), pins);
    for (const m of story.missing) console.error(`error: pinned story missing — ${m}`);
    if (story.missing.length) process.exit(1);
    stories = story.resolved;
  }

  const lab = args.baseUrl ? undefined : await startSeededApp(REPO, args.webDist);
  const appBase = args.baseUrl ?? lab?.base ?? "";
  const staging = mkdtempSync(join(tmpdir(), "brand-product-shots-"));
  let server: Server | undefined;
  let browser: Browser | undefined;
  try {
    const app = resolveAppPins(await seededWorkspaces(appBase), pins);
    if (app.missing.length) {
      throw new Error(
        app.missing.map(m => `pinned screen missing from the seed — ${m}`).join("\nerror: ")
      );
    }
    const served = storybook ? await serve(storybook) : undefined;
    server = served?.server;
    const chromium = await import("playwright").then(m => m.chromium.launch());
    browser = chromium;
    const encoder = await chromium.newPage();
    const rows: string[] = [];
    const jobs = [
      ...app.resolved.map(({ pin, path }) => ({
        pin,
        label: path,
        shoot: (b: Browser, theme: ShotTheme) => captureAppPng(b, appBase, pin, path, theme),
      })),
      ...stories.map(({ pin, id }) => ({
        pin,
        label: `story ${id}`,
        shoot: (b: Browser, theme: ShotTheme) =>
          captureStoryPng(b, served?.base ?? "", pin, id, theme),
      })),
    ];
    for (const job of jobs) {
      for (const theme of SHOT_THEMES) {
        const webp = await encodeWebp(encoder, await job.shoot(chromium, theme));
        if (webp.length > MAX_BYTES)
          throw new Error(`${job.pin.name}-${theme} is ${webp.length} B > 400 KB`);
        writeFileSync(join(staging, productShotFile(job.pin.name, theme)), webp);
        rows.push(
          `${productShotFile(job.pin.name, theme).padEnd(24)} ${(webp.length / 1024).toFixed(0).padStart(4)} KB  ${job.label}`
        );
      }
    }
    mkdirSync(args.out, { recursive: true });
    cpSync(staging, args.out, { recursive: true });
    console.log(rows.join("\n"));
    console.log(`wrote ${rows.length} captures → ${args.out}`);
  } finally {
    await browser?.close();
    server?.close();
    lab?.stop();
    rmSync(staging, { recursive: true, force: true });
  }
}

main().catch((error: unknown) => fail(error instanceof Error ? error.message : String(error)));
