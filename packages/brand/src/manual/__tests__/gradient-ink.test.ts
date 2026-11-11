import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

// Text on the brand or Live gradient takes the fixed dark gradient ink, never white: the
// gradient's glint end drops white to ~1.1:1. Every class list that paints a gradient fill and
// sets a text colour must use `text-on-gradient` (the token pair is held in @compozy/ui's
// theme-token contract).
const MANUAL_DIR = fileURLToPath(new URL("..", import.meta.url));
const GRADIENT_FILL =
  /\[background:var\(--gradient-(?:brand|live)\)\]|bg-\(image:--gradient-(?:brand|live)\)/;
const NON_COLOR_TEXT =
  /^text-(?:\[\d|\[clamp|xs|sm|base|lg|xl|\dxl|left|right|center|justify|start|end|wrap|nowrap|balance|pretty|ellipsis|clip|display)/;

function sourcesUnder(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(e => {
    if (e.name === "__tests__") return [];
    const path = join(dir, e.name);
    if (e.isDirectory()) return sourcesUnder(path);
    return /\.tsx?$/.test(e.name) ? [path] : [];
  });
}

function gradientTextClasses() {
  return sourcesUnder(MANUAL_DIR).flatMap(file => {
    const source = readFileSync(file, "utf8");
    return Array.from(source.matchAll(/"([^"\n]*)"/g), m => m[1])
      .filter(classes => GRADIENT_FILL.test(classes))
      .flatMap(classes =>
        classes
          .split(/\s+/)
          .filter(c => c.startsWith("text-") && !NON_COLOR_TEXT.test(c))
          .map(color => ({ file: relative(MANUAL_DIR, file), color }))
      );
  });
}

describe("text on the brand gradients", () => {
  it("uses the on-gradient ink wherever a gradient fill sets a text colour", () => {
    const pairs = gradientTextClasses();
    expect(pairs.length).toBeGreaterThan(0);
    expect(pairs.filter(p => p.color !== "text-on-gradient")).toEqual([]);
  });
});
