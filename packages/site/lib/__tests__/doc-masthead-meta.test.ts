import type { Root } from "fumadocs-core/page-tree";
import { describe, expect, it } from "vitest";
import {
  buildMastheadCrumbs,
  resolveAudience,
  resolveDocMastheadMeta,
  resolveProductLabel,
  sectionPageCount,
} from "../doc-masthead-meta";

const loopsTree: Root = {
  name: "Runtime",
  children: [
    {
      type: "folder",
      $id: "core",
      name: "Core Concepts",
      root: true,
      children: [
        {
          type: "folder",
          $id: "core/loops",
          name: "Loops",
          index: {
            type: "page",
            $id: "core/loops/index.mdx",
            name: "Loops",
            url: "/runtime/core/loops",
          },
          children: [
            {
              type: "page",
              $id: "core/loops/catalog.mdx",
              name: "Catalog",
              url: "/runtime/core/loops/catalog",
            },
            {
              type: "page",
              $id: "core/loops/running.mdx",
              name: "Running",
              url: "/runtime/core/loops/running",
            },
          ],
        },
      ],
    },
  ],
};

describe("doc-masthead-meta", () => {
  it("resolves runtime product and audience labels", () => {
    expect(resolveProductLabel("runtime", ["core", "loops"])).toBe("CompozyOS Runtime");
    expect(resolveAudience("runtime")).toBe("people running agent work");
  });

  it("resolves protocol product labels from the family slug", () => {
    expect(resolveProductLabel("protocol", ["specification"])).toBe("Compozy Network Protocol");
    expect(resolveProductLabel("protocol", ["guides"])).toBe("Compozy Network");
    expect(resolveAudience("protocol")).toBe("protocol implementers");
  });

  it("counts navigable pages in the immediate parent folder including the index", () => {
    expect(sectionPageCount(loopsTree, "/runtime/core/loops")).toBe(3);
    expect(sectionPageCount(loopsTree, "/runtime/core/loops/catalog")).toBe(3);
  });

  it("builds masthead crumbs with a non-linked leaf matching the page title", () => {
    const meta = resolveDocMastheadMeta(
      "runtime",
      ["core", "loops"],
      loopsTree,
      "/runtime/core/loops",
      "Loops"
    );

    expect(meta.product).toBe("CompozyOS Runtime");
    expect(meta.sectionPageCount).toBe(3);
    expect(meta.crumbs.at(-1)).toEqual({ name: "Loops" });
    expect(buildMastheadCrumbs(loopsTree, "/runtime/core/loops/catalog", "Catalog")).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: "Loops", href: "/runtime/core/loops" }),
        { name: "Catalog" },
      ])
    );
  });
});
