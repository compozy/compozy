import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { primarySessionFixture } from "../../mocks/fixtures";
import type { SessionTranscriptOutlineResponse } from "../../types";
import { SessionMessageTrail, type SessionMessageTrailProps } from "../session-message-trail";

// Suite: SessionMessageTrail (S9, US-029).
// Invariant: one tick per operator message from the daemon's outline (full history), the
// anchor is `aria-current`, hover/focus shows what was asked and the final reply, click and
// Enter jump through the host's scroll owner, the rail is one roving tab stop, and it is
// absent (not squeezed) below two messages.
// Owning layer: session navigation component. Canonical suite: this file.
// Boundary IN: outline route (mocked fetch). Boundary OUT: host jump handler.
const WORKSPACE_ID = primarySessionFixture.workspace_id!;
const SESSION_ID = primarySessionFixture.id;
const OUTLINE_PATH = `/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/transcript/outline`;

function outline(count: number): SessionTranscriptOutlineResponse {
  return {
    entries: Array.from({ length: count }, (_, index) => ({
      at: `2026-09-06T14:${String(index).padStart(2, "0")}:00Z`,
      preview: `Ask number ${index + 1}`,
      reply_preview: index === count - 1 ? "" : `Reply number ${index + 1}`,
      sequence: (index + 1) * 10,
      turn_id: `turn-${index + 1}`,
    })),
  };
}

let outlineResponse: SessionTranscriptOutlineResponse = outline(0);

function renderTrail(overrides: Partial<SessionMessageTrailProps> = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const props: SessionMessageTrailProps = {
    onJumpToSequence: vi.fn(),
    paneHeightPx: 600,
    sessionId: SESSION_ID,
    viewportTopSequence: 25,
    visibleRange: { from: 20, to: 45 },
    workspaceId: WORKSPACE_ID,
    ...overrides,
  };
  render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delay={0}>
        <SessionMessageTrail {...props} />
      </TooltipProvider>
    </QueryClientProvider>
  );
  return props;
}

describe("SessionMessageTrail", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = new URL(
          typeof input === "string" ? input : input instanceof URL ? input : input.url,
          "http://localhost"
        );
        const body = url.pathname === OUTLINE_PATH ? outlineResponse : { error: "unexpected" };
        return new Response(JSON.stringify(body), {
          headers: { "Content-Type": "application/json" },
          status: url.pathname === OUTLINE_PATH ? 200 : 404,
        });
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should render one tick per sent message with the reading anchor current and jump on click", async () => {
    const user = userEvent.setup();
    outlineResponse = outline(6);
    const props = renderTrail();

    const ticks = await screen.findAllByTestId("session-trail-tick");
    expect(ticks).toHaveLength(6);
    expect(screen.getByRole("navigation", { name: "Message navigation" })).not.toHaveAttribute(
      "data-compressed"
    );
    // Anchor = last sent message at or above the viewport top (25 → message 2 at 20).
    expect(ticks[1]).toHaveAttribute("aria-current", "location");
    expect(ticks[1]).toHaveAttribute("data-vis", "anchor");
    expect(ticks[3]).toHaveAttribute("data-vis", "in-view");
    expect(ticks[5]).toHaveAttribute("data-vis", "rest");
    expect(ticks[1]).toHaveAttribute("aria-label", "Message 2: Ask number 2");
    // One roving tab stop: only the anchor is reachable with Tab.
    expect(ticks.filter(tick => tick.getAttribute("tabindex") === "0")).toHaveLength(1);

    await user.click(ticks[3]!);
    expect(props.onJumpToSequence).toHaveBeenCalledWith(40);
  });

  it("Should show what was asked and the final reply on hover and follow the keyboard", async () => {
    const user = userEvent.setup();
    outlineResponse = outline(4);
    const props = renderTrail();
    const ticks = await screen.findAllByTestId("session-trail-tick");

    await user.hover(ticks[2]!);
    await waitFor(() => expect(screen.getByText("Ask number 3")).toBeInTheDocument());
    expect(screen.getByText("Reply number 3")).toBeInTheDocument();
    expect(screen.getByText(/3 of 4/)).toBeInTheDocument();

    await user.tab();
    expect(ticks[1]).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(ticks[2]).toHaveFocus();
    await user.keyboard("{End}");
    expect(ticks[3]).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(props.onJumpToSequence).toHaveBeenCalledWith(40);
    await user.keyboard("{Home}");
    expect(ticks[0]).toHaveFocus();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(ticks[0]).not.toHaveFocus());
  });

  it("Should compress a dense rail to the pane share and stay absent below two messages", async () => {
    outlineResponse = outline(60);
    renderTrail({ paneHeightPx: 220 });
    const rail = await screen.findByRole("navigation", { name: "Message navigation" });
    expect(rail).toHaveAttribute("data-compressed", "true");
    expect(rail.style.height).toBe("176px");
    expect(screen.getAllByTestId("session-trail-tick")).toHaveLength(60);

    outlineResponse = outline(1);
    renderTrail({ sessionId: "single-message-session" });
    await waitFor(() => expect(screen.getAllByRole("navigation")).toHaveLength(1));
  });
});
