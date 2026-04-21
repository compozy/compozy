import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

describe("web shared token wiring", () => {
  it("imports the shared token stylesheet and scans the shared ui source", () => {
    const css = readFileSync(resolve(import.meta.dirname, "./styles.css"), "utf8");

    expect(css).toContain('@import "@compozy/ui/tokens.css";');
    expect(css).toContain('@source "../../packages/ui/src";');
  });
});
