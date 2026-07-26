import { describe, expect, it } from "vitest";

import { loopRunDetailByRunId } from "../../mocks/fixtures";
import type { LoopDefinition, LoopRunRecord } from "../../types";
import {
  buildInputRows,
  humanizeInputKey,
  humanizeStartOrigin,
  watchedSubject,
} from "../loop-run-about";

function run(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return { ...loopRunDetailByRunId.get("looprun_watching")!.run, ...overrides };
}

function definitionInputs(
  inputs: Record<string, { required?: boolean; ref?: { kind: string } }>
): Pick<LoopDefinition, "inputs"> {
  return {
    inputs: Object.fromEntries(
      Object.entries(inputs).map(([key, value]) => [key, { type: "string", ...value }])
    ),
  } as Pick<LoopDefinition, "inputs">;
}

describe("watchedSubject", () => {
  it("Should prefer the required declared scalar and render `key value`", () => {
    const subject = watchedSubject(
      run({ inputs: { branch: "main", pr: "128" } }),
      definitionInputs({ pr: { required: true }, branch: {} })
    );
    expect(subject).toBe("PR 128");
  });

  it("Should fall back to the first scalar input and skip non-scalars", () => {
    const subject = watchedSubject(
      run({ inputs: { payload: { nested: true }, slug: "loops-catalog-api" } }),
      undefined
    );
    expect(subject).toBe("Slug loops-catalog-api");
  });

  it("Should return null when the run has no scalar inputs", () => {
    expect(watchedSubject(run({ inputs: {} }), undefined)).toBeNull();
  });
});

describe("buildInputRows", () => {
  it("Should flag agent-bound inputs from the declared ref kind", () => {
    const rows = buildInputRows(
      run({ inputs: { fixer: "review-fixer", pr: "128" } }),
      definitionInputs({ fixer: { ref: { kind: "agent" } }, pr: {} })
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
    expect(humanizeStartOrigin(run({ started_origin_kind: "" }))).toBe("Started by hand");
  });
});
