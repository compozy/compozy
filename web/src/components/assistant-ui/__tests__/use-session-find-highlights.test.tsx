import { render } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  FIND_ACTIVE_HIGHLIGHT,
  FIND_MATCH_HIGHLIGHT,
  useSessionFindHighlights,
} from "../hooks/use-session-find-highlights";
import { locateFindTarget } from "../session-find-ranges";

// Suite: in-transcript find marks (task_08, S8).
// Invariant: the document-global highlight registry holds the union of every mounted
// viewport's ranges under the two shared names, so one conversation closing never erases
// another's marks; ranges are cut in the original text with the daemon's per-code-point folding
// (İ, final sigma, astral characters keep their UTF-16 offsets); the active mark is the first
// occurrence in the active row; a jump target is located inside the matched part.
// Owning layer: assistant-ui highlight hook + find ranges. Canonical suite: this file.
// Boundary IN: rendered rows + query. Boundary OUT: `CSS.highlights` (stubbed) and DOM ranges.

class HighlightStub {
  readonly ranges: Range[];
  constructor(...ranges: Range[]) {
    this.ranges = ranges;
  }
}

function registry(): Map<string, HighlightStub> {
  return (CSS as unknown as { highlights: Map<string, HighlightStub> }).highlights;
}

function rangesOf(name: string): string[] {
  return (registry().get(name)?.ranges ?? []).map(range => range.toString());
}

function Host({
  query,
  activeMessageId,
  rows,
}: {
  query: string;
  activeMessageId: string | null;
  rows: { id: string; text: string }[];
}) {
  const contentRef = useRef<HTMLDivElement | null>(null);
  useSessionFindHighlights(contentRef, query, activeMessageId);
  return (
    <div ref={contentRef}>
      {rows.map(row => (
        <div key={row.id} data-message-id={row.id}>
          {row.text}
        </div>
      ))}
    </div>
  );
}

describe("useSessionFindHighlights", () => {
  beforeEach(() => {
    vi.stubGlobal("Highlight", HighlightStub);
    vi.stubGlobal("CSS", { highlights: new Map<string, HighlightStub>() });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should keep the other conversation's marks when one viewport closes", () => {
    const first = render(
      <Host
        query="lifecycle"
        activeMessageId="a-1"
        rows={[{ id: "a-1", text: "the lifecycle tests" }]}
      />
    );
    const second = render(
      <Host
        query="lifecycle"
        activeMessageId={null}
        rows={[{ id: "b-1", text: "a Lifecycle channel and the lifecycle tail" }]}
      />
    );
    expect(rangesOf(FIND_MATCH_HIGHLIGHT)).toEqual(["lifecycle", "Lifecycle", "lifecycle"]);
    expect(rangesOf(FIND_ACTIVE_HIGHLIGHT)).toEqual(["lifecycle"]);

    first.unmount();
    expect(rangesOf(FIND_MATCH_HIGHLIGHT)).toEqual(["Lifecycle", "lifecycle"]);
    expect(registry().has(FIND_ACTIVE_HIGHLIGHT)).toBe(false);

    second.unmount();
    expect(registry().has(FIND_MATCH_HIGHLIGHT)).toBe(false);
  });

  it("Should cut ranges in the original text under per-code-point folding", () => {
    render(
      <Host
        query="LIFECYCLE"
        activeMessageId="u-1"
        rows={[
          { id: "u-1", text: "İstanbul lifecycle" },
          { id: "u-2", text: "ΟΔΥΣΣΕΥΣ 🚀 lifecycle" },
        ]}
      />
    );
    expect(rangesOf(FIND_MATCH_HIGHLIGHT)).toEqual(["lifecycle", "lifecycle"]);
    expect(rangesOf(FIND_ACTIVE_HIGHLIGHT)).toEqual(["lifecycle"]);
  });

  it("Should drop the marks with the query and locate a jump inside the matched part", () => {
    const view = render(
      <Host query="lifecycle" activeMessageId={null} rows={[{ id: "a-1", text: "lifecycle" }]} />
    );
    expect(registry().has(FIND_MATCH_HIGHLIGHT)).toBe(true);
    view.rerender(
      <Host query="" activeMessageId={null} rows={[{ id: "a-1", text: "lifecycle" }]} />
    );
    expect(registry().has(FIND_MATCH_HIGHLIGHT)).toBe(false);

    const row = document.createElement("div");
    row.innerHTML =
      '<p>lifecycle in prose</p><div data-part-index="1"><span>tool</span> Lifecycle output</div>';
    expect(locateFindTarget(row, 1, "lifecycle")?.toString()).toBe("Lifecycle");
    expect(locateFindTarget(row, 0, "lifecycle")?.toString()).toBe("lifecycle");
    expect(locateFindTarget(row, null, "lifecycle")?.toString()).toBe("lifecycle");
    expect(locateFindTarget(row, 1, "missing")?.toString()).toBe("tool Lifecycle output");
    expect(locateFindTarget(row, null, "missing")).toBeNull();
  });
});
