import { describe, expect, it } from "vitest";

import { childRunInputLabels } from "../loop-run-child-inputs";

describe("childRunInputLabels", () => {
  it("Should name only the inputs that differ between siblings", () => {
    const labels = childRunInputLabels([
      { runId: "a", inputs: { batch: "api", mode: "strict", files: 4 } },
      { runId: "b", inputs: { batch: "web", mode: "strict", files: 11 } },
    ]);
    expect(labels.get("a")).toEqual({
      label: "batch: api · files: 4",
      title: "batch: api\nfiles: 4",
    });
    expect(labels.get("b")?.label).toBe("batch: web · files: 11");
  });

  it("Should name a lone child's inputs, since it has no sibling to differ from", () => {
    expect(childRunInputLabels([{ runId: "a", inputs: { wave: 2 } }]).get("a")?.label).toBe(
      "wave: 2"
    );
  });

  it("Should shorten long and structured values but keep them whole in the title", () => {
    const file = "internal/billing/webhooks/handlers/stripe_events.go";
    const labels = childRunInputLabels([
      { runId: "a", inputs: { file, tags: ["a", "b"] } },
      { runId: "b", inputs: { file: "x.go", tags: ["c"] } },
    ]);
    expect(labels.get("a")?.label).toBe('file: internal/billing/webhooks/h… · tags: ["a","b"]');
    expect(labels.get("a")?.title).toContain(file);
  });

  it("Should label nothing for identical siblings or children not read yet", () => {
    const labels = childRunInputLabels([
      { runId: "a", inputs: { batch: "api" } },
      { runId: "b", inputs: { batch: "api" } },
      { runId: "c", inputs: null },
    ]);
    expect(labels.size).toBe(0);
  });
});
