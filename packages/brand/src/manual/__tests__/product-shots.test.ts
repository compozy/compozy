// UT-013: export:product-shots fails loudly on a pinned screen the seed or Storybook no longer
// holds and leaves the published captures untouched (US-004.EC-1). The script runs for real
// against a stub seeded app (--base-url) and a fixture Storybook index; it exits before any
// browser or daemon starts, so no build is needed.
import { spawn } from "node:child_process";
import { mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer, type Server } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";

import {
  PRODUCT_SHOT_PINS,
  SEED_WORKSPACE,
  resolveAppPins,
  resolveStoryPins,
  type AppShotPin,
  type StoryShotPin,
} from "../art/product-pins";

const SCRIPT = fileURLToPath(new URL("../../../scripts/product-shots.ts", import.meta.url));
const SEEDED = [{ id: "ws_seed", name: SEED_WORKSPACE }];
const storyPins = PRODUCT_SHOT_PINS.filter((p): p is StoryShotPin => p.source === "story");
const scopedPin = PRODUCT_SHOT_PINS.find(
  (p): p is AppShotPin => p.source === "app" && p.workspace !== undefined
);

const dirs: string[] = [];
const servers: Server[] = [];
const temp = (tag: string) => {
  const d = mkdtempSync(join(tmpdir(), `brand-ut013-${tag}-`));
  dirs.push(d);
  return d;
};
afterEach(() => {
  for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
  for (const s of servers.splice(0)) s.close();
});

/** Storybook index.json holding every story pin except `drop`. */
function storybookWithout(drop: string) {
  const dir = temp("sb");
  const entries = Object.fromEntries(
    storyPins
      .filter(p => p.name !== drop)
      .map(p => {
        const id = `story-${p.name}`;
        return [id, { id, type: "story", importPath: `./${p.story}`, exportName: p.exportName }];
      })
  );
  writeFileSync(join(dir, "index.json"), JSON.stringify({ v: 5, entries }));
  return dir;
}

/** A stub of the seeded app's workspace list. */
function seededApp(workspaces: { id: string; name: string }[]): Promise<string> {
  const server = createServer((req, res) => {
    if (req.url?.startsWith("/api/workspaces")) {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({ workspaces }));
    } else res.writeHead(404).end();
  });
  servers.push(server);
  return new Promise(ok =>
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address();
      ok(`http://127.0.0.1:${typeof addr === "object" && addr ? addr.port : 0}`);
    })
  );
}

function runScript(args: string[]): Promise<{ status: number | null; stderr: string }> {
  return new Promise(done => {
    const child = spawn("bun", [SCRIPT, ...args]);
    let stderr = "";
    child.stderr.on("data", chunk => (stderr += String(chunk)));
    child.on("close", status => done({ status, stderr }));
  });
}

function publishedCapture() {
  const out = temp("out");
  writeFileSync(join(out, "board-dark.webp"), "previous capture");
  return out;
}

function expectUntouched(out: string) {
  expect(readdirSync(out)).toEqual(["board-dark.webp"]);
  expect(readFileSync(join(out, "board-dark.webp"), "utf8")).toBe("previous capture");
}

describe("export:product-shots", () => {
  it("scopes every app pin to its seeded workspace by name", () => {
    const { resolved, missing } = resolveAppPins(SEEDED);
    expect(missing).toEqual([]);
    expect(resolved.map(r => r.pin.name)).toEqual(
      PRODUCT_SHOT_PINS.filter(p => p.source === "app").map(p => p.name)
    );
    for (const { pin, path } of resolved) {
      expect(path.startsWith(pin.route)).toBe(true);
      expect(path.includes("workspace=ws_seed")).toBe(pin.workspace !== undefined);
    }
  });

  it("resolves every story pin to its story id through importPath and exportName", () => {
    const index = JSON.parse(readFileSync(join(storybookWithout("none"), "index.json"), "utf8"));
    const { resolved, missing } = resolveStoryPins(index);
    expect(missing).toEqual([]);
    expect(resolved.map(r => r.id)).toEqual(storyPins.map(p => `story-${p.name}`));
  });

  it("exits 1 naming a pin whose seeded workspace is gone and keeps the captures", async () => {
    expect(scopedPin).toBeDefined();
    const base = await seededApp([{ id: "ws_other", name: "Another workspace" }]);
    const out = publishedCapture();

    const run = await runScript([
      "--base-url",
      base,
      "--storybook-dir",
      storybookWithout("none"),
      "--out",
      out,
    ]);

    expect(run.status).toBe(1);
    expect(run.stderr).toContain(
      `${scopedPin?.name}: ${scopedPin?.route} needs seeded workspace "${SEED_WORKSPACE}"`
    );
    expectUntouched(out);
  });

  it("exits 1 naming a missing pinned story and keeps the captures", async () => {
    const [pin] = storyPins;
    expect(pin).toBeDefined();
    const base = await seededApp(SEEDED);
    const out = publishedCapture();

    const run = await runScript([
      "--base-url",
      base,
      "--storybook-dir",
      storybookWithout(pin?.name ?? ""),
      "--out",
      out,
    ]);

    expect(run.status).toBe(1);
    expect(run.stderr).toContain(`${pin?.name}: web/${pin?.story} → ${pin?.exportName}`);
    expectUntouched(out);
  });
});
