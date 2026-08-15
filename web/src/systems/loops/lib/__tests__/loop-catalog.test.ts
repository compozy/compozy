import { describe, expect, it } from "vitest";

import { loopCatalogFixtures } from "../../mocks/fixtures";
import type { LoopCatalogEntry } from "../../types";
import {
  groupLoopCatalog,
  hasActiveLoopFilters,
  hasHumanGate,
  isUnboundedCap,
  iterationCapLabel,
  type LoopCatalogFilter,
  loopCategory,
  loopInputCount,
  loopKind,
  loopKindFacetCount,
  loopSourceLabel,
  matchesLoopFilter,
  successRateLabel,
} from "../loop-catalog";
import { loopFactsSegments, loopLastRunFact } from "../loop-catalog-presentation";

const [delivery, review] = loopCatalogFixtures;

describe("loop-catalog", () => {
  it("Should classify read-only vs workspace loops by source", () => {
    expect(loopKind({ source: "marketplace" })).toBe("read-only");
    expect(loopKind({ source: "workspace" })).toBe("workspace");
    expect(loopKind(delivery)).toBe("read-only");
    expect(loopKind(review)).toBe("read-only");
  });

  it("Should read only the category the daemon sent (no invented taxonomy)", () => {
    expect(loopCategory(delivery)).toBe("Engineering");
    expect(loopCategory(review)).toBe("Engineering");
    const blank: LoopCatalogEntry = {
      ...delivery,
      catalog: { ...delivery.catalog, category: "  " },
    };
    expect(loopCategory(blank)).toBeNull();
  });

  it("Should count declared inputs and distinguish bounded review loops", () => {
    expect(loopInputCount(delivery)).toBe(3);
    expect(loopInputCount(review)).toBe(4);
    expect(isUnboundedCap(review)).toBe(false);
    expect(isUnboundedCap(delivery)).toBe(false);
  });

  it("Should render iteration cap 0 as the unbounded glyph", () => {
    expect(iterationCapLabel(0)).toBe("∞");
    expect(iterationCapLabel(50)).toBe("50");
  });

  it("Should format a 0..1 success rate as a whole percent", () => {
    expect(successRateLabel(0.9)).toBe("90%");
    expect(successRateLabel(1)).toBe("100%");
    expect(successRateLabel(Number.NaN)).toBe("—");
  });

  it("Should detect a human gate from verification criteria", () => {
    expect(hasHumanGate(delivery)).toBe(false);
    const withHuman: LoopCatalogEntry = {
      ...delivery,
      contract: {
        ...delivery.contract,
        verification: [{ id: "approve", type: "human" }],
      },
    };
    expect(hasHumanGate(withHuman)).toBe(true);
  });

  it("Should filter by kind, category, and last-run status", () => {
    expect(matchesLoopFilter(delivery, { kind: "workspace", category: null, status: null })).toBe(
      false
    );
    expect(matchesLoopFilter(delivery, { kind: "read-only", category: null, status: null })).toBe(
      true
    );
    expect(
      matchesLoopFilter(delivery, { kind: "all", category: "Engineering", status: null })
    ).toBe(true);
    expect(matchesLoopFilter(delivery, { kind: "all", category: "watch", status: null })).toBe(
      false
    );
    expect(
      matchesLoopFilter(delivery, {
        kind: "all",
        category: null,
        status: delivery.last_run!.status,
      })
    ).toBe(true);
    expect(matchesLoopFilter(delivery, { kind: "all", category: null, status: "failed" })).toBe(
      false
    );
  });

  it("Should label provenance as Built-in or Custom without changing the source values", () => {
    expect(loopSourceLabel(delivery)).toBe("Built-in");
    expect(loopSourceLabel(review)).toBe("Built-in");
    expect(loopSourceLabel({ source: "marketplace" })).toBe("Built-in");
    expect(loopSourceLabel({ source: "user" })).toBe("Built-in");
    expect(loopSourceLabel({ source: "additional" })).toBe("Built-in");
    expect(loopKind(delivery)).toBe("read-only");
    expect(loopKind(review)).toBe("read-only");
  });

  it("Should report active filters for a query or any chip, ignoring whitespace", () => {
    const cleared: LoopCatalogFilter = { kind: "all", category: null, status: null };
    expect(hasActiveLoopFilters("", cleared)).toBe(false);
    expect(hasActiveLoopFilters("   ", cleared)).toBe(false);
    expect(hasActiveLoopFilters("watch", cleared)).toBe(true);
    expect(hasActiveLoopFilters("", { kind: "workspace", category: null, status: null })).toBe(
      true
    );
    expect(hasActiveLoopFilters("", { kind: "all", category: "delivery", status: null })).toBe(
      true
    );
    expect(hasActiveLoopFilters("", { kind: "all", category: null, status: "running" })).toBe(true);
  });

  it("Should group into Built-in/Custom and drop empty groups", () => {
    const all = groupLoopCatalog(loopCatalogFixtures, {
      kind: "all",
      category: null,
      status: null,
    });
    expect(all.map(group => group.kind)).toEqual(["read-only"]);
    expect(all.map(group => group.label)).toEqual(["Built-in"]);
    expect(all[0].entries).toHaveLength(2);

    const readOnlyOnly = groupLoopCatalog(loopCatalogFixtures, {
      kind: "read-only",
      category: null,
      status: null,
    });
    expect(readOnlyOnly).toHaveLength(1);
    expect(readOnlyOnly[0].kind).toBe("read-only");

    const none = groupLoopCatalog(loopCatalogFixtures, {
      kind: "workspace",
      category: "Engineering",
      status: null,
    });
    expect(none).toHaveLength(0);
  });

  it("Should prefer server kind facets and fall back to the loaded length", () => {
    expect(loopKindFacetCount("read-only", 2, { read_only: 80 })).toBe(80);
    expect(loopKindFacetCount("workspace", 0, { workspace: 5 })).toBe(5);
    expect(loopKindFacetCount("read-only", 2)).toBe(2);
    expect(loopKindFacetCount("read-only", 2, {})).toBe(2);
  });

  it("Should lead facts with category and append best as a plain segment", () => {
    expect(loopFactsSegments(delivery)).toEqual(["Engineering", "3 inputs", "iteration cap 50"]);
    const withBest: LoopCatalogEntry = {
      ...delivery,
      last_run: { ...delivery.last_run!, best_generation: 2, best_score: 0.92 },
    };
    expect(loopFactsSegments(withBest)).toEqual([
      "Engineering",
      "3 inputs",
      "iteration cap 50",
      "best Gen 2 · 0.92",
    ]);
    expect(loopLastRunFact(delivery)).toEqual({
      id: "looprun_running",
      iso: "2026-07-05T12:00:00Z",
    });
    expect(loopLastRunFact({ ...delivery, last_run: undefined })).toBeNull();
  });
});
