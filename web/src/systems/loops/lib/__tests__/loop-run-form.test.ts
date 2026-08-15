import { describe, expect, it } from "vitest";

import {
  declaredInputCountsGist,
  environmentGist,
  hasInputValue,
  initialRunInputs,
  isRunFormValid,
  missingRequiredInputs,
  participationGist,
  serializeRunInputs,
} from "../loop-run-form";
import type { LoopInputSchema } from "../../types";

const schema: LoopInputSchema = {
  slug: { type: "string", required: true },
  implementer: { type: "agent", default: "code_implementer" },
  verify_command: { type: "string", default: "" },
  auto_commit: { type: "boolean", default: false },
  max_files: { type: "number", default: 20 },
};

describe("loop-run-form model", () => {
  it("Should seed defaults and give a boolean without a default an off state", () => {
    expect(initialRunInputs(schema)).toEqual({
      implementer: "code_implementer",
      verify_command: "",
      auto_commit: false,
      max_files: 20,
    });
    expect(initialRunInputs(undefined)).toEqual({});
  });

  it("Should treat blank text and NaN numbers as missing values", () => {
    expect(hasInputValue(schema.slug, "")).toBe(false);
    expect(hasInputValue(schema.slug, "  ")).toBe(false);
    expect(hasInputValue(schema.slug, "billing")).toBe(true);
    expect(hasInputValue(schema.max_files, Number.NaN)).toBe(false);
    expect(hasInputValue(schema.max_files, 5)).toBe(true);
    expect(hasInputValue(schema.auto_commit, false)).toBe(true);
  });

  it("Should block submit until every required input is filled", () => {
    const seeded = initialRunInputs(schema);
    expect(missingRequiredInputs(schema, seeded)).toEqual(["slug"]);
    expect(isRunFormValid(schema, seeded)).toBe(false);
    const filled = { ...seeded, slug: "billing-webhooks" };
    expect(missingRequiredInputs(schema, filled)).toEqual([]);
    expect(isRunFormValid(schema, filled)).toBe(true);
  });

  it("Should serialize defined values and keep an empty string only for a default-empty field", () => {
    const body = serializeRunInputs(schema, {
      slug: "billing",
      verify_command: "",
      auto_commit: false,
      note: undefined,
      blank_free_text: "",
    });
    expect(body).toEqual({ slug: "billing", verify_command: "", auto_commit: false });
    expect(body).not.toHaveProperty("blank_free_text");
    expect(body).not.toHaveProperty("note");
  });

  it("Should summarize declared input counts for the Inputs gist", () => {
    expect(declaredInputCountsGist(schema)).toBe("1 required · 4 optional");
    expect(declaredInputCountsGist(undefined)).toBe("0 required · 0 optional");
  });

  it("Should summarize participation and environment drafts for folded gists", () => {
    expect(participationGist({ mode: "local", channelId: "", channelStrategy: "" })).toBe("Local");
    expect(
      participationGist({ mode: "live", channelId: "release", channelStrategy: "loop_run" })
    ).toBe("Live · loop_run");
    expect(environmentGist(null)).toBe("Loop default");
    expect(environmentGist({ mode: "worktree", worktree_ref: "feat/billing" })).toBe(
      "worktree · feat/billing"
    );
    expect(environmentGist({ mode: "directory", directory: "packages/api" })).toBe(
      "directory · packages/api"
    );
    expect(environmentGist({ mode: "per_run" })).toBe("Per-run");
  });
});
