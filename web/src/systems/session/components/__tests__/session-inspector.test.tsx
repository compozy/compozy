import { render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SessionInspector, type InspectorUsage } from "../session-inspector";
import { userEvent } from "@testing-library/user-event";
import { SessionContextControl } from "../session-context-control";
import { deriveSessionContext } from "../../lib/session-context";
import { useSessionInspectorState } from "../../hooks/use-session-inspector-state";
import { sessionContextFixture, sessionContextTurnsFixture } from "../../mocks/context-fixtures";
import type { SessionContextPayload } from "../../types";

const ORIGINAL_MATCH_MEDIA = window.matchMedia;

function installMatchMedia(matches: boolean): void {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches,
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
    expect(screen.getByTestId("session-inspector-usage-tokens-in")).toHaveTextContent("128,400");
    expect(screen.getByTestId("session-inspector-usage-tokens-out")).toHaveTextContent("24,900");
    expect(screen.getByTestId("session-inspector-usage-total-tokens")).toHaveTextContent("153,300");
    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent("$18.42");
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent(
      "Across 12 turns"
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
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent("Across 1 turn");
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
    expect(screen.getByTestId("session-inspector-usage-tokens-in")).toHaveTextContent("—");
    expect(screen.queryByTestId("session-inspector-usage-turns")).not.toBeInTheDocument();
  });

  it("Should open the usage panel when only a positive turn count is reported", () => {
    render(<SessionInspector usage={{ turnCount: 2 }} />);

    expect(screen.getByTestId("session-inspector-usage-grid")).toBeInTheDocument();
    expect(screen.queryByTestId("session-inspector-usage-empty")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent("Across 2 turns");
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

  it("Should render actual cost as measured spend without an estimate glyph", () => {
    renderUsage({
      tokensIn: 1_000,
      costUsd: 18.42,
      costCurrency: "USD",
      costStatus: "actual",
      costSource: "agent_reported",
      turnCount: 3,
    });
    const cell = screen.getByTestId("session-inspector-usage-cost");
    expect(cell).toHaveTextContent("$18.42");
    expect(cell).not.toHaveTextContent("≈");
    expect(cell).toHaveTextContent("Reported by agent");
  });

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

describe("Session context", () => {
  it("Should open the tab-less sidebar from the keyboard and share its preference", async () => {
    window.localStorage.clear();
    const user = userEvent.setup();
    render(<ContextJourney />);
    const button = screen.getByRole("button", { name: "Context 35% used" });
    await user.tab();
    expect(button).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent("35% · 89.7K / 256K");
    expect(screen.getByRole("tooltip")).toHaveTextContent("as of turn 12");
    await user.keyboard("{Enter}");
    expect(screen.getByRole("complementary", { name: "Context" })).toBeVisible();
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
      context: { state: "reported", used: 89_700 } as SessionContextPayload,
      label: "Context 89.7K used",
      copy: "89.7K used",
    },
    {
      context: {
        ...sessionContextFixture,
        state: "estimated_size",
        size_source: "catalog",
        pressure_threshold: undefined,
      } as SessionContextPayload,
      label: "Context 35% used",
      copy: "window from model catalog",
    },
  ])(
    "Should render $label without inventing context or a compaction policy",
    async ({ context, label, copy }) => {
      const user = userEvent.setup();
      render(<SessionContextControl context={deriveSessionContext(context)} onOpen={vi.fn()} />);
      await user.hover(screen.getByRole("button", { name: label }));
      expect(await screen.findByRole("tooltip")).toHaveTextContent(copy);
      expect(screen.getByRole("tooltip")).not.toHaveTextContent("Compaction runs at");
      if (context.used == null) expect(screen.getByRole("button")).not.toHaveTextContent("0%");
    }
  );

  it("Should retain raw over-capacity values, mark stale, and expose the eligible threshold", async () => {
    const user = userEvent.setup();
    const context = deriveSessionContext({
      ...sessionContextFixture,
      used: 281_600,
      ratio: 1.1,
      stale: true,
    });
    render(<SessionContextControl context={context} onOpen={vi.fn()} />);
    await user.hover(screen.getByRole("button"));
    expect(await screen.findByRole("tooltip")).toHaveTextContent("110% · 281.6K / 256K");
    expect(screen.getByRole("tooltip")).toHaveTextContent("stale");
    expect(screen.getByRole("tooltip")).toHaveTextContent("Compaction runs at 85%");
    expect(screen.getByRole("button").querySelectorAll("circle")[1]).toHaveAttribute(
      "stroke-dasharray",
      "1 1"
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
    await user.click(screen.getByRole("button", { name: /CompozyOS context/ }));
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

  it("Should keep the last meter with an unavailable chip and omit unreported cache tiles", () => {
    const { rerender } = render(
      <SessionInspector
        context={deriveSessionContext(sessionContextFixture, { unavailable: true })}
        usage={{ cacheReadTokens: 12_800, cacheWriteTokens: 400 }}
      />
    );
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("unavailable");
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("89.7K");
    expect(screen.getByTestId("session-inspector-usage")).toHaveTextContent("Cache read");
    expect(screen.getByTestId("session-inspector-usage")).toHaveTextContent("Cache write");
    rerender(<SessionInspector context={deriveSessionContext(undefined, { unavailable: true })} />);
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("Usage unavailable");
    expect(screen.queryByText("Cache read")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-context-turns")).toHaveTextContent("No turns yet");
    expect(screen.getByTestId("session-context-activity").querySelector("li")).toBeNull();
  });

  it("Should order the union and compaction markers by sequence without claiming completion", () => {
    render(
      <SessionInspector
        turns={sessionContextTurnsFixture}
        activity={{ status: "Working for 49m 20s" }}
      />
    );
    const rows = screen.getAllByTestId("session-context-turn-row");
    expect(rows.map(row => within(row).getByText(/^Turn [0-9]+$/).textContent)).toEqual([
      "Turn 4",
      "Turn 3",
      "Turn 2",
      "Turn 1",
    ]);
    expect(rows[0]).toHaveTextContent("1K in200 out800 cache read");
    expect(rows[1]).toHaveTextContent("≈ 4K injected");
    expect(rows[2]).toHaveTextContent("20K / 256K");
    expect(rows[3]).toHaveTextContent("10K / 256K");
    expect(screen.getAllByTestId("session-context-compaction")[0]).toHaveTextContent(
      "CompozyOS compaction · at 88% · replay span not archived"
    );
    expect(screen.getAllByTestId("session-context-compaction")[1]).toHaveTextContent(
      "CompozyOS compaction · at 85% · replay span archived"
    );
    expect(screen.getByTestId("session-context-activity")).toHaveTextContent("Working for 49m 20s");
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
    expect(screen.getAllByTestId("session-context-turn-row")).toHaveLength(50);
    expect(screen.getAllByTestId("session-context-turn-row")[0]).toHaveTextContent("Turn 120");
    await user.click(screen.getByRole("button", { name: "Show earlier turns" }));
    expect(screen.getAllByTestId("session-context-turn-row")).toHaveLength(120);
  });

  it("Should keep the same Context content in the narrow drawer", () => {
    installMatchMedia(false);
    render(<SessionInspector drawerOpen />);
    expect(screen.getByRole("dialog", { name: "Context" })).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector")).toBeInTheDocument();
  });
});

// Invariant: loading is not a report, eligible pressure is visible, and per-turn costs preserve absence/currency.
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

  it("Should use the meter empty copy and show near compaction only with eligible pressure", () => {
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
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("near compaction");
    expect(screen.getByTestId("session-context-meter")).toHaveTextContent("Compaction runs at 85%");
    rerender(
      <SessionInspector
        context={deriveSessionContext({
          ...sessionContextFixture,
          state: "estimated_size",
          ratio: 0.88,
          size_source: "catalog",
          pressure_threshold: undefined,
        })}
      />
    );
    expect(screen.getByTestId("session-context-meter")).not.toHaveTextContent("near compaction");
  });

  it("Should show reported per-turn cost and currency while preserving the empty cost cell", () => {
    render(
      <SessionInspector
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
