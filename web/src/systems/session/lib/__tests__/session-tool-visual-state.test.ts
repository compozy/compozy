import { describe, expect, it } from "vitest";

import {
  liveToolLabel,
  parallelToolLabel,
  toolVisualKind,
  toolVisualState,
  toolVisualStatus,
} from "../session-tool-visual-state";

// Suite: tool visual hierarchy mapping (ADR-009, US-026, UT-108).
// Invariant: every tool entry maps to exactly one semantic status and one kind
// with an accessible word; the failure signal exists only for a failure that
// ended the turn — an absorbed error stays in the settled ink as information.
// Palette/CSS fidelity is owned by the artboards and Storybook, not here.
describe("tool visual state", () => {
  it("Should map kinds from the production tool registry, never from color", () => {
    expect(toolVisualKind("Bash")).toBe("command");
    expect(toolVisualKind("Edit")).toBe("edit");
    expect(toolVisualKind("Write")).toBe("edit");
    expect(toolVisualKind("Read /tmp/a.ts")).toBe("read");
    expect(toolVisualKind("Grep")).toBe("search");
    expect(toolVisualKind("WebFetch")).toBe("web");
    expect(toolVisualKind("Task")).toBe("agent");
    expect(toolVisualKind("mcp__linear__list_issues")).toBe("other");
  });

  it("Should reserve the failed status for a failure that ended the turn", () => {
    expect(toolVisualStatus({ status: "running" })).toBe("live");
    expect(toolVisualStatus({ status: "settled" })).toBe("settled");
    expect(toolVisualStatus({ status: "settled", resultEmpty: true })).toBe("empty");
    expect(toolVisualStatus({ status: "interrupted" })).toBe("stopped");
    expect(toolVisualStatus({ status: "settled", isError: true })).toBe("absorbed");
    expect(toolVisualStatus({ status: "settled", resultFailed: true })).toBe("absorbed");
    expect(toolVisualStatus({ status: "settled", isError: true, turnFailed: true })).toBe("failed");
    expect(toolVisualStatus({ status: "settled", turnFailed: true })).toBe("settled");
  });

  it("Should carry every state in words as well", () => {
    expect(toolVisualState("Bash", { status: "settled", isError: true })).toEqual({
      status: "absorbed",
      kind: "command",
      statusLabel: "Failed, turn continued",
    });
    expect(toolVisualState("Read", { status: "interrupted" }).statusLabel).toBe("Stopped");
    expect(toolVisualState("Edit", { status: "running" }).statusLabel).toBe("Running");
  });

  it("Should phrase the live row from the kind and the production preview", () => {
    expect(liveToolLabel("Bash", { command: "go test ./..." })).toEqual({
      verb: "Running shell",
      preview: "go test ./...",
      text: "Running shell — go test ./...",
    });
    expect(liveToolLabel("Edit", { file_path: "internal/store/retry_test.go" }).text).toBe(
      "Editing internal/store/retry_test.go"
    );
    expect(liveToolLabel("Read", { file_path: "a.go" }).text).toBe("Reading a.go");
    expect(liveToolLabel("Grep", { pattern: "time.Sleep" }).text).toBe(
      "Searching content — time.Sleep"
    );
    expect(liveToolLabel("Glob", { pattern: "**/*.go" }).verb).toBe("Finding files");
    expect(liveToolLabel("WebFetch", { url: "https://pkg.go.dev" }).text).toBe(
      "Fetching — https://pkg.go.dev"
    );
    expect(liveToolLabel("Task", { description: "review the diff" }).text).toBe(
      "Running agent — review the diff"
    );
    expect(liveToolLabel("mcp__linear__list_issues").text).toBe("Running mcp__linear__list_issues");
    expect(parallelToolLabel(3)).toBe("Running 3 tools…");
  });
});

// Invariant: provider descriptions and agent prompts cannot become unbounded action headings.
it("Should keep identity separate from long descriptive titles and bound agent previews", () => {
  const title = "Inspect\n" + "ação 👩🏽‍💻 ".repeat(100);
  expect(liveToolLabel("Bash", { command: "ls" }, title)).toMatchObject({ verb: "Running shell" });
  expect(liveToolLabel(title).verb).toBe("Running tool");
  expect(liveToolLabel("Bash dependency investigation")).toMatchObject({
    verb: "Running tool",
    preview: "Bash dependency investigation",
  });
  expect(liveToolLabel("Bash", {}, "Bash dependency investigation").verb).toBe("Running shell");
  expect(liveToolLabel(title).preview).not.toContain("\n");
  const label = liveToolLabel("Agent", { prompt: title });
  expect(label.verb).toBe("Running agent");
  expect(label.preview).toMatch(/…$/);
  expect(label.preview!.length).toBeLessThan(title.length);
});
