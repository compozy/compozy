import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DETAIL_INSPECTOR_INLINE_BREAKPOINT } from "@agh/ui";

import { SessionLedgerUnavailableError } from "../../adapters/session-api";
import type { SessionLedgerResponse } from "../../types";
import { SessionInspector, type InspectorUsage } from "../session-inspector";

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

function makeLedger(overrides?: Partial<SessionLedgerResponse>): SessionLedgerResponse {
  return {
    meta: {
      version: 1,
      session_id: "sess_123",
      workspace_id: "ws_alpha",
      root_session_id: "sess_root",
      parent_session_id: "sess_parent",
      spawn_depth: 2,
      path: "/sessions/ws_alpha/sess_123/ledger.jsonl",
      checksum: "sha256:abc123",
      created_at: "2026-04-20T10:00:00Z",
      stopped_at: "2026-04-20T11:00:00Z",
      ...overrides?.meta,
    },
    events: overrides?.events ?? [
      { sequence: 1, event_type: "session.started", emitted_at: "2026-04-20T10:00:00Z" },
      { sequence: 2, event_type: "memory.recall", emitted_at: "2026-04-20T10:01:00Z" },
      { sequence: 3, event_type: "memory.event", emitted_at: "2026-04-20T10:02:00Z" },
    ],
  };
}

function openMemoryTab() {
  fireEvent.click(screen.getByTestId("session-inspector-tab-memory"));
}

function openUsageTab() {
  fireEvent.click(screen.getByTestId("session-inspector-tab-usage"));
}

describe("SessionInspector — DetailInspector chrome (/ §3)", () => {
  it("Should consume <DetailInspector> with 5 tabs in a single flat tab strip", () => {
    const ledger = makeLedger();
    render(<SessionInspector messages={[]} sessionId="sess_123" memory={{ ledger }} />);

    expect(screen.getByTestId("session-inspector-tab-trace")).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-tab-usage")).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-tab-memory")).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-tab-files")).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-tab-vault")).toBeInTheDocument();
  });

  it("Should render inline at >= 1440 px viewport (data-mode=inline) at 320 px width", () => {
    installMatchMedia(true);
    const { container } = render(
      <SessionInspector messages={[]} sessionId="sess_123" memory={{ ledger: null }} />
    );
    const root = container.querySelector<HTMLElement>(
      '[data-slot="detail-inspector"][data-mode="inline"]'
    );
    expect(root).not.toBeNull();
    expect(root?.style.width).toBe("320px");
  });

  it("Should collapse into the right-anchored sheet drawer below 1440 px", () => {
    installMatchMedia(false);
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        memory={{ ledger: null }}
        drawerOpen
        onDrawerOpenChange={() => {}}
      />
    );
    const drawer = document.querySelector('[data-slot="detail-inspector"][data-mode="drawer"]');
    expect(drawer).not.toBeNull();
  });

  it("Should expose DETAIL_INSPECTOR_INLINE_BREAKPOINT as the canonical 1440 px constant", () => {
    expect(DETAIL_INSPECTOR_INLINE_BREAKPOINT).toBe(1440);
  });
});

describe("SessionInspector — Usage tab truthful wiring (/ §3.4)", () => {
  it("Should render real aggregated usage values from the daemon summary", () => {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
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
        memory={{ ledger: null }}
      />
    );

    openUsageTab();

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
        messages={[]}
        sessionId="sess_123"
        usage={{
          costUsd: 2.5,
          costCurrency: "EUR",
          costStatus: "actual",
          costSource: "agent_reported",
          turnCount: 1,
        }}
        memory={{ ledger: null }}
      />
    );

    openUsageTab();

    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent(expectedCost);
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent("Across 1 turn");
  });

  it("Should show the truthful empty state when the session reported no usage", () => {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        usage={{ turnCount: 0 }}
        memory={{ ledger: null }}
      />
    );

    openUsageTab();

    expect(screen.queryByTestId("session-inspector-usage-grid")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-empty")).toHaveTextContent("No usage yet");
    expect(screen.queryByTestId("session-inspector-usage-turns")).not.toBeInTheDocument();
  });

  it("Should open the usage panel for a classification-only summary with no token counters", () => {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        usage={{ costStatus: "included", costSource: "none", turnCount: 0 }}
        memory={{ ledger: null }}
      />
    );

    openUsageTab();

    expect(screen.getByTestId("session-inspector-usage-grid")).toBeInTheDocument();
    expect(screen.queryByTestId("session-inspector-usage-empty")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent("Included");
    expect(screen.getByTestId("session-inspector-usage-tokens-in")).toHaveTextContent("—");
    expect(screen.queryByTestId("session-inspector-usage-turns")).not.toBeInTheDocument();
  });

  it("Should open the usage panel when only a positive turn count is reported", () => {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        usage={{ turnCount: 2 }}
        memory={{ ledger: null }}
      />
    );

    openUsageTab();

    expect(screen.getByTestId("session-inspector-usage-grid")).toBeInTheDocument();
    expect(screen.queryByTestId("session-inspector-usage-empty")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-turns")).toHaveTextContent("Across 2 turns");
    expect(screen.getByTestId("session-inspector-usage-cost")).toHaveTextContent("—");
  });

  it("Should keep the empty state when only a statusless cost amount is present", () => {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        usage={{ costUsd: 18.42, costCurrency: "USD", turnCount: 0 }}
        memory={{ ledger: null }}
      />
    );

    openUsageTab();

    expect(screen.queryByTestId("session-inspector-usage-grid")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-usage-empty")).toHaveTextContent("No usage yet");
  });
});

describe("SessionInspector — Usage tab cost provenance (W4)", () => {
  function renderUsage(usage: InspectorUsage) {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        usage={usage}
        memory={{ ledger: null }}
      />
    );
    openUsageTab();
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

describe("SessionInspector — Memory v2 forensic ledger surface", () => {
  it("Should render lineage meta and ledger events when the ledger is materialized", () => {
    const ledger = makeLedger();

    render(<SessionInspector messages={[]} sessionId="sess_123" memory={{ ledger }} />);

    openMemoryTab();

    const memorySurface = screen.getByTestId("session-inspector-memory");
    expect(memorySurface).toHaveAttribute("data-state", "ready");

    const meta = screen.getByTestId("session-inspector-memory-meta");
    expect(
      within(meta).getByTestId("session-inspector-memory-meta-workspace-value")
    ).toHaveTextContent("ws_alpha");
    expect(
      within(meta).getByTestId("session-inspector-memory-meta-root-session-value")
    ).toHaveTextContent("sess_root");
    expect(
      within(meta).getByTestId("session-inspector-memory-meta-parent-session-value")
    ).toHaveTextContent("sess_parent");
    expect(
      within(meta).getByTestId("session-inspector-memory-meta-spawn-depth-value")
    ).toHaveTextContent("2");
    expect(within(meta).getByTestId("session-inspector-memory-meta-path-value")).toHaveTextContent(
      "/sessions/ws_alpha/sess_123/ledger.jsonl"
    );
    expect(
      within(meta).getByTestId("session-inspector-memory-meta-checksum-value")
    ).toHaveTextContent("sha256:abc123");
    expect(
      within(meta).getByTestId("session-inspector-memory-meta-version-value")
    ).toHaveTextContent("v1");

    const eventsPanel = screen.getByTestId("session-inspector-memory-events");
    expect(within(eventsPanel).getByText("Ledger events")).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-memory-events-count")).toHaveTextContent("3");
    const rows = screen.getAllByTestId("session-inspector-memory-event-row");
    expect(rows).toHaveLength(3);
    expect(
      within(rows[0]!).getByTestId("session-inspector-memory-event-sequence")
    ).toHaveTextContent("#1");
    expect(within(rows[0]!).getByTestId("session-inspector-memory-event-type")).toHaveTextContent(
      "session.started"
    );
    expect(within(rows[1]!).getByTestId("session-inspector-memory-event-type")).toHaveTextContent(
      "memory.recall"
    );
    expect(within(rows[2]!).getByTestId("session-inspector-memory-event-type")).toHaveTextContent(
      "memory.event"
    );
  });

  it("Should label the events panel as ledger events even when no memory.* events are present", () => {
    const ledger = makeLedger({
      events: [
        { sequence: 1, event_type: "session.started", emitted_at: "2026-04-20T10:00:00Z" },
        { sequence: 2, event_type: "transcript.user", emitted_at: "2026-04-20T10:01:00Z" },
        { sequence: 3, event_type: "session.stopped", emitted_at: "2026-04-20T10:05:00Z" },
      ],
    });

    render(<SessionInspector messages={[]} sessionId="sess_123" memory={{ ledger }} />);

    openMemoryTab();

    const eventsPanel = screen.getByTestId("session-inspector-memory-events");
    expect(within(eventsPanel).getByText("Ledger events")).toBeInTheDocument();
    expect(within(eventsPanel).queryByText("Memory events")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-memory-events-count")).toHaveTextContent("3");
    const rows = screen.getAllByTestId("session-inspector-memory-event-row");
    expect(rows).toHaveLength(3);
    expect(within(rows[0]!).getByTestId("session-inspector-memory-event-type")).toHaveTextContent(
      "session.started"
    );
    expect(within(rows[1]!).getByTestId("session-inspector-memory-event-type")).toHaveTextContent(
      "transcript.user"
    );
    expect(within(rows[2]!).getByTestId("session-inspector-memory-event-type")).toHaveTextContent(
      "session.stopped"
    );
  });

  it("Should render a forensic-empty state when no ledger has materialized yet", () => {
    render(<SessionInspector messages={[]} sessionId="sess_123" memory={{ ledger: null }} />);

    openMemoryTab();

    const memorySurface = screen.getByTestId("session-inspector-memory");
    expect(memorySurface).toHaveAttribute("data-state", "unavailable");
    expect(screen.getByTestId("session-inspector-memory-empty")).toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-memory-empty")).toHaveTextContent(
      "No session ledger yet"
    );
  });

  it("Should treat 404 ledger errors as the truthful empty/unavailable state", () => {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        memory={{ ledger: null, error: new SessionLedgerUnavailableError("sess_123") }}
      />
    );

    openMemoryTab();

    const memorySurface = screen.getByTestId("session-inspector-memory");
    expect(memorySurface).toHaveAttribute("data-state", "unavailable");
    expect(screen.queryByTestId("session-inspector-memory-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-inspector-memory-empty")).toBeInTheDocument();
  });

  it("Should render a loading state while the ledger query resolves", () => {
    render(<SessionInspector messages={[]} sessionId="sess_123" memory={{ isLoading: true }} />);

    openMemoryTab();

    expect(screen.getByTestId("session-inspector-memory")).toHaveAttribute("data-state", "loading");
    expect(screen.getByTestId("session-inspector-memory-loading")).toBeInTheDocument();
  });

  it("Should render a forensic error state for non-404 ledger failures", () => {
    render(
      <SessionInspector
        messages={[]}
        sessionId="sess_123"
        memory={{ error: new Error("ledger materializer crashed") }}
      />
    );

    openMemoryTab();

    expect(screen.getByTestId("session-inspector-memory")).toHaveAttribute("data-state", "error");
    expect(screen.getByTestId("session-inspector-memory-error")).toHaveTextContent(
      "ledger materializer crashed"
    );
  });

  it("Should remain read-only and never expose editor, promote, or replay controls", () => {
    const ledger = makeLedger();

    render(<SessionInspector messages={[]} sessionId="sess_123" memory={{ ledger }} />);

    openMemoryTab();

    const memorySurface = screen.getByTestId("session-inspector-memory");
    expect(within(memorySurface).queryAllByRole("button")).toHaveLength(0);
    expect(within(memorySurface).queryAllByRole("textbox")).toHaveLength(0);
    expect(within(memorySurface).queryByText(/promote/i)).not.toBeInTheDocument();
    expect(within(memorySurface).queryByText(/replay/i)).not.toBeInTheDocument();
    expect(within(memorySurface).queryByText(/edit/i)).not.toBeInTheDocument();
  });

  it("Should render an event-empty state when the ledger has zero events", () => {
    const ledger = makeLedger({
      meta: {
        version: 1,
        session_id: "sess_x",
        spawn_depth: 0,
        path: "/p",
        checksum: "sha256:x",
        created_at: "2026-04-20T10:00:00Z",
      },
      events: [],
    });

    render(<SessionInspector messages={[]} sessionId="sess_x" memory={{ ledger }} />);

    openMemoryTab();

    const empty = screen.getByTestId("session-inspector-memory-events-empty");
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveTextContent("No ledger events");
    expect(screen.queryByTestId("session-inspector-memory-events-list")).not.toBeInTheDocument();
  });
});
