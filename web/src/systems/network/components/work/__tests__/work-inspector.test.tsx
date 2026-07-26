// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { WorkInspector } from "../work-inspector";
import type { OpenWorkEntry } from "../../../hooks/use-work";

const sampleEntries: OpenWorkEntry[] = [
  {
    workId: "work-1",
    state: "working",
    messageId: "msg-100",
    targetPeerId: "peer-remote",
    openedAt: "2026-04-17T18:00:00Z",
    lastActivityAt: "2026-04-17T18:01:00Z",
  },
  {
    workId: "work-2",
    state: "needs_input",
    messageId: "msg-101",
    targetPeerId: null,
    openedAt: "2026-04-17T17:55:00Z",
    lastActivityAt: "2026-04-17T18:00:00Z",
  },
];

describe("WorkInspector", () => {
  it("Should render the open count and one row per entry", () => {
    render(<WorkInspector entries={sampleEntries} />);
    expect(screen.getByTestId("network-work-inspector-count")).toHaveTextContent("2 open");
    expect(screen.getByTestId("network-work-inspector-row-work-1")).toBeInTheDocument();
    expect(screen.getByTestId("network-work-inspector-row-work-2")).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(screen.getAllByRole("listitem").every(item => item.tagName === "LI")).toBe(true);
  });

  it("Should render an empty placeholder when no entries are open", () => {
    render(<WorkInspector entries={[]} />);
    expect(screen.getByText("No work in flight.")).toBeInTheDocument();
  });

  it("Should call onJump with the entry when the jump button is clicked", async () => {
    const onJump = vi.fn();
    const user = userEvent.setup();
    render(<WorkInspector entries={sampleEntries} onJump={onJump} />);
    await user.click(screen.getByTestId("network-work-inspector-jump-work-1"));
    expect(onJump).toHaveBeenCalledTimes(1);
    expect(onJump.mock.calls[0]?.[0]).toMatchObject({ workId: "work-1" });
  });
});
