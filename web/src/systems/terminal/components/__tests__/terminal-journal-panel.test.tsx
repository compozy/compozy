import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  terminalJournalChipsFromFilters,
  terminalJournalFiltersFromChips,
} from "../../lib/terminal-journal-filter-fields";
import { shouldKeepTerminalJournalHost } from "../../lib/terminal-journal-host";
import { JOURNAL_FIXTURES } from "../../mocks/terminal-fixtures";
import type { TerminalJournalEntry } from "../../types";
import { TerminalJournalPanel } from "../terminal-journal-panel";

const ORIGINAL_MATCH_MEDIA = window.matchMedia;

function installLayout(inline: boolean): void {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: inline && query.includes("min-width"),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

/**
 * Canonical suite for the journal panel (UT-116).
 *
 * Invariant: filters compose into the query identity, an approximate row is
 * marked estimated, cursor paging appends without duplicating rows, the two
 * empty states say different things, a filtered miss names N only when the
 * host counted, Stopped/Ended stay hollow, and selection is an elevated plate.
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
  beforeEach(() => {
    installLayout(true);
  });

  afterEach(() => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: ORIGINAL_MATCH_MEDIA,
    });
  });

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

  it("Should omit N on a filtered miss until the host counts the examined rows", async () => {
    const { props } = renderPanel({
      entries: [],
      chips: [
        { id: "result", field: "result", operator: "is", values: ["failed"] },
        { id: "actor", field: "actor", operator: "is", values: ["human"] },
      ],
    });

    const empty = screen.getByTestId("terminal-journal-filtered-empty");
    expect(empty).toHaveTextContent("No matches in the rows loaded");
    expect(empty).not.toHaveTextContent("50");
    expect(empty).toHaveTextContent("a match may still be further back");
    expect(within(empty).getByRole("button", { name: "Load older rows" })).toBeInTheDocument();
    expect(screen.getByTestId("terminal-journal-loaded")).toHaveTextContent("2 filters");
    expect(screen.getByTestId("terminal-journal-loaded")).not.toHaveTextContent("rows loaded");
    expect(screen.queryByTestId("terminal-journal-load-more")).not.toBeInTheDocument();

    await userEvent.click(within(empty).getByText("Clear filters"));
    expect(props.onFiltersChange).toHaveBeenCalledWith([]);
  });

  it("Should name the examined count only when the host supplies it", () => {
    renderPanel({
      chips: [{ id: "result", field: "result", operator: "is", values: ["failed"] }],
      entries: [],
      examinedCount: 50,
    });

    expect(screen.getByTestId("terminal-journal-filtered-empty")).toHaveTextContent(
      "No matches in the 50 rows loaded"
    );
    expect(screen.getByTestId("terminal-journal-loaded")).toHaveTextContent(
      "50 rows loaded · 1 filter"
    );
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

  it("Should keep the table mounted and the detail closed when the host passes a replay", () => {
    renderPanel({
      replay: <div data-testid="host-replay">replay</div>,
      selectedCommandId: "cmd-5f0a1e",
    });

    expect(screen.getByTestId("terminal-journal-replay")).toBeInTheDocument();
    expect(screen.getByTestId("host-replay")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-journal-row-cmd-5f0a1e")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-journal-detail")).not.toBeInTheDocument();
  });

  it("Should restore the selected record when the host clears replay and keeps the id", () => {
    const { rerender, props } = renderPanel({
      replay: <div data-testid="host-replay">replay</div>,
      selectedCommandId: "cmd-77c1d0",
    });

    expect(screen.getByTestId("terminal-journal-replay")).toBeInTheDocument();
    rerender(<TerminalJournalPanel {...props} replay={undefined} selectedCommandId="cmd-77c1d0" />);

    expect(screen.queryByTestId("terminal-journal-replay")).not.toBeInTheDocument();
    expect(screen.getByTestId("terminal-journal-detail")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-journal-row-cmd-77c1d0")).toHaveAttribute(
      "data-selected",
      ""
    );
  });

  it("Should open the whole record for a row and fold the attribute columns away", async () => {
    renderPanel();

    await userEvent.click(screen.getByTestId("terminal-journal-row-cmd-77c1d0"));

    const row = screen.getByTestId("terminal-journal-row-cmd-77c1d0");
    expect(row).toHaveAttribute("data-state", "selected");

    const detail = screen.getByTestId("terminal-journal-detail");
    expect(
      within(detail).getByText("psql -h staging.internal -U atlas atlas_api")
    ).toBeInTheDocument();
    expect(within(detail).getByText("approved by you")).toBeInTheDocument();
    expect(within(detail).getByTestId("terminal-journal-output")).toHaveTextContent("4,096 bytes");
    expect(within(detail).queryByText(/hidden input/i)).not.toBeInTheDocument();
    expect(within(detail).queryByText(/characters/i)).not.toBeInTheDocument();
    expect(screen.queryByTestId("terminal-journal-confidence-cmd-77c1d0")).not.toBeInTheDocument();
  });

  it("Should plate stopped and ended as hollow, and leave a failed exit filled", () => {
    renderPanel();

    const stopped = within(screen.getByTestId("terminal-journal-row-cmd-2c8de1")).getByText(
      "Stopped"
    );
    expect(stopped).toHaveAttribute("data-form", "hollow");

    const ended = within(screen.getByTestId("terminal-journal-row-cmd-8be44d")).getByText("Ended");
    expect(ended).toHaveAttribute("data-form", "hollow");

    const failed = within(screen.getByTestId("terminal-journal-row-cmd-4aa01f")).getByText(
      "Finished with errors"
    );
    expect(failed).toHaveAttribute("data-form", "tint");
  });

  it("Should copy the recorded command from the rail", async () => {
    const onCopyCommand = vi.fn();
    renderPanel({ onCopyCommand });

    await userEvent.click(screen.getByTestId("terminal-journal-row-cmd-77c1d0"));
    await userEvent.click(screen.getByTestId("terminal-journal-copy-command"));

    expect(onCopyCommand).toHaveBeenCalledExactlyOnceWith(
      "psql -h staging.internal -U atlas atlas_api"
    );
  });

  it("Should show a kind label when the actor id is only a machine identifier", () => {
    const uuidEntry: TerminalJournalEntry = {
      ...JOURNAL_FIXTURES[0],
      command_id: "cmd-actor-uuid",
      actor: { kind: "agent", id: "3d2c1b0a-9f8e-4d7c-a6b5-4e3d2c1b0a9f" },
    };
    renderPanel({ entries: [uuidEntry] });

    expect(
      within(screen.getByTestId("terminal-journal-row-cmd-actor-uuid")).getByText("An agent")
    ).toBeInTheDocument();
  });

  it("Should keep the table and footer visible when replay is shown inside the journal", () => {
    renderPanel({
      replay: <div data-testid="terminal-journal-replay-slot">replay</div>,
    });

    expect(screen.getByTestId("terminal-journal-filters-add")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-journal-replay-slot")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-journal-row-cmd-5f0a1e")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-journal-loaded")).toBeInTheDocument();
  });

  it("Should keep attribute columns and one named drawer close below 1440", async () => {
    installLayout(false);
    renderPanel();

    await userEvent.click(screen.getByTestId("terminal-journal-row-cmd-77c1d0"));

    const table = screen.getByRole("table", { hidden: true, name: "Command journal" });
    expect(within(table).getByText("Permission")).toBeInTheDocument();
    expect(within(table).getByTestId("terminal-journal-confidence-cmd-5f0a1e")).toBeInTheDocument();

    const drawer = await screen.findByRole("dialog", { name: "Command record" });
    expect(within(drawer).getAllByRole("button", { name: /close/i })).toHaveLength(1);
    expect(screen.queryByRole("button", { name: "Close record" })).not.toBeInTheDocument();
  });

  it("Should tell the host to keep the journal when the last terminal closes", () => {
    expect(shouldKeepTerminalJournalHost({ remainingTerminalCount: 0, journalVisible: true })).toBe(
      true
    );
    expect(
      shouldKeepTerminalJournalHost({ remainingTerminalCount: 0, journalVisible: false })
    ).toBe(false);
    expect(
      shouldKeepTerminalJournalHost({ remainingTerminalCount: 1, journalVisible: false })
    ).toBe(true);
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
