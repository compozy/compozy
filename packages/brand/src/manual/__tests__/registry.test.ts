import { readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { EDITION, pages, VERSION } from "../chapters";

const ROUTES_DIR = fileURLToPath(new URL("../../routes", import.meta.url));
/** route files that are not chapters: the document shell and the static not-found page */
// Export surfaces from _dx.md Routes are not chapters.
const NON_CHAPTER_ROUTES = new Set(["__root", "404", "visual.$id"]);

const routeFileOf = (slug: string) => (slug === "" ? "index" : slug);

describe("chapter registry", () => {
  it("lists the 12 v1.2.0 chapters with unique slugs and numbers", () => {
    expect(VERSION).toBe("v1.2.0");
    expect(EDITION).toBe("November 2026");
    expect(pages).toHaveLength(12);
    expect(new Set(pages.map(p => p.slug)).size).toBe(12);
    expect(pages.map(p => p.n)).toEqual(
      Array.from({ length: 12 }, (_, i) => String(i + 1).padStart(2, "0"))
    );
  });

  it("has exactly one route file per chapter and no orphan route", () => {
    const routes = readdirSync(ROUTES_DIR)
      .filter(f => f.endsWith(".tsx"))
      .map(f => f.replace(/\.tsx$/, ""))
      .filter(r => !NON_CHAPTER_ROUTES.has(r))
      .sort();
    expect(routes).toEqual(pages.map(p => routeFileOf(p.slug)).sort());
  });

  it("approves only Foundations and Library; every other chapter is a proposal", () => {
    const approved = pages.filter(p => p.status === "approved").map(p => p.slug);
    expect(approved).toEqual(["", "library"]);
    expect(pages.filter(p => p.status === "proposal")).toHaveLength(10);
  });

  it("gives every chapter a non-empty, duplicate-free section index", () => {
    for (const p of pages) {
      const ids = p.sections.map(([id]) => id);
      expect(ids.length, p.title).toBeGreaterThan(0);
      expect(new Set(ids).size, p.title).toBe(ids.length);
    }
  });
});
