import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { JOURNAL_FIXTURES } from "../../mocks/terminal-fixtures";
import {
  terminalJournalChipsFromFilters,
  terminalJournalFiltersFromChips,
} from "../../lib/terminal-journal-filter-fields";
import type { TerminalJournalEntry } from "../../types";
import { TerminalJournalPanel } from "../terminal-journal-panel";

/**
 * Canonical suite for the journal panel (UT-116).
 *
 * Invariant: filters compose into the query identity, an approximate row is
 * marked approximate, cursor paging appends without duplicating rows, and the
 * two empty states say different things because they are different facts.
 */

function renderPanel(overrides: Partial<Parameters<typeof TerminalJournalPanel>[0]> = {}) {
  const props = {
    entries: JOURNAL_FIXTURES,
    chips: [],
    hasMore: true,
    onFiltersChange: vi.fn(),
    onLoadMore: vi.fn(),
    ...overrides,
  };
  return { ...render(<TerminalJournalPanel {...props} />), props };
}

describe("TerminalJournalPanel", () => {
  it("Should read each command as the row, with where it ran demoted beneath", () => {
    renderPanel();

    const row = screen.getByTestId("terminal-journal-row-cmd-5f0a1e");
    expect(within(row).getByText("make gate")).toBeInTheDocument();
    expect(within(row).getByText(/~\/dev\/atlas-api/)).toBeInTheDocument();
    expect(within(row).getByText("Succeeded")).toBeInTheDocument();
    expect(within(row).getByText("exit 0")).toBeInTheDocument();
  });

  it("Should mark a heuristic boundary as estimated and leave certainty quiet", () => {
    renderPanel();

    expect(screen.getByTestId("terminal-journal-confidence-cmd-1e8f7a")).toHaveTextContent(
      "estimated"
    );
    expect(screen.getByTestId("terminal-journal-confidence-cmd-77c1d0")).toHaveTextContent("exact");
    expect(screen.getByTestId("terminal-journal-confidence-cmd-5f0a1e")).toHaveTextContent(
      "verified"
    );
  });

  it("Should keep a non-zero exit neutral and an unverifiable cause unknown", () => {
    renderPanel();

    const failed = screen.getByTestId("terminal-journal-row-cmd-4aa01f");
    expect(within(failed).getByText("Finished with errors")).toBeInTheDocument();
    expect(within(failed).getByText("exit 1")).toBeInTheDocument();

    const unknown = screen.getByTestId("terminal-journal-row-cmd-8be44d");
    expect(within(unknown).getByText("Ended")).toBeInTheDocument();
    expect(within(unknown).getByText("cause unknown")).toBeInTheDocument();
  });

  it("Should state rows loaded rather than a total nothing counted", () => {
    renderPanel();

    expect(screen.getByTestId("terminal-journal-loaded")).toHaveTextContent(
      `${JOURNAL_FIXTURES.length} rows loaded · newest first`
    );
  });

  it("Should append an older page without duplicating what is already loaded", () => {
    const older: TerminalJournalEntry = {
      ...JOURNAL_FIXTURES[0],
      command_id: "cmd-older01",
      command: "bun install",
      started_at: "2026-08-25T12:10:00Z",
    };
    const { rerender, props } = renderPanel();

    expect(screen.getAllByTestId(/^terminal-journal-row-/)).toHaveLength(JOURNAL_FIXTURES.length);

    rerender(
      <TerminalJournalPanel {...props} entries={[...JOURNAL_FIXTURES, older]} hasMore={false} />
    );

    const rows = screen.getAllByTestId(/^terminal-journal-row-/);
    expect(rows).toHaveLength(JOURNAL_FIXTURES.length + 1);
    expect(new Set(rows.map(row => row.dataset.testid)).size).toBe(JOURNAL_FIXTURES.length + 1);
    expect(screen.getByTestId("terminal-journal-row-cmd-older01")).toBeInTheDocument();
    // With no cursor left, the panel stops offering a page that does not exist.
    expect(screen.queryByTestId("terminal-journal-load-more")).not.toBeInTheDocument();
  });

  it("Should ask for the next page only through the cursor affordance", async () => {
    const { props } = renderPanel();

    await userEvent.click(screen.getByTestId("terminal-journal-load-more"));

    expect(props.onLoadMore).toHaveBeenCalledOnce();
  });

  it("Should say nothing has run yet when the journal is genuinely empty", async () => {
    const onOpenNewTerminal = vi.fn();
    renderPanel({ entries: [], chips: [], hasMore: false, onOpenNewTerminal });

    expect(screen.getByTestId("terminal-journal-empty")).toHaveTextContent(
      "Nothing has run here yet"
    );
    expect(screen.queryByTestId("terminal-journal-filtered-empty")).not.toBeInTheDocument();
    // Nothing has run because nothing is running: the offer is a terminal.
    await userEvent.click(screen.getByTestId("terminal-journal-empty-open"));
    expect(onOpenNewTerminal).toHaveBeenCalledOnce();
  });

  it("Should scope a filtered miss to the rows actually loaded", async () => {
    const { props } = renderPanel({
      entries: [],
      chips: [
        { id: "result", field: "result", operator: "is", values: ["failed"] },
        { id: "actor", field: "actor", operator: "is", values: ["human"] },
      ],
    });

    const empty = screen.getByTestId("terminal-journal-filtered-empty");
    expect(empty).toHaveTextContent("No matches in the rows loaded");
    expect(empty).toHaveTextContent("a match may still be further back");

    await userEvent.click(within(empty).getByText("Clear filters"));
    expect(props.onFiltersChange).toHaveBeenCalledWith([]);
  });

  it("Should render active chips and route their edits through one change handler", () => {
    renderPanel({
      chips: [{ id: "actor", field: "actor", operator: "is", values: ["agent"] }],
    });

    // The chip is the Filters primitive's: field label, operator, and value.
    // "Who" also heads the table column, so the chip is identified by its value.
    expect(screen.getByText("An agent")).toBeInTheDocument();
    expect(screen.getAllByText("Who")).toHaveLength(2);
    expect(screen.getByTestId("terminal-journal-filters-add")).toBeInTheDocument();
  });

  it("Should open the whole record for a row and fold the attribute columns away", async () => {
    renderPanel();

    await userEvent.click(screen.getByTestId("terminal-journal-row-cmd-77c1d0"));

    const detail = screen.getByTestId("terminal-journal-detail");
    expect(
      within(detail).getByText("psql -h staging.internal -U atlas atlas_api")
    ).toBeInTheDocument();
    expect(within(detail).getByText("approved by you")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-journal-confidence-cmd-77c1d0")).not.toBeInTheDocument();
  });

  it("Should label each row's owning profile only in the all-profiles view", () => {
    const scoped = renderPanel();
    expect(screen.queryByTestId("terminal-journal-owner-cmd-5f0a1e")).not.toBeInTheDocument();
    scoped.unmount();

    renderPanel({ showOwner: true });
    expect(screen.getByTestId("terminal-journal-owner-cmd-5f0a1e")).toHaveTextContent("work");
  });

  it("Should compose chips into one query input and drop values still being typed", () => {
    expect(
      terminalJournalFiltersFromChips([
        { id: "actor", field: "actor", operator: "is", values: ["agent"] },
        { id: "result", field: "result", operator: "is", values: ["failed"] },
        { id: "since", field: "since", operator: "is", values: ["24h"] },
        { id: "terminal", field: "terminal", operator: "is", values: ["term-7"] },
        // A chip whose value is still empty filters nothing yet.
        { id: "half", field: "terminal", operator: "is", values: [""] },
      ])
    ).toEqual({ actor: "agent", failed: true, since: "24h", terminalId: "term-7" });
  });

  it("Should round-trip filters through chips without inventing or losing a fact", () => {
    const filters = { actor: "human" as const, failed: true, since: "1h", terminalId: "term-8" };
    expect(terminalJournalFiltersFromChips(terminalJournalChipsFromFilters(filters))).toEqual(
      filters
    );
    expect(terminalJournalChipsFromFilters({})).toEqual([]);
  });
});
