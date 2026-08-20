// Suite: palette domain filter projection
// Invariant: task statuses collapse onto the chip ids, unknown statuses keep their slug, and empty filters stay labeled.
// Boundary IN: domainChips / domainFilter / domainEmptyMessage.
// Boundary OUT: OsPaletteDomainChips rendering.
import { describe, expect, it } from "vitest";

import { domainChips, domainEmptyMessage, domainFilter } from "../os-palette-domain-filters";
import type { OsPaletteDomainRow } from "../use-os-palette-domain-search";

function row(status?: string): OsPaletteDomainRow {
  return {
    key: `task:${status ?? "none"}`,
    label: status ?? "plain",
    app: "tasks",
    route: { pathname: "/tasks", search: {} },
    ...(status === undefined ? {} : { status }),
  };
}

describe("palette domain filter projection", () => {
  it("Should map task statuses onto the chip order and keep unknown slugs [RA0301]", () => {
    expect(domainFilter(row("ready"))).toBe("queued");
    expect(domainFilter(row("in_progress"))).toBe("running");
    expect(domainFilter(row("needs_attention"))).toBe("needs-approval");
    expect(domainFilter(row("completed"))).toBe("done");
    expect(domainFilter(row("weird-state"))).toBe("weird-state");
    expect(domainFilter(row())).toBe("all");

    const chips = domainChips("Tasks", [row("ready"), row("failed"), row()]);
    expect(chips.map(chip => chip.id)).toEqual([
      "all",
      "queued",
      "running",
      "needs-approval",
      "done",
      "failed",
    ]);
    expect(chips[0]?.count).toBe(3);
    expect(chips.find(chip => chip.id === "queued")?.count).toBe(1);
    expect(chips.find(chip => chip.id === "failed")?.count).toBe(1);
  });

  it("Should name empty states from loading, error, query, and filter [RA0301]", () => {
    expect(domainEmptyMessage("Tasks", "", "all", true, null)).toBe("Loading tasks…");
    expect(domainEmptyMessage("Tasks", "", "all", false, "Tasks unavailable")).toBe(
      "Tasks unavailable"
    );
    expect(domainEmptyMessage("Tasks", "auth", "all", false, null)).toBe("No tasks match “auth”.");
    expect(domainEmptyMessage("Tasks", "", "queued", false, null)).toBe("No tasks are queued.");
    expect(domainEmptyMessage("Tasks", "", "all", false, null)).toBe("No tasks yet.");
  });
});
