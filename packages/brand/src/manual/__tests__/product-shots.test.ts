// UT-013: export:product-shots fails loudly on a pinned story that left the Storybook index and
// leaves the published captures untouched (US-004.EC-1). The script runs for real against a
// fixture index; it exits before any browser starts, so no Storybook build is needed.
import { spawnSync } from "node:child_process";
import { mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";

import { PRODUCT_SHOT_PINS, resolvePins } from "../art/product-pins";

const SCRIPT = fileURLToPath(new URL("../../../scripts/product-shots.ts", import.meta.url));
const dirs: string[] = [];
const temp = (tag: string) => {
  const d = mkdtempSync(join(tmpdir(), `brand-ut013-${tag}-`));
  dirs.push(d);
  return d;
};
afterEach(() => {
  for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
});

/** Storybook index.json holding every pin except `drop`. */
function indexWithout(drop: string) {
  const entries = Object.fromEntries(
    PRODUCT_SHOT_PINS.filter(p => p.name !== drop).map(p => {
      const id = `story-${p.name}`;
      return [id, { id, type: "story", importPath: `./${p.story}`, exportName: p.exportName }];
    })
  );
  return { v: 5, entries };
}

describe("export:product-shots", () => {
  it("resolves every pin to its story id through importPath and exportName", () => {
    const { resolved, missing } = resolvePins(indexWithout("none"));
    expect(missing).toEqual([]);
    expect(resolved.map(r => r.id)).toEqual(PRODUCT_SHOT_PINS.map(p => `story-${p.name}`));
  });

  it("exits 1 naming a missing pinned story and keeps the existing captures", () => {
    const storybook = temp("sb");
    writeFileSync(join(storybook, "index.json"), JSON.stringify(indexWithout("board")));
    const out = temp("out");
    writeFileSync(join(out, "board-dark.webp"), "previous capture");

    const run = spawnSync("bun", [SCRIPT, "--storybook-dir", storybook, "--out", out], {
      encoding: "utf8",
    });

    expect(run.status).toBe(1);
    expect(run.stderr).toContain(
      "board: web/src/systems/session/components/stories/sessions-board.stories.tsx → Populated"
    );
    expect(readdirSync(out)).toEqual(["board-dark.webp"]);
    expect(readFileSync(join(out, "board-dark.webp"), "utf8")).toBe("previous capture");
  });
});
