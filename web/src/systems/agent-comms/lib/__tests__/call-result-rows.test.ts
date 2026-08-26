// Invariant: result preview projection never emits more than the fixed row cap,
// including summary rows added after nested array traversal.
// Owning layer: call-result-rows. Canonical suite: this file.
import { describe, expect, it } from "vitest";

import { buildCallResultShape } from "../call-result-rows";

describe("buildCallResultShape", () => {
  it("Should keep array summary rows inside the global row cap", () => {
    const preview = Object.fromEntries(
      Array.from({ length: 60 }, (_, index) => [
        `field_${index}`,
        index === 59 ? ["one", "two", "three", "four"] : index,
      ])
    );

    const shape = buildCallResultShape(preview);

    expect(shape.kind).toBe("rows");
    if (shape.kind !== "rows") return;
    expect(shape.rows).toHaveLength(60);
    expect(shape.truncated).toBe(true);
  });
});
