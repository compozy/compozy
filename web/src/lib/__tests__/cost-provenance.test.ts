import { describe, expect, it } from "vitest";

import { describeCost, type CostSource } from "../cost-provenance";

function currency(amount: number, code: string, digits: number): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: code,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(amount);
}

describe("describeCost — monetary presentation contract", () => {
  it("Should render actual cost as a plain measured amount with no estimate glyph", () => {
    const display = describeCost({ status: "actual", source: "agent_reported", amount: 18.42 });
    expect(display.value).toBe(currency(18.42, "USD", 2));
    expect(display.isAmount).toBe(true);
    expect(display.isEstimated).toBe(false);
    expect(display.value).not.toContain("≈");
    expect(display.sourceLabel).toBe("Reported by agent");
    expect(display.note).toBe("Reported by agent");
    expect(display.hasCost).toBe(true);
  });

  it("Should mark estimated cost with the ≈ glyph so it never reads as measured spend", () => {
    const display = describeCost({ status: "estimated", source: "catalog_config", amount: 0.18 });
    expect(display.isEstimated).toBe(true);
    expect(display.isAmount).toBe(true);
    expect(display.value.startsWith("≈")).toBe(true);
    expect(display.value).toContain(currency(0.18, "USD", 3));
    expect(display.sourceLabel).toBe("Catalog rate");
    expect(display.note).toBe("Estimated · Catalog rate");
    // Contract: an estimate is never presented as the bare actual amount.
    expect(display.value).not.toBe(describeCost({ status: "actual", amount: 0.18 }).value);
  });

  it("Should render included usage with no monetary amount", () => {
    const display = describeCost({ status: "included", source: "none", amount: null });
    expect(display.value).toBe("Included");
    expect(display.isAmount).toBe(false);
    expect(display.isEstimated).toBe(false);
    expect(display.sourceLabel).toBeNull();
    expect(display.note).toBe("Subscription");
    expect(display.hasCost).toBe(true);
    expect(display.value).not.toMatch(/[$€£¥\d]/);
  });

  it("Should render unknown cost with no monetary amount", () => {
    const display = describeCost({ status: "unknown", amount: null });
    expect(display.value).toBe("Unavailable");
    expect(display.isAmount).toBe(false);
    expect(display.note).toBe("Cost unavailable");
    expect(display.hasCost).toBe(true);
    expect(display.value).not.toMatch(/[$€£¥\d]/);
  });

  it("Should keep included/unknown free of a source label even when the daemon sends one", () => {
    expect(describeCost({ status: "included", source: "catalog_config" }).sourceLabel).toBeNull();
    expect(describeCost({ status: "unknown", source: "builtin" }).sourceLabel).toBeNull();
  });

  it("Should map each cost_source to a provenance label and collapse none to null", () => {
    const labels: Record<CostSource, string | null> = {
      agent_reported: "Reported by agent",
      catalog_config: "Catalog rate",
      models_dev: "models.dev rate",
      builtin: "Built-in rate",
      none: null,
    };
    for (const [source, label] of Object.entries(labels) as [CostSource, string | null][]) {
      expect(describeCost({ status: "actual", source, amount: 1 }).sourceLabel).toBe(label);
    }
    expect(describeCost({ status: "actual", amount: 1 }).sourceLabel).toBeNull();
  });

  it("Should format sub-$1 amounts with three fraction digits and larger amounts with two", () => {
    expect(describeCost({ status: "actual", amount: 0.184 }).value).toBe(currency(0.184, "USD", 3));
    expect(describeCost({ status: "actual", amount: 42.5 }).value).toBe(currency(42.5, "USD", 2));
  });

  it("Should honor a non-USD currency code and fall back to USD for a missing/invalid code", () => {
    expect(describeCost({ status: "actual", amount: 2.5, currency: "EUR" }).value).toBe(
      currency(2.5, "EUR", 2)
    );
    expect(describeCost({ status: "actual", amount: 2.5, currency: "??" }).value).toBe(
      currency(2.5, "USD", 2)
    );
  });

  it("Should render no cost when an amount arrives without a cost status", () => {
    const display = describeCost({ amount: 18.42, currency: "USD" });
    expect(display.value).toBe("—");
    expect(display.isAmount).toBe(false);
    expect(display.isEstimated).toBe(false);
    expect(display.hasCost).toBe(false);
  });

  it("Should report no cost to render when neither status nor a finite amount is present", () => {
    const display = describeCost({ amount: null });
    expect(display.value).toBe("—");
    expect(display.isAmount).toBe(false);
    expect(display.note).toBeNull();
    expect(display.hasCost).toBe(false);
  });

  it("Should degrade an estimate with no amount to a truthful placeholder", () => {
    const display = describeCost({ status: "estimated", source: "catalog_config", amount: null });
    expect(display.value).toBe("—");
    expect(display.isAmount).toBe(false);
    expect(display.isEstimated).toBe(false);
    expect(display.hasCost).toBe(true);
    expect(display.sourceLabel).toBe("Catalog rate");
  });
});
