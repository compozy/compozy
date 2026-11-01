// Invariant: the Activity page reads honest totals and never guesses who started the work.
import { describe, expect, it } from "vitest";

import {
  activityResult,
  activityRow,
  activityStarter,
  deriveActivityModel,
} from "../lib/activity-view";
import {
  ACTIVITY_NOW,
  activityEntriesFixture,
  activityRollupFixture,
} from "../mocks/activity-fixtures";

const NOW = new Date(ACTIVITY_NOW);

describe("activity view", () => {
  it("Should show a row from before the upgrade as unknown [UT-049]", () => {
    const legacy = activityEntriesFixture(false).find(row => row.initiator_kind === "");
    expect(activityStarter("")).toBe("unknown");
    expect(activityRow(legacy!, NOW).starter).toBe("unknown");
    expect(activityRow(legacy!, NOW).source).toBeUndefined();
  });

  it("Should classify decisions by what came of them", () => {
    expect(activityResult("allow-always")).toBe("allowed");
    expect(activityResult("reject-once")).toBe("declined");
    expect(activityResult("timeout")).toBe("expired");
  });

  it("Should total the rollups and leave out human work under Nobody watching", () => {
    const rollup = activityRollupFixture(true);
    const model = deriveActivityModel(
      "unattended",
      rollup.rollups,
      activityEntriesFixture(true),
      new Date(rollup.until),
      NOW
    );
    expect(model.total).toBe(5);
    expect(model.allowed).toBe(3);
    expect(model.declined).toBe(1);
    expect(model.expired).toBe(1);
    expect(model.sentence).toBe("5 actions ran with nobody in the conversation.");
    expect(model.starters.map(item => item.key)).not.toContain("human");
    expect(model.days).toHaveLength(14);
    expect(model.pulse.buckets.reduce((sum, cell) => sum + cell.events, 0)).toBe(5);
  });

  it("Should word a refused action as wanted, not done", () => {
    const refused = activityEntriesFixture(true).find(row => row.decision === "reject-once");
    expect(activityRow(refused!, NOW)).toMatchObject({
      action: "Wanted to run a command",
      detail: "make migrate",
      source: "Token rollout",
      result: "declined",
    });
  });
});
