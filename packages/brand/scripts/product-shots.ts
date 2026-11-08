// export:product-shots — captures the pinned web/ Storybook stories in the current @compozy/ui
// theme and writes web-size WebP files to public/brand/product/<shot>-<theme>.webp (ADR-005).
//
//   bun run export:product-shots                         builds web/ Storybook, then captures
//   bun run export:product-shots --storybook-dir <dir>   reuses a static Storybook build
//   bun run export:product-shots --out <dir> --only desktop,board
//
// Every pin is resolved against the Storybook index before anything is written: a missing story
// exits 1 naming it (US-004.EC-1, UT-013). Captures land in a staging dir and replace the
// published files only after every capture succeeded, so a failed run leaves them untouched.
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
  resolvePins,
  type ProductShotPin,
  type ShotTheme,
} from "../src/manual/art/product-pins";

const PKG = resolve(fileURLToPath(new URL("..", import.meta.url)));
const WEB = resolve(PKG, "../../web");
const MAX_BYTES = 400 * 1024;
const MAX_WIDTH = 2560;
const QUALITIES = [0.9, 0.84, 0.78, 0.7, 0.62, 0.54];
const SETTLE_MS = 1800;

interface Args {
  storybookDir?: string;
  out: string;
  only?: Set<string>;
}

function parseArgs(argv: string[]): Args {
  const args: Args = { out: join(PKG, "public/brand/product") };
  for (let i = 0; i < argv.length; i++) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (flag === "--storybook-dir" && value) args.storybookDir = resolve(value);
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

async function capturePng(
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
    await page.addStyleTag({
      content:
        "*,*::before,*::after{animation-play-state:paused!important;transition:none!important;caret-color:transparent!important}",
    });
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

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const pins = PRODUCT_SHOT_PINS.filter(p => !args.only || args.only.has(p.name));
  if (args.only && pins.length !== args.only.size) fail(`--only names an unknown shot`);

  const storybook = args.storybookDir ?? buildStorybook();
  const indexFile = join(storybook, "index.json");
  if (!existsSync(indexFile)) fail(`no Storybook index at ${indexFile}`);
  const { resolved, missing } = resolvePins(JSON.parse(readFileSync(indexFile, "utf8")), pins);
  if (missing.length) {
    for (const m of missing)
      console.error(`error: pinned story missing from Storybook index — ${m}`);
    process.exit(1);
  }

  const { chromium } = await import("playwright");
  const { server, base } = await serve(storybook);
  const browser = await chromium.launch();
  const staging = mkdtempSync(join(tmpdir(), "brand-product-shots-"));
  try {
    const encoder = await browser.newPage();
    const rows: string[] = [];
    for (const { pin, id } of resolved) {
      for (const theme of SHOT_THEMES) {
        const webp = await encodeWebp(encoder, await capturePng(browser, base, pin, id, theme));
        if (webp.length > MAX_BYTES)
          throw new Error(`${pin.name}-${theme} is ${webp.length} B > 400 KB`);
        writeFileSync(join(staging, productShotFile(pin.name, theme)), webp);
        rows.push(
          `${productShotFile(pin.name, theme).padEnd(24)} ${(webp.length / 1024).toFixed(0).padStart(4)} KB  ${id}`
        );
      }
    }
    mkdirSync(args.out, { recursive: true });
    cpSync(staging, args.out, { recursive: true });
    console.log(rows.join("\n"));
    console.log(`wrote ${rows.length} captures → ${args.out}`);
  } finally {
    await browser.close();
    server.close();
    rmSync(staging, { recursive: true, force: true });
  }
}

main().catch((error: unknown) => fail(error instanceof Error ? error.message : String(error)));
