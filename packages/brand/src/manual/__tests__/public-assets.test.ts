import { existsSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { atmosphereSrcSet, HI_TIER } from "../art/visuals";

// Asset policy (ADR-003, US-009): only web-size files ship in git; masters and exports live in
// ~/Dev/compozy/_rebranding.
const PUBLIC_DIR = fileURLToPath(new URL("../../../public", import.meta.url));
const MAX_TOTAL = 15 * 1024 * 1024;
const MAX_FILE = 2 * 1024 * 1024;

function filesUnder(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(e =>
    e.isDirectory() ? filesUnder(join(dir, e.name)) : [join(dir, e.name)]
  );
}

describe("public asset budget", () => {
  const files = filesUnder(PUBLIC_DIR).map(f => ({
    path: relative(PUBLIC_DIR, f),
    size: statSync(f).size,
  }));

  it("keeps public/ at or under 15 MB", () => {
    const total = files.reduce((sum, f) => sum + f.size, 0);
    expect(total).toBeLessThanOrEqual(MAX_TOTAL);
  });

  it("has no file over 2 MB", () => {
    expect(files.filter(f => f.size > MAX_FILE).map(f => f.path)).toEqual([]);
  });

  it("lists a hi-res visual only when its file is published", () => {
    for (const file of HI_TIER) {
      expect(existsSync(join(PUBLIC_DIR, "brand/visuals/hi", `${file}.jpg`)), file).toBe(true);
    }
  });
});

describe("Atmosphere srcset", () => {
  it("serves 1280 and 2560 and omits the hi tier when it is not published", () => {
    expect(atmosphereSrcSet("swarm", "land", new Set())).toBe(
      "/brand/visuals/swarm-1280.jpg 1280w, /brand/visuals/swarm.jpg 2560w"
    );
  });

  it("adds the hi tier when the file is published", () => {
    expect(atmosphereSrcSet("swarm-portrait", "port", new Set(["swarm-portrait"]))).toBe(
      "/brand/visuals/swarm-portrait-1280.jpg 1024w, /brand/visuals/swarm-portrait.jpg 2049w, /brand/visuals/hi/swarm-portrait.jpg 4488w"
    );
  });
});
