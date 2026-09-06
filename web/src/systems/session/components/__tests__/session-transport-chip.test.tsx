import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { SessionTransportContext } from "../../lib/session-transcript-thread-context-value";
import {
  SESSION_TRANSPORT_GRACE_MS,
  SESSION_TRANSPORT_LIVE,
  type SessionTransportSnapshot,
} from "../../lib/session-transport";
import { SessionTransportChip } from "../session-transport-chip";
import {
  SessionTransportFailureNotice,
  SessionTransportHistoryResetNotice,
} from "../session-transport-notices";

// Suite: SessionTransportChip + transport notices (S4).
// Invariant: the chip mounts only once a degraded phase outlasts the 2s grace (UT-103), reads
// the attempt count from the store, and the notices state a dead stream / a history reset
// with the store's own numbers; Try again is the store's manual recovery.
// Owning layer: session transport presentation. Canonical suite: this file.
// Boundary IN: SessionTransportContext. Boundary OUT: window head slot, thread rail.
function renderWithTransport(
  snapshot: Partial<SessionTransportSnapshot>,
  ui: React.ReactNode,
  retry = vi.fn()
) {
  return render(
    <UIProvider>
      <SessionTransportContext.Provider
        value={{ ...SESSION_TRANSPORT_LIVE, lastLiveAt: 1_000, ...snapshot, retry }}
      >
        {ui}
      </SessionTransportContext.Provider>
    </UIProvider>
  );
}

describe("SessionTransportChip", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should mount the reconnecting chip only after the 2s grace, with the attempt count (UT-103)", () => {
    vi.useFakeTimers();
    vi.setSystemTime(10_000);
    renderWithTransport(
      { degradedAt: 10_000, phase: "waiting-reconnect", reconnectAttempt: 3 },
      <SessionTransportChip windowLive />
    );
    expect(screen.queryByTestId("session-transport-chip")).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(SESSION_TRANSPORT_GRACE_MS - 1);
    });
    expect(screen.queryByTestId("session-transport-chip")).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    const chip = screen.getByTestId("session-transport-chip");
    expect(chip).toHaveAttribute("data-phase", "reconnecting");
    expect(chip).toHaveAttribute("role", "status");
    expect(chip).toHaveTextContent("Reconnecting");
    expect(screen.getByTestId("session-transport-chip-count")).toHaveTextContent("·3");
  });

  it("Should stay absent for a healthy stream and read paused in a background window", () => {
    const { rerender } = renderWithTransport({}, <SessionTransportChip windowLive />);
    expect(screen.queryByTestId("session-transport-chip")).not.toBeInTheDocument();

    rerender(
      <UIProvider>
        <SessionTransportContext.Provider
          value={{ ...SESSION_TRANSPORT_LIVE, lastLiveAt: 1_000, retry: vi.fn() }}
        >
          <SessionTransportChip windowLive={false} />
        </SessionTransportContext.Provider>
      </UIProvider>
    );
    const chip = screen.getByTestId("session-transport-chip");
    expect(chip).toHaveAttribute("data-phase", "paused");
    expect(chip).toHaveTextContent("Paused");
  });

  it("Should read disconnected at once when retries ran out and offer Try again in the notice", () => {
    const retry = vi.fn();
    renderWithTransport(
      { degradedAt: 1_000, failure: { at: 9_000, attempts: 6 }, phase: "failed" },
      <>
        <SessionTransportChip windowLive />
        <SessionTransportFailureNotice />
      </>,
      retry
    );
    expect(screen.getByTestId("session-transport-chip")).toHaveAttribute(
      "data-phase",
      "disconnected"
    );
    const notice = screen.getByTestId("session-transport-failure");
    expect(notice).toHaveAttribute("role", "alert");
    expect(notice).toHaveTextContent("couldn't reconnect after 6 tries");
    screen.getByTestId("session-transport-failure-retry").click();
    expect(retry).toHaveBeenCalledOnce();
  });

  it("Should state a history reset with its generation and nothing otherwise", () => {
    const { rerender } = renderWithTransport({}, <SessionTransportHistoryResetNotice />);
    expect(screen.queryByTestId("session-transport-history-reset")).not.toBeInTheDocument();
    rerender(
      <UIProvider>
        <SessionTransportContext.Provider
          value={{
            ...SESSION_TRANSPORT_LIVE,
            historyReset: { at: 2_000, generation: 4, reason: "generation_mismatch" },
            retry: vi.fn(),
          }}
        >
          <SessionTransportHistoryResetNotice />
        </SessionTransportContext.Provider>
      </UIProvider>
    );
    const notice = screen.getByTestId("session-transport-history-reset");
    expect(notice).toHaveAttribute("role", "status");
    expect(notice).toHaveTextContent("The conversation history changed while you were away");
    expect(screen.getByTestId("session-transport-history-reset-generation")).toHaveTextContent(
      "generation 4 · generation_mismatch"
    );
  });

  it("Should explain a retention reset as older history no longer retained (US-017.EC-1)", () => {
    renderWithTransport(
      { historyReset: { at: 2_000, generation: 1, reason: "cursor_expired" } },
      <SessionTransportHistoryResetNotice />
    );
    const notice = screen.getByTestId("session-transport-history-reset");
    expect(notice).toHaveAttribute("data-reset-reason", "cursor_expired");
    expect(notice).toHaveTextContent("Older history is no longer retained");
    expect(notice).not.toHaveTextContent(/rewound|compact/);
  });
});
