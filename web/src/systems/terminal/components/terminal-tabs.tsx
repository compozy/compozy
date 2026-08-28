import { FileText, Plus, ScrollText, X } from "lucide-react";
import { useState } from "react";

import { cn, Pill, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { terminalExitCopy } from "../lib/terminal-copy";
import type { TerminalInfo } from "../types";
import {
  TERMINAL_JOURNAL_TAB,
  terminalPanelDomId,
  terminalTabDomId,
  type TerminalTabId,
} from "./terminal-tab-id";

export interface TerminalTabsProps {
  terminals: readonly TerminalInfo[];
  activeTab: TerminalTabId;
  /** Shared id base wiring each tab to the panel the window renders. */
  idBase: string;
  /** Terminals with a question waiting, by id. */
  attentionIds?: ReadonlySet<string>;
  /** The project cap. Beyond it, opening is refused by the daemon. */
  limit: number;
  /** Absent on execute-only platforms — the option does not exist there. */
  onOpenTerminal?: () => void;
  onSelect: (tab: TerminalTabId) => void;
  onCloseTerminal?: (terminalId: string) => void;
  /** Labels mixed-profile rows with their owner. */
  showOwner?: boolean;
}

/** The deck-add recipe for the plus button. */
const DECK_CONTROL_CLASS =
  "mb-[calc((var(--height-deck-tab)-var(--size-deck-add))/2)] grid size-deck-add shrink-0 place-items-center rounded-menubar-control text-subtle transition-colors duration-base hover:bg-btn-default-fill hover:text-fg-strong focus-visible:shadow-focus-ring focus-visible:outline-none";

/**
 * The project's terminals as a switcher, plus the pinned journal.
 *
 * Every tab stays on the strip at min 96px until the per-workspace cap. The
 * daemon refuses create at the cap, so overflow never hides a live terminal.
 * Identity lives in the OS topbar; this strip only switches the visible pane.
 */
export function TerminalTabs({
  terminals,
  activeTab,
  idBase,
  attentionIds,
  limit,
  onOpenTerminal,
  onSelect,
  onCloseTerminal,
  showOwner = false,
}: TerminalTabsProps) {
  const [tabNodes] = useState(() => new Map<TerminalTabId, HTMLButtonElement>());
  const atLimit = terminals.length >= limit;
  // Arrow keys move between tabs, as a tablist owes its users; the strip owns
  // the movement because only it knows the order, including the pinned journal.
  // Focus travels with the selection in the same gesture — the tab left behind
  // becomes unreachable by Tab the moment it stops being selected, so leaving
  // focus on it would strand the keyboard on an element nothing can return to.
  const order: TerminalTabId[] = [...terminals.map(terminal => terminal.id), TERMINAL_JOURNAL_TAB];
  const moveSelection = (event: React.KeyboardEvent, from: TerminalTabId) => {
    const jump =
      event.key === "Home" ? order[0] : event.key === "End" ? order[order.length - 1] : null;
    const step = event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
    if (jump === null && step === 0) return;
    const index = order.indexOf(from);
    if (index < 0) return;
    event.preventDefault();
    const next = jump ?? order[(index + step + order.length) % order.length];
    onSelect(next);
    tabNodes.get(next)?.focus();
  };
  const registerTab = (tab: TerminalTabId) => (element: HTMLButtonElement | null) => {
    if (element === null) tabNodes.delete(tab);
    else tabNodes.set(tab, element);
  };
  const journalActive = activeTab === TERMINAL_JOURNAL_TAB;
  return (
    <div
      aria-label="Terminal tabs"
      className="flex h-deck flex-none items-end gap-0.5 bg-rail px-2.5 shadow-[inset_0_-1px_0_var(--color-line)] select-none"
      data-testid="terminal-tabs"
      role="tablist"
    >
      <div className="no-scrollbar flex min-w-0 items-end gap-0.5 overflow-x-auto">
        {terminals.map(terminal => (
          <TerminalTab
            active={activeTab === terminal.id}
            idBase={idBase}
            key={terminal.id}
            needsAttention={attentionIds?.has(terminal.id) ?? false}
            onClose={onCloseTerminal ? () => onCloseTerminal(terminal.id) : undefined}
            onKeyDown={event => moveSelection(event, terminal.id)}
            onSelect={() => onSelect(terminal.id)}
            refCallback={registerTab(terminal.id)}
            showOwner={showOwner}
            terminal={terminal}
          />
        ))}
      </div>
      {onOpenTerminal ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <button
                aria-label="Open a new terminal"
                className={DECK_CONTROL_CLASS}
                data-testid="terminal-open"
                onClick={onOpenTerminal}
                type="button"
              />
            }
          >
            <Plus aria-hidden="true" className="size-3" strokeWidth={1.5} />
          </TooltipTrigger>
          <TooltipContent side="bottom">
            {atLimit ? `${terminals.length} of ${limit} terminals` : "Open a new terminal"}
          </TooltipContent>
        </Tooltip>
      ) : null}
      <span aria-hidden="true" className="min-w-4 flex-1" />
      <button
        aria-controls={terminalPanelDomId(idBase)}
        aria-selected={journalActive}
        className={cn(
          "inline-flex h-deck-tab flex-none items-center gap-1.75 rounded-t-deck-tab border border-b-0 border-transparent px-2.5 text-small-body font-medium text-subtle transition-colors duration-base",
          journalActive
            ? "border-line bg-canvas font-semibold text-fg-strong shadow-[0_1px_0_var(--color-canvas)]"
            : "hover:bg-canvas-soft hover:text-fg",
          "focus-visible:shadow-focus-inset focus-visible:outline-none"
        )}
        data-testid="terminal-tab-journal"
        id={terminalTabDomId(idBase, TERMINAL_JOURNAL_TAB)}
        onClick={() => onSelect(TERMINAL_JOURNAL_TAB)}
        onKeyDown={event => moveSelection(event, TERMINAL_JOURNAL_TAB)}
        ref={registerTab(TERMINAL_JOURNAL_TAB)}
        role="tab"
        tabIndex={journalActive ? 0 : -1}
        type="button"
      >
        <ScrollText
          aria-hidden="true"
          className={cn(
            "size-deck-glyph shrink-0",
            journalActive ? "text-fg-strong" : "text-subtle"
          )}
        />
        Journal
      </button>
    </div>
  );
}

/**
 * One terminal in the deck.
 *
 * The element that selects is the tab: it carries `role="tab"`, its own
 * `aria-selected`, and roving focus. Closing is a separate control beside it
 * rather than inside it — a button nested in a tab is neither reachable nor
 * announceable, and it is what made the first shape mouse-only.
 */
function TerminalTab({
  terminal,
  active,
  idBase,
  needsAttention,
  onSelect,
  onClose,
  onKeyDown,
  refCallback,
  showOwner,
}: {
  terminal: TerminalInfo;
  active: boolean;
  idBase: string;
  needsAttention: boolean;
  onSelect: () => void;
  onClose?: () => void;
  onKeyDown: (event: React.KeyboardEvent) => void;
  refCallback: (element: HTMLButtonElement | null) => void;
  showOwner: boolean;
}) {
  const exit = terminal.exit ? terminalExitCopy(terminal.exit) : null;
  const running = terminal.state === "running";
  return (
    <div
      className={cn(
        "group/tab relative inline-flex h-deck-tab w-deck-tab-max min-w-deck-tab items-center rounded-t-deck-tab border border-b-0 border-transparent text-small-body font-medium text-subtle transition-colors duration-base select-none",
        active
          ? "border-line bg-canvas font-semibold text-fg-strong shadow-[0_1px_0_var(--color-canvas)]"
          : "hover:bg-canvas-soft hover:text-fg",
        "focus-within:shadow-focus-inset"
      )}
      data-mode={terminal.mode}
      data-testid={`terminal-tab-${terminal.id}`}
      role="presentation"
    >
      <button
        aria-controls={terminalPanelDomId(idBase)}
        aria-selected={active}
        className="flex min-w-0 flex-1 cursor-pointer items-center gap-1.75 rounded-[inherit] px-2.5 text-left focus-visible:shadow-focus-inset focus-visible:outline-none"
        data-testid={`terminal-tab-select-${terminal.id}`}
        id={terminalTabDomId(idBase, terminal.id)}
        onClick={onSelect}
        onKeyDown={onKeyDown}
        ref={refCallback}
        role="tab"
        tabIndex={active ? 0 : -1}
        type="button"
      >
        {terminal.mode === "pipe" ? (
          <FileText aria-hidden="true" className="size-deck-glyph shrink-0 text-subtle" />
        ) : (
          <Pill.Dot
            aria-hidden={undefined}
            aria-label={running ? "Running" : "Not running"}
            className="shrink-0"
            data-state={terminal.state}
            pulse={running}
            role="img"
            size="sm"
            tone={running ? "success" : "neutral"}
          />
        )}
        <span className="min-w-0 flex-1 truncate">
          {terminal.title}
          {showOwner ? ` · ${terminal.profile_name}` : ""}
        </span>
        {needsAttention ? (
          <Pill.Dot
            aria-hidden={undefined}
            aria-label="Input requested"
            className="shrink-0"
            data-testid={`terminal-tab-attention-${terminal.id}`}
            role="img"
            size="sm"
            tone="warning"
          />
        ) : null}
        {exit && terminal.state === "exited" ? (
          <span className="shrink-0 font-mono text-micro text-subtle">{exit.code}</span>
        ) : null}
      </button>
      {onClose ? (
        <button
          aria-label={`Close ${terminal.title}`}
          className={cn(
            // The visible glyph stays 16px; the pseudo-element grows the hit
            // area to the 24px floor without widening the deck tab.
            "relative mr-1.5 grid size-4 shrink-0 place-items-center rounded-xs text-faint opacity-0 transition-opacity duration-base after:absolute after:-inset-1 after:content-['']",
            "hover:bg-btn-default-fill hover:text-fg-strong focus-visible:opacity-100 focus-visible:shadow-focus-ring focus-visible:outline-none",
            "group-hover/tab:opacity-100",
            active && "opacity-100"
          )}
          onClick={onClose}
          type="button"
        >
          <X aria-hidden="true" className="size-2.25" strokeWidth={1.4} />
        </button>
      ) : null}
    </div>
  );
}
