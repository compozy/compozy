import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TooltipProvider } from "@compozy/ui";
import { SessionInspector, type InspectorUsage } from "../session-inspector";
import { userEvent } from "@testing-library/user-event";
import { expectFetchRequest } from "@/test/fetch-test-utils";
import { SessionContextControl } from "../session-context-control";
import { SessionContextMeterSection } from "../session-context-meter-section";
import { deriveSessionContext } from "../../lib/session-context";
import { isAwaitingUsageAfterCompaction } from "../../lib/session-context-view";
import { useSessionInspectorState } from "../../hooks/use-session-inspector-state";
import { sessionContextFixture, sessionContextTurnsFixture } from "../../mocks/context-fixtures";
import type { SessionContextPayload, SessionPayload, SessionUsageTurnsResponse } from "../../types";
import {
  continuedSessionFixture,
  deriveSourceSessionFixture,
  forkedSessionFixture,
} from "../../mocks/derive-fixtures";

const ORIGINAL_MATCH_MEDIA = window.matchMedia;

function installMatchMedia(matches: boolean): void {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: query.includes("min-width") && matches,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: () => false,
    }),
  });
}

beforeEach(() => {
  installMatchMedia(true);
});

afterEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: ORIGINAL_MATCH_MEDIA,
  });
});

describe("SessionInspector — Usage tab truthful wiring (/ §3.4)", () => {
  it("Should render real aggregated usage values from the daemon summary", () => {
    render(
      <SessionInspector
        usage={{
          tokensIn: 128_400,
          tokensOut: 24_900,
          totalTokens: 153_300,
          costUsd: 18.42,
          costCurrency: "USD",
          costStatus: "actual",
          costSource: "agent_reported",
          turnCount: 12,
        }}
      />
    );

    expect(screen.getByTestId("session-inspector-usage-grid")).toBeInTheDocument();
    expect(screen.queryByTestId("session-inspector-usage-empty")).not.toBeInTheDocument();
    // Cost and total lead; the in/out/cache split ships folded.
    expect(screen.queryByTestId("session-inspector-usage-tokens-in")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("session-inspector-usage-breakdown-toggle"));
    expect(screen.getByTestId("session-inspector-usage-tokens-in")).toHaveTextContent("128,400");
    expect(screen.getByTestId("session-inspector-usage-tokens-out")).toHaveTextContent("24,900");
    expect(screen.getByTestId("session-inspector-usage-total-tokens")).toHaveTextContent("153,300");
    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent("$18.42");
    expect(screen.getByTestId("session-inspector-usage-cost")).not.toHaveTextContent("≈");
    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent(
      "Reported by agent"
    );
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent(
      "across 12 turns"
    );
  });

  it("Should format a non-USD cost with its currency code", () => {
    const expectedCost = new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: "EUR",
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(2.5);
    render(
      <SessionInspector
        usage={{
          costUsd: 2.5,
          costCurrency: "EUR",
          costStatus: "actual",
          costSource: "agent_reported",
          turnCount: 1,
        }}
      />
    );

    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent(expectedCost);
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent("across 1 turn");
  });

  it("Should show the truthful empty state when the session reported no usage", () => {
    render(<SessionInspector usage={{ turnCount: 0 }} />);

    expect(screen.queryByTestId("session-inspector-usage-grid")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-empty")).toHaveTextContent("No usage yet");
    expect(screen.queryByTestId("session-inspector-usage-turns")).not.toBeInTheDocument();
  });

  it("Should open the usage panel for a classification-only summary with no token counters", () => {
    render(
      <SessionInspector usage={{ costStatus: "included", costSource: "none", turnCount: 0 }} />
    );

    expect(screen.getByTestId("session-inspector-usage-grid")).toBeInTheDocument();
    expect(screen.queryByTestId("session-inspector-usage-empty")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent("Included");
    fireEvent.click(screen.getByTestId("session-inspector-usage-breakdown-toggle"));
    expect(screen.getByTestId("session-inspector-usage-tokens-in")).toHaveTextContent("—");
    expect(screen.queryByTestId("session-inspector-usage-turns")).not.toBeInTheDocument();
  });

  it("Should open the usage panel when only a positive turn count is reported", () => {
    render(<SessionInspector usage={{ turnCount: 2 }} />);

    expect(screen.getByTestId("session-inspector-usage-grid")).toBeInTheDocument();
    expect(screen.queryByTestId("session-inspector-usage-empty")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent("across 2 turns");
    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent("—");
  });

  it("Should keep the empty state when only a statusless cost amount is present", () => {
    render(<SessionInspector usage={{ costUsd: 18.42, costCurrency: "USD", turnCount: 0 }} />);

    expect(screen.queryByTestId("session-inspector-usage-grid")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-empty")).toHaveTextContent("No usage yet");
  });
});

describe("SessionInspector — Usage tab cost provenance (W4)", () => {
  function renderUsage(usage: InspectorUsage) {
    render(<SessionInspector usage={usage} />);
  }

  it("Should mark estimated cost with the ≈ cue and source, never as measured spend", () => {
    renderUsage({
      tokensIn: 1_000,
      costUsd: 0.18,
      costCurrency: "USD",
      costStatus: "estimated",
      costSource: "catalog_config",
      turnCount: 3,
    });
    const cell = screen.getByTestId("session-inspector-usage-cost");
    expect(cell).toHaveTextContent("≈");
    expect(cell).toHaveTextContent("Estimated");
    expect(cell).toHaveTextContent("Catalog rate");
  });

  it("Should render included usage with no monetary amount while tokens stay visible", () => {
    renderUsage({
      tokensIn: 128_400,
      tokensOut: 24_900,
      totalTokens: 153_300,
      costStatus: "included",
      costSource: "none",
      turnCount: 3,
    });
    const cell = screen.getByTestId("session-inspector-usage-cost");
    expect(cell).toHaveTextContent("Included");
    expect(cell).not.toHaveTextContent("$");
    fireEvent.click(screen.getByTestId("session-inspector-usage-breakdown-toggle"));
    expect(screen.getByTestId("session-inspector-usage-tokens-in")).toHaveTextContent("128,400");
    expect(screen.getByTestId("session-inspector-usage-total-tokens")).toHaveTextContent("153,300");
  });

  it("Should render unknown cost with no monetary amount while tokens stay visible", () => {
    renderUsage({ totalTokens: 153_300, costStatus: "unknown", turnCount: 3 });
    const cell = screen.getByTestId("session-inspector-usage-cost");
    expect(cell).toHaveTextContent("Unavailable");
    expect(cell).not.toHaveTextContent("$");
    expect(screen.getByTestId("session-inspector-usage-total-tokens")).toHaveTextContent("153,300");
  });
});

// Invariant: the single Context surface renders reported facts, bounded magnitude, and honest absence.
// Owner: session domain components; canonical suite: SessionInspector.

function ContextJourney() {
  const inspector = useSessionInspectorState("context-journey");
  return (
    <>
      <SessionContextControl
        context={deriveSessionContext(sessionContextFixture)}
        onOpen={() => inspector.setOpen(true)}
      />
      {inspector.open ? (
        <SessionInspector context={deriveSessionContext(sessionContextFixture)} />
      ) : null}
    </>
  );
}

type UsageTurn = SessionUsageTurnsResponse["turns"][number];
type CompactionMarker = SessionUsageTurnsResponse["compactions"][number];

function contextTurn(turnId: string, sequence: number, used: number): UsageTurn {
  return {
    turn_id: turnId,
    sequence,
    usage: { sequence, timestamp: "2026-09-12T09:00:00Z", context_used: used },
  };
}

function compactionMarker(overrides: Partial<CompactionMarker>): CompactionMarker {
  return {
    compaction_id: `compaction-${overrides.sequence ?? 0}`,
    trigger: "agent",
    status: "completed",
    turn_id: "turn-1",
    sequence: 0,
    at: "2026-09-12T09:20:00Z",
    ...overrides,
  };
}

describe("Session context", () => {
  it("Should open the tab-less sidebar from the keyboard and share its preference", async () => {
    window.localStorage.clear();
    const user = userEvent.setup();
    render(<ContextJourney />);
    const button = screen.getByRole("button", { name: "Context 35% used" });
    await user.tab();
    expect(button).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent("35% · 89.7K / 256K");
    expect(screen.getByRole("tooltip")).not.toHaveTextContent("as of turn");
    expect(screen.getByRole("tooltip")).not.toHaveTextContent("reported");
    await user.keyboard("{Enter}");
    expect(screen.getByRole("complementary", { name: "Context" })).toBeVisible();
    const meter = screen.getByTestId("session-context-meter");
    expect(meter).toHaveTextContent("35%");
    expect(meter).not.toHaveTextContent("reported");
    expect(meter).not.toHaveTextContent("as of turn");
    expect(meter).not.toHaveTextContent("summarizes older messages");
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(document.querySelector('[data-testid^="session-inspector-tab-"]')).toBeNull();
    expect(screen.getByTestId("session-inspector").children).toHaveLength(5);
    expect(
      Array.from(screen.getByTestId("session-inspector").children).map(element =>
        element.getAttribute("data-testid")
      )
    ).toEqual([
      "session-context-meter",
      "session-context-injected",
      "session-inspector-usage",
      "session-context-turns",
      "session-context-activity",
    ]);
    expect(window.localStorage.getItem("compozy:session:inspector:v2")).toContain(
      '"context-journey":true'
    );
  });

  it.each([
    {
      context: { state: "unknown" } as SessionContextPayload,
      label: "Context usage unknown",
      copy: "This agent hasn't reported context usage.",
    },
    {
      // The wire shape right after a terminal compaction: the reading is cleared, not zero.
      context: {
        state: "unknown",
        used: null,
        size: null,
        ratio: null,
        injected: sessionContextFixture.injected,
      } as SessionContextPayload,
      afterCompaction: true,
      label: "Context usage unknown",
      copy: "Context compacted. Waiting for the agent's next usage report.",
    },
    {
      context: { state: "reported", used: 89_700 } as SessionContextPayload,
      label: "Context 89.7K used",
      copy: "89.7K used",
    },
    {
      context: {
        ...sessionContextFixture,
        state: "estimated_size",
        size_source: "catalog",
      } as SessionContextPayload,
      label: "Context 35% used",
      copy: "Size from the model's specs.",
    },
  ])(
    "Should render $label without inventing context or a compaction policy",
    async ({ context, afterCompaction, label, copy }) => {
      const user = userEvent.setup();
      // This case owns tooltip content, not the primitive's hover delay.
      render(
        <TooltipProvider delay={0}>
          <SessionContextControl
            context={deriveSessionContext(context, {
              awaitingUsageAfterCompaction: afterCompaction,
            })}
            onOpen={vi.fn()}
          />
        </TooltipProvider>
      );
      await user.hover(screen.getByRole("button", { name: label }));
      expect(await screen.findByRole("tooltip")).toHaveTextContent(copy);
      expect(screen.getByRole("tooltip")).not.toHaveTextContent("summarizes older messages");
      if (context.used == null) expect(screen.getByRole("button")).not.toHaveTextContent("0%");
    }
  );

  it("Should retain raw over-capacity values and mark stale", async () => {
    const user = userEvent.setup();
    const context = deriveSessionContext({
      ...sessionContextFixture,
      used: 281_600,
      ratio: 1.1,
      stale: true,
    });
    render(
      <TooltipProvider delay={0}>
        <SessionContextControl context={context} onOpen={vi.fn()} />
      </TooltipProvider>
    );
    await user.hover(screen.getByRole("button"));
    expect(await screen.findByRole("tooltip")).toHaveTextContent("110% · 281.6K / 256K");
    expect(screen.getByRole("tooltip")).toHaveTextContent("Updated a while ago");
    expect(screen.getByRole("tooltip")).not.toHaveTextContent("summarizes older messages");
    expect(screen.getByRole("button").querySelectorAll("circle")[1]).toHaveAttribute(
      "stroke-dasharray",
      "1 1"
    );
    // Freshness is shape: the dotted mask rides the arc and its fill stays the truth.
    expect(screen.getByRole("button")).toHaveAttribute("data-state", "stale");
    expect(screen.getByRole("button").querySelectorAll("circle")[1]).toHaveAttribute(
      "mask",
      expect.stringMatching(/^url\(#/)
    );
    expect(screen.getByRole("button").querySelectorAll("circle")[1]).toHaveAttribute(
      "stroke",
      "var(--color-subtle)"
    );
  });

  it("Should bound the bar to used and size while retaining the raw estimate", () => {
    const context = deriveSessionContext({
      ...sessionContextFixture,
      injected: { ...sessionContextFixture.injected!, tokens: 90_000 },
    });
    expect(context.display).toEqual({ compozy: 89_700, agent: 0, free: 166_300, total: 256_000 });
    render(<SessionInspector context={context} />);
    const meter = screen.getByTestId("session-context-meter");
    expect(meter).toHaveTextContent("estimate exceeds reported");
    expect(meter).toHaveTextContent("90K");
    expect(meter).toHaveTextContent("166.3K");
    expect(within(meter).getByRole("img")).toHaveAccessibleName(
      "Context window: 89.7K of 256K used"
    );
    expect(deriveSessionContext(sessionContextFixture).display).toEqual({
      compozy: 12_400,
      agent: 77_300,
      free: 166_300,
      total: 256_000,
    });
    expect(deriveSessionContext({ ...sessionContextFixture, used: 281_600 }).display?.free).toBe(0);
  });

  it("Should show attribution without an agent report and disclose receipts honestly", async () => {
    const user = userEvent.setup();
    render(
      <SessionInspector
        context={deriveSessionContext({
          state: "unknown",
          injected: {
            ...sessionContextFixture.injected!,
            rows: sessionContextFixture.injected!.rows.map(row => ({ ...row, stale: true })),
          },
        })}
      />
    );
    expect(
      within(screen.getByTestId("session-context-meter")).queryByRole("img")
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("Context usage unknown");
    expect(screen.getByTestId("session-context-meter")).not.toHaveTextContent("No context report");
    await user.click(screen.getByRole("button", { name: /CompozyOS context/ }));
    expect(screen.getByTestId("session-context-injected")).toHaveTextContent(
      "The agent hasn't said how much it can hold, so there is nothing to compare these against."
    );
    const rows = screen.getAllByTestId("session-context-injected-row");
    expect(rows).toHaveLength(4);
    expect(
      screen
        .getByTestId("session-context-injected")
        .querySelector('[data-slot="status-breakdown-bar"]')
    ).toBeNull();
    expect(rows[0]).toHaveTextContent("≈ 3.1K");
    expect(rows[0]).toHaveTextContent("unchanged since turn 1 · last seen turn 12");
    expect(rows[1]).toHaveTextContent("modified by a hook");
    expect(rows[2]).toHaveTextContent("architecture.png");
    expect(rows[2]).toHaveTextContent("200 KiB");
    expect(rows[2]).not.toHaveTextContent("≈");
    expect(rows[3]).toHaveTextContent("included in the startup prompt");
    expect(rows[3]).not.toHaveTextContent("≈");
    expect(rows[0]).toHaveTextContent("may have been summarized");
  });

  it("Should keep the last meter with an unavailable chip and omit unreported cache tiles", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <SessionInspector
        context={deriveSessionContext(sessionContextFixture, { unavailable: true })}
        usage={{ cacheReadTokens: 12_800, cacheWriteTokens: 400 }}
      />
    );
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("unavailable");
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("89.7K");
    await user.click(screen.getByTestId("session-inspector-usage-breakdown-toggle"));
    expect(screen.getByTestId("session-inspector-usage")).toHaveTextContent("Cache read");
    expect(screen.getByTestId("session-inspector-usage")).toHaveTextContent("Cache write");
    rerender(<SessionInspector context={deriveSessionContext(undefined, { unavailable: true })} />);
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("Usage unavailable");
    expect(screen.queryByText("Cache read")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-context-turns")).not.toHaveTextContent("No turns yet");
    await user.click(screen.getByRole("button", { name: /^Turns/ }));
    expect(screen.getByTestId("session-context-turns")).toHaveTextContent("No turns yet");
    expect(screen.getByTestId("session-context-activity").querySelector("li")).toBeNull();
  });

  it("Should order the union and compaction markers by sequence without claiming completion", async () => {
    const user = userEvent.setup();
    render(
      <SessionInspector
        turns={sessionContextTurnsFixture}
        activity={{ status: "Working for 49m 20s" }}
      />
    );
    const turnsHead = screen.getByRole("button", { name: /^Turns/ });
    expect(turnsHead).toHaveAttribute("aria-expanded", "false");
    expect(turnsHead).toHaveTextContent("4 turns");
    expect(screen.queryAllByTestId("session-context-turn-row")).toHaveLength(0);
    expect(screen.getByTestId("session-context-turns")).not.toHaveTextContent("newest first");
    await user.click(turnsHead);
    expect(turnsHead).toHaveAttribute("aria-expanded", "true");
    const rows = screen.getAllByTestId("session-context-turn-row");
    expect(rows.map(row => within(row).getByText(/^Turn [0-9]+$/).textContent)).toEqual([
      "Turn 4",
      "Turn 3",
      "Turn 2",
      "Turn 1",
    ]);
    expect(rows[0]).toHaveTextContent("in 1K · out 200 · cache 800");
    expect(rows[1]).toHaveTextContent("≈ 4K injected");
    expect(rows[2]).toHaveTextContent("20K / 256K");
    expect(rows[3]).toHaveTextContent("10K / 256K");
    const markers = screen.getAllByTestId("session-context-compaction-marker");
    expect(markers[0]).toHaveTextContent("Requested compaction · completed · 225.3K");
    expect(markers[1]).toHaveTextContent("Agent compaction · completed · 217.6K");
    expect(
      Array.from(rows[0]!.parentElement!.children).map(row => row.getAttribute("data-testid"))
    ).toEqual([
      "session-context-compaction-marker",
      "session-context-turn-row",
      "session-context-turn-row",
      "session-context-compaction-marker",
      "session-context-turn-row",
      "session-context-turn-row",
    ]);
    expect(screen.getByTestId("session-context-activity")).toHaveTextContent("Working for 49m 20s");
  });

  it.each([
    { trigger: "agent", status: "completed", text: "Agent compaction · completed" },
    { trigger: "requested", status: "completed", text: "Requested compaction · completed" },
    { trigger: "agent", status: "in_progress", text: "Agent compaction · in progress" },
    { trigger: "requested", status: "failed", text: "Requested compaction · failed" },
    { trigger: "agent", status: "cancelled", text: "Agent compaction · cancelled" },
    {
      trigger: "requested",
      status: "compaction_paused",
      text: "Requested compaction · compaction_paused",
    },
  ])("Should render a $trigger compaction marker as $text", ({ trigger, status, text }) => {
    render(
      <SessionInspector
        turnsDefaultOpen
        turns={{
          turns: [],
          compactions: [compactionMarker({ trigger, status, sequence: 1 })],
        }}
      />
    );
    const marker = screen.getByTestId("session-context-compaction-marker");
    expect(marker.textContent).toBe(text);
    expect(marker).toHaveAttribute("data-trigger", trigger);
    expect(marker).toHaveAttribute("data-status", status);
  });

  it("Should show tokens before to after only for the figures the daemon knows", () => {
    render(
      <SessionInspector
        turnsDefaultOpen
        turns={{
          turns: [
            contextTurn("turn-1", 10, 180_000),
            contextTurn("turn-2", 30, 42_000),
            // The compacted turn's own later report is not a reading from a later turn.
            contextTurn("turn-3", 55, 30_000),
            // A report after a second compaction belongs to that compaction, not the first.
            contextTurn("turn-5", 120, 61_000),
          ],
          compactions: [
            compactionMarker({ sequence: 20, turn_id: "turn-1", context_used: 180_000 }),
            compactionMarker({ sequence: 50, turn_id: "turn-3", context_used: 150_000 }),
            compactionMarker({ sequence: 70, turn_id: "turn-4" }),
            compactionMarker({ sequence: 100, turn_id: "turn-4", context_used: 90_000 }),
            compactionMarker({ sequence: 110, turn_id: "turn-4", context_used: 95_000 }),
          ],
        }}
      />
    );
    // Newest first.
    const [fifth, fourth, third, second, first] = screen
      .getAllByTestId("session-context-compaction-marker")
      .map(marker => marker.textContent);
    expect(first).toBe("Agent compaction · completed · 180K → 42K");
    expect(second).toBe("Agent compaction · completed · 150K");
    expect(third).toBe("Agent compaction · completed");
    expect(fourth).toBe("Agent compaction · completed · 90K");
    expect(fifth).toBe("Agent compaction · completed · 95K → 61K");
  });

  it.each([
    {
      name: "a finished compaction with no report after it",
      turns: {
        turns: [contextTurn("turn-1", 10, 180_000)],
        compactions: [compactionMarker({ sequence: 20 })],
      },
      awaiting: true,
    },
    {
      name: "a failed compaction with no report after it",
      turns: { turns: [], compactions: [compactionMarker({ sequence: 20, status: "failed" })] },
      awaiting: true,
    },
    {
      name: "a report from a later turn",
      turns: {
        turns: [contextTurn("turn-1", 10, 180_000), contextTurn("turn-2", 30, 42_000)],
        compactions: [compactionMarker({ sequence: 20 })],
      },
      awaiting: false,
    },
    {
      name: "a compaction still in progress",
      turns: {
        turns: [],
        compactions: [compactionMarker({ sequence: 20, status: "in_progress" })],
      },
      awaiting: false,
    },
    {
      name: "a vendor status that is not finished",
      turns: {
        turns: [],
        compactions: [compactionMarker({ sequence: 20, status: "compaction_paused" })],
      },
      awaiting: false,
    },
    { name: "no compaction at all", turns: { turns: [], compactions: [] }, awaiting: false },
  ] satisfies { name: string; turns: SessionUsageTurnsResponse; awaiting: boolean }[])(
    "Should say a compaction cleared the reading only after $name: $awaiting",
    ({ turns, awaiting }) => {
      expect(isAwaitingUsageAfterCompaction(turns)).toBe(awaiting);
    }
  );

  it("Should explain an empty meter after a compaction instead of claiming the agent never reported", () => {
    const cleared = {
      state: "unknown",
      used: null,
      size: null,
      ratio: null,
      injected: sessionContextFixture.injected,
    } as SessionContextPayload;
    const { rerender } = render(
      <SessionContextMeterSection
        context={deriveSessionContext(cleared, { awaitingUsageAfterCompaction: true })}
      />
    );
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent(
      "Context compacted. Waiting for the agent's next usage report."
    );
    // No attribution rows to show: the meter still names the compaction, not a first report.
    rerender(
      <SessionContextMeterSection
        context={deriveSessionContext({ state: "unknown" }, { awaitingUsageAfterCompaction: true })}
      />
    );
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent(
      "Waiting for the agent's next usage report."
    );
    // The same empty reading without a compaction keeps the plain wording.
    rerender(<SessionContextMeterSection context={deriveSessionContext(cleared)} />);
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent(
      "This agent hasn't reported context usage."
    );
  });

  it("Should show the newest fifty turns and reveal earlier turns on demand", async () => {
    const user = userEvent.setup();
    render(
      <SessionInspector
        turns={{
          compactions: [],
          turns: Array.from({ length: 120 }, (_, index) => ({
            turn_id: `turn-${index + 1}`,
            sequence: index + 1,
          })),
        }}
      />
    );
    await user.click(screen.getByRole("button", { name: /^Turns/ }));
    expect(screen.getAllByTestId("session-context-turn-row")).toHaveLength(50);
    expect(screen.getAllByTestId("session-context-turn-row")[0]).toHaveTextContent("Turn 120");
    await user.click(screen.getByRole("button", { name: /^Show earlier turns/ }));
    expect(screen.getAllByTestId("session-context-turn-row")).toHaveLength(120);
  });

  it("Should keep the same Context content in the narrow drawer", () => {
    installMatchMedia(false);
    render(<SessionInspector drawerOpen />);
    expect(screen.getByRole("dialog", { name: "Context" })).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector")).toBeInTheDocument();
  });
});

// Invariant: loading is not a report, a nearly full window shows its fill without a warning state, and per-turn costs preserve absence/currency.
// Owner: session domain surfaces; canonical suite: SessionInspector.
describe("Fable context surface corrections", () => {
  it("Should reserve the loading control without asserting that the agent has not reported", async () => {
    const user = userEvent.setup();
    render(
      <SessionContextControl
        context={deriveSessionContext(undefined, { loading: true })}
        onOpen={vi.fn()}
      />
    );
    const button = screen.getByRole("button", { name: "Context usage loading" });
    expect(button).toHaveAttribute("aria-busy", "true");
    await user.tab();
    expect(await screen.findByRole("tooltip")).toHaveTextContent("Loading context usage");
    expect(screen.getByRole("tooltip")).not.toHaveTextContent("hasn't reported");
    expect(button).toHaveAttribute("aria-describedby", screen.getByRole("tooltip").id);
  });

  it("Should use the meter empty copy and keep a nearly full window free of warning copy", () => {
    const { rerender } = render(<SessionInspector />);
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("No context report yet");
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent(
      "The meter fills once the agent reports its first turn."
    );
    rerender(
      <SessionInspector
        context={deriveSessionContext({ ...sessionContextFixture, ratio: 0.88, used: 225_280 })}
      />
    );
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("88%");
    expect(screen.getByTestId("session-context-meter")).not.toHaveTextContent("almost full");
    expect(screen.getByTestId("session-context-meter")).not.toHaveTextContent(
      "summarizes older messages"
    );
  });

  it("Should show reported per-turn cost and currency while preserving the empty cost cell", () => {
    render(
      <SessionInspector
        turnsDefaultOpen
        turns={{
          compactions: [],
          turns: [
            {
              turn_id: "turn-1",
              sequence: 1,
              usage: { timestamp: "2026-09-12T12:00:00Z", cost_amount: 1.25, cost_currency: "USD" },
            },
            {
              turn_id: "turn-2",
              sequence: 2,
              usage: { timestamp: "2026-09-12T12:01:00Z", cost_amount: 2.5, cost_currency: "EUR" },
            },
            { turn_id: "turn-3", sequence: 3 },
          ],
        }}
        activity={{ tools: "38 tools", thoughts: "12 thoughts", queued: "1 queued" }}
      />
    );
    const rows = screen.getAllByTestId("session-context-turn-row");
    expect(within(rows[0]!).getByRole("definition")).toHaveTextContent("—");
    expect(within(rows[1]!).getByRole("definition")).toHaveTextContent("€2.50");
    expect(within(rows[2]!).getByRole("definition")).toHaveTextContent("$1.25");
    expect(screen.getByTestId("session-context-activity").querySelectorAll("li")).toHaveLength(2);
    expect(screen.getByTestId("session-context-activity")).toHaveTextContent(
      "38 tools · 12 thoughts"
    );
  });
});

// Invariant (UT-100): a derived session's inspector states its origin, the seed the daemon used
// (a failed native clone says the carried context was used, with the redacted error on hover),
// and a bind-time `route_not_found` in the daemon's own words; roots show no Origin section.
// Owning layer: the inspector Origin section. Canonical suite: this file.
describe("SessionInspector — origin", () => {
  it("Should state a failed native clone and the carried context it fell back to", () => {
    const child = continuedSessionFixture();
    render(
      <SessionInspector
        session={{
          ...child,
          derivation: {
            ...child.derivation!,
            seed: "native_fork",
            native_state: "failed",
            native_fork_error: "session/load rejected: <redacted>",
          },
        }}
      />
    );

    expect(screen.getByTestId("ledger-origin")).toHaveTextContent("continue · from claude");
    const seed = screen.getByTestId("ledger-seed");
    expect(seed).toHaveTextContent("native clone · failed — carried context used");
    expect(within(seed).getByTitle("session/load rejected: <redacted>")).toBeInTheDocument();
  });

  it("Should show the replay seed and a fork's anchor", () => {
    const child = continuedSessionFixture();
    render(
      <SessionInspector
        session={{
          ...child,
          lineage: { ...child.lineage!, kind: "fork", origin_message_id: "msg_01J9R3ZQ8PVX" },
        }}
      />
    );

    expect(screen.getByTestId("ledger-origin")).toHaveTextContent(
      "fork · through msg_01J9R3ZQ8PVX"
    );
    expect(screen.getByTestId("ledger-seed")).toHaveTextContent("replay");
  });

  it("Should state a clone refused at fork time as failed, with the carried context", () => {
    const child = forkedSessionFixture();
    render(
      <SessionInspector
        session={{
          ...child,
          derivation: {
            ...child.derivation!,
            seed: "replay",
            native_fork_error: "session/fork: method not found",
          },
        }}
      />
    );

    expect(screen.getByTestId("ledger-origin")).toHaveTextContent("fork");
    const seed = screen.getByTestId("ledger-seed");
    expect(seed).toHaveTextContent("native clone · failed — carried context used");
    expect(within(seed).getByTitle("session/fork: method not found")).toBeInTheDocument();
  });

  it("Should render a bind-time route_not_found with the daemon message", () => {
    const child = continuedSessionFixture();
    const failure = 'route_not_found: agent "claude" route 2 changed since it was chosen';
    render(
      <SessionInspector
        session={{ ...child, runtime: { ...child.runtime, status: "unbound", failure } }}
      />
    );

    expect(screen.getByTestId("ledger-route-failure")).toHaveTextContent(failure);
  });

  it("Should show no Origin section for a root session", () => {
    render(<SessionInspector session={deriveSourceSessionFixture} />);

    expect(screen.queryByTestId("session-inspector-origin")).not.toBeInTheDocument();
  });
});

// Invariant (UT-W09): Compact now asks the agent to compact through the command it advertises: it is
// offered only for `compact`/`compress`, inert while a turn runs, sends exactly one request per click,
// and states a refusal in the daemon's words. Owning layer: the rail's meter section. Canonical suite: this file.
describe("SessionInspector — Compact now", () => {
  const compactPath = (session: SessionPayload) =>
    `/api/workspaces/${session.workspace_id}/sessions/${session.id}/compact`;
  const advertising = (...names: string[]): SessionPayload => ({
    ...deriveSourceSessionFixture,
    available_commands: names.map(name => ({ name, description: `${name} the conversation` })),
  });
  const respond = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), {
      headers: { "Content-Type": "application/json" },
      status,
    });
  const receipt = (session: SessionPayload) => ({
    command: "compact",
    prompt_id: "prompt-compact-1",
    session_id: session.id,
    status: "accepted",
  });
  function renderRail(session: SessionPayload) {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const tree = (current: SessionPayload) => (
      <QueryClientProvider client={queryClient}>
        <SessionInspector session={current} />
      </QueryClientProvider>
    );
    const view = render(tree(session));
    return { rerenderSession: (next: SessionPayload) => view.rerender(tree(next)) };
  }

  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([[["compact"]], [["compress"]], [["review", "compress", "compact"]]])(
    "Should offer the action when the agent advertises %j",
    commands => {
      renderRail(advertising(...commands));

      expect(screen.getByRole("button", { name: "Compact now" })).toBeEnabled();
    }
  );

  it.each([[["review", "init"]], [[]]])(
    "Should not offer the action when the agent advertises %j",
    commands => {
      renderRail(advertising(...commands));

      expect(screen.queryByRole("button", { name: "Compact now" })).not.toBeInTheDocument();
    }
  );

  it.each([
    { reason: "a turn is running", patch: { badge: "running" as const } },
    { reason: "the session is stopped", patch: { state: "stopped" as const } },
  ])("Should keep the action inert when $reason", async ({ patch }) => {
    renderRail({ ...advertising("compact"), ...patch });

    const button = screen.getByRole("button", { name: "Compact now" });
    expect(button).toBeDisabled();
    await userEvent.setup().click(button);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("Should send one request per click and hold the button until the request settles", async () => {
    const session = advertising("compact");
    let accept: (response: Response) => void = () => undefined;
    vi.mocked(globalThis.fetch).mockReturnValueOnce(
      new Promise<Response>(resolve => {
        accept = resolve;
      })
    );
    renderRail(session);

    const button = screen.getByRole("button", { name: "Compact now" });
    await userEvent.setup().click(button);

    await expectFetchRequest({ body: {}, method: "POST", path: compactPath(session) });
    await waitFor(() => expect(button).toBeDisabled());
    expect(button).toHaveAttribute("aria-busy", "true");
    fireEvent.click(button);
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);

    accept(respond(202, receipt(session)));
    await waitFor(() => expect(button).toBeEnabled());
    expect(screen.queryByTestId("session-context-compact-error")).not.toBeInTheDocument();
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  });

  it.each([
    { code: "session_busy", message: "session: a prompt is already in progress" },
    { code: "compaction_unsupported", message: "session: agent does not advertise compaction" },
  ])(
    "Should state a $code refusal in the daemon's words and clear it on the next attempt",
    async ({ code, message }) => {
      const session = advertising("compact");
      vi.mocked(globalThis.fetch)
        .mockResolvedValueOnce(respond(409, { code, error: message }))
        .mockResolvedValueOnce(respond(202, receipt(session)));
      renderRail(session);

      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: "Compact now" }));
      expect(await screen.findByTestId("session-context-compact-error")).toHaveTextContent(message);
      expect(screen.getByRole("button", { name: "Compact now" })).toBeEnabled();

      await user.click(screen.getByRole("button", { name: "Compact now" }));
      await waitFor(() =>
        expect(screen.queryByTestId("session-context-compact-error")).not.toBeInTheDocument()
      );
    }
  );

  it("Should keep a busy refusal while the turn runs and drop it once the turn ends", async () => {
    const session = advertising("compact");
    vi.mocked(globalThis.fetch).mockResolvedValueOnce(
      respond(409, { code: "session_busy", error: "session: a prompt is already in progress" })
    );
    const { rerenderSession } = renderRail(session);

    await userEvent.setup().click(screen.getByRole("button", { name: "Compact now" }));
    expect(await screen.findByTestId("session-context-compact-error")).toBeInTheDocument();

    rerenderSession({ ...session, badge: "running" });
    expect(screen.getByTestId("session-context-compact-error")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Compact now" })).toBeDisabled();

    rerenderSession(session);
    await waitFor(() =>
      expect(screen.queryByTestId("session-context-compact-error")).not.toBeInTheDocument()
    );
  });

  it("Should use a generic line for a failure the daemon did not classify", async () => {
    const session = advertising("compact");
    vi.mocked(globalThis.fetch).mockResolvedValueOnce(
      respond(500, { error: "internal detail the rail must not echo" })
    );
    renderRail(session);

    await userEvent.setup().click(screen.getByRole("button", { name: "Compact now" }));

    const error = await screen.findByTestId("session-context-compact-error");
    expect(error).toHaveTextContent("Couldn't request compaction. Try again.");
    expect(error).not.toHaveTextContent("internal detail");
  });
});
