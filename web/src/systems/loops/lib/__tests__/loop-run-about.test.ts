import { describe, expect, it } from "vitest";

import { loopRunDetailByRunId } from "../../mocks/fixtures";
import type { LoopDefinition, LoopRunRecord } from "../../types";
import { buildInputRows, humanizeInputKey, humanizeStartOrigin } from "../loop-run-about";

function run(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return { ...loopRunDetailByRunId.get("looprun_review_running")!.run, ...overrides };
}

function definitionInputs(
  inputs: Record<string, { required?: boolean; type?: "agent" | "string" }>
): Pick<LoopDefinition, "inputs"> {
  return {
    inputs: Object.fromEntries(
      Object.entries(inputs).map(([key, value]) => [key, { type: "string", ...value }])
    ),
  } as Pick<LoopDefinition, "inputs">;
}

describe("buildInputRows", () => {
  it("Should flag agent-bound inputs from the closed input type", () => {
    const rows = buildInputRows(
      run({ inputs: { fixer: "review-fixer", pr: "128" } }),
      definitionInputs({ fixer: { type: "agent" }, pr: {} })
    );
    expect(rows).toEqual([
      { key: "fixer", label: "Fixer", value: "review-fixer", isAgent: true },
      { key: "pr", label: "PR", value: "128", isAgent: false },
    ]);
  });
});

describe("humanizeInputKey", () => {
  it("Should read short keys as initialisms and spaced keys as sentences", () => {
    expect(humanizeInputKey("pr")).toBe("PR");
    expect(humanizeInputKey("max_files")).toBe("Max files");
  });
});

describe("humanizeStartOrigin", () => {
  it("Should map known origin kinds and append the mono ref", () => {
    expect(humanizeStartOrigin(run({ started_origin_kind: "webhook" }))).toBe("A webhook");
    expect(
      humanizeStartOrigin(run({ started_origin_kind: "schedule", started_origin_ref: "nightly" }))
    ).toBe("A schedule · nightly");
    expect(humanizeStartOrigin(run({ started_origin_kind: "" }))).toBe("hand");
  });
});
