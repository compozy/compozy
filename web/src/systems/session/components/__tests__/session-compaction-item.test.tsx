import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { SessionCompactionItemData } from "../../types";
import { SessionCompactionItem } from "../session-compaction-item";

// Suite: Compaction timeline item (S18, UT-W10).
// Invariant: the row tells the agent's own compaction status truthfully — a live
// spinner only while `in_progress`, one sentence per terminal status, the agent's
// error on failure, the summary behind a disclosure only when the agent sent one,
// and a vendor status verbatim without ever borrowing a terminal sentence.
// Boundary IN: SessionCompactionItem presentation from a typed item.
// Boundary OUT: transcript projection and thread wiring (session-thread.test.tsx).

const TERMINAL_SENTENCES = [
  "Compacting context…",
  "Context compacted",
  "Context compaction failed",
  "Context compaction cancelled",
];

function item(overrides: Partial<SessionCompactionItemData>): SessionCompactionItemData {
  return {
    kind: "compaction",
    compaction_id: "c1f0b8f4",
    status: "completed",
    started_at: "2026-10-08T14:02:11Z",
    ...overrides,
  };
}

function renderRow(data: SessionCompactionItemData) {
  render(<SessionCompactionItem item={data} />);
  return screen.getByTestId("session-compaction-item");
}

describe("SessionCompactionItem", () => {
  it("Should show a live spinner and the in-progress sentence while the agent compacts", () => {
    const row = renderRow(item({ status: "in_progress" }));

    expect(row).toHaveAttribute("data-status", "in_progress");
    expect(row).toHaveTextContent("Compacting context…");
    expect(
      within(screen.getByRole("status")).getByTestId("session-compaction-spinner")
    ).toBeInTheDocument();
    expect(screen.queryByTestId("session-compaction-summary")).not.toBeInTheDocument();
  });

  it("Should keep the summary behind a closed disclosure and render it as markdown when opened", async () => {
    const user = userEvent.setup();
    const row = renderRow(
      item({
        summary: "Moved invoices to the ledger API:\n\n- invoices\n- refunds",
        ended_at: "2026-10-08T14:02:39Z",
      })
    );

    expect(row).toHaveAttribute("data-status", "completed");
    expect(row).toHaveTextContent("Context compacted");
    expect(screen.queryByTestId("session-compaction-spinner")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-compaction-summary-content")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Summary" }));

    const content = screen.getByTestId("session-compaction-summary-content");
    expect(content).toHaveTextContent("Moved invoices to the ledger API:");
    expect(content.querySelectorAll("li")).toHaveLength(2);
    expect(screen.getByTestId("session-compaction-summary")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Summary" }));
    expect(screen.queryByTestId("session-compaction-summary-content")).not.toBeInTheDocument();
  });

  it("Should show only the sentence, with no disclosure, when the agent sent no summary", () => {
    const row = renderRow(item({ status: "completed", summary: undefined }));

    expect(row).toHaveTextContent("Context compacted");
    expect(screen.queryByRole("button", { name: "Summary" })).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-compaction-summary")).not.toBeInTheDocument();
  });

  it("Should not offer a disclosure for a blank summary", () => {
    renderRow(item({ summary: "  \n " }));

    expect(screen.queryByTestId("session-compaction-summary")).not.toBeInTheDocument();
  });

  it("Should name the agent's error on a failed compaction and announce it as an alert", () => {
    const row = renderRow(
      item({ status: "failed", error: "context window exceeded", ended_at: "2026-10-08T14:02:12Z" })
    );

    expect(row).toHaveAttribute("data-status", "failed");
    expect(row).toHaveTextContent("Context compaction failed");
    expect(screen.getByTestId("session-compaction-error")).toHaveTextContent(
      "context window exceeded"
    );
    expect(within(screen.getByRole("alert")).getByTestId("session-compaction-error")).toBeVisible();
    expect(screen.getByRole("alert")).toHaveAttribute("data-tone", "danger");
  });

  it("Should read a cancelled compaction as neutral, never as a failure", () => {
    const row = renderRow(item({ status: "cancelled", ended_at: "2026-10-08T14:02:12Z" }));

    expect(row).toHaveAttribute("data-status", "cancelled");
    expect(row).toHaveTextContent("Context compaction cancelled");
    expect(screen.getByRole("status")).toHaveAttribute("data-tone", "neutral");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("Should show a vendor status verbatim and treat it as not finished", async () => {
    const user = userEvent.setup();
    const row = renderRow(
      item({ status: "compaction_paused", summary: "Paused after the first pass." })
    );

    expect(row).toHaveAttribute("data-status", "compaction_paused");
    expect(row).toHaveTextContent("compaction_paused");
    for (const sentence of TERMINAL_SENTENCES) {
      expect(row).not.toHaveTextContent(sentence);
    }
    expect(screen.getByRole("status")).toHaveAttribute("data-tone", "neutral");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-compaction-spinner")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Summary" }));
    expect(screen.getByTestId("session-compaction-summary-content")).toHaveTextContent(
      "Paused after the first pass."
    );
  });
});
