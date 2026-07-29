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
  name: "Docs",
  children: [
    {
      type: "folder",
      $id: "loops",
      name: "Loops",
      index: {
        type: "page",
        $id: "loops/index.mdx",
        name: "Loops",
        url: "/docs/loops",
      },
      children: [
        {
          type: "page",
          $id: "loops/catalog.mdx",
          name: "Catalog",
          url: "/docs/loops/catalog",
        },
        {
          type: "page",
          $id: "loops/running.mdx",
          name: "Running",
          url: "/docs/loops/running",
        },
      ],
    },
  ],
};

describe("doc-masthead-meta", () => {
  it("resolves the CompozyOS product and audience labels for docs pages", () => {
    expect(resolveProductLabel(["loops"])).toBe("CompozyOS");
    expect(resolveAudience(["loops"])).toBe("people running agent work");
  });

  it("resolves protocol product labels for pages nested under network/protocol", () => {
    expect(resolveProductLabel(["network", "protocol", "envelope"])).toBe(
      "Compozy Network Protocol"
    );
    expect(resolveProductLabel(["network", "threads"])).toBe("CompozyOS");
    expect(resolveAudience(["network", "protocol"])).toBe("protocol implementers");
    expect(resolveAudience(["network"])).toBe("people running agent work");
  });

  it("counts navigable pages in the immediate parent folder including the index", () => {
    expect(sectionPageCount(loopsTree, "/docs/loops")).toBe(3);
    expect(sectionPageCount(loopsTree, "/docs/loops/catalog")).toBe(3);
  });

  it("builds masthead crumbs with a non-linked leaf matching the page title", () => {
    const meta = resolveDocMastheadMeta(["loops"], loopsTree, "/docs/loops", "Loops");

    expect(meta.product).toBe("CompozyOS");
    expect(meta.sectionPageCount).toBe(3);
    expect(meta.crumbs.at(-1)).toEqual({ name: "Loops" });
    expect(buildMastheadCrumbs(loopsTree, "/docs/loops/catalog", "Catalog")).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ name: "Loops", href: "/docs/loops" }),
        { name: "Catalog" },
      ])
    );
  });
});
