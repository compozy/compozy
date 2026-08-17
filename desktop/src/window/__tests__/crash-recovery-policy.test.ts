import { describe, expect, it } from "vitest";

import { CrashRecoveryBudget } from "../crash-recovery-policy";

// Invariant: three renderer failures in a rolling minute recover automatically; the next one needs a person.
describe("crash recovery policy", () => {
  it("Should allow three reloads and then require a dialog", () => {
    const budget = new CrashRecoveryBudget();
    expect([0, 1, 2].map(second => budget.record(second * 1_000))).toEqual([
      "reload",
      "reload",
      "reload",
    ]);
    expect(budget.record(3_000)).toBe("dialog");
  });

  it("Should forget crashes outside the rolling minute", () => {
    const budget = new CrashRecoveryBudget();
    budget.record(0);
    budget.record(1_000);
    budget.record(2_000);
    expect(budget.record(60_000)).toBe("reload");
  });
});
