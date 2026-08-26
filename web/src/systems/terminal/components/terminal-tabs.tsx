import { ChevronRight, FileText, Plus, ScrollText, X } from "lucide-react";
import { useEffect, useId, useState } from "react";

import {
  Button,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";

import { terminalExitCopy } from "../lib/terminal-copy";
import type { TerminalInfo } from "../types";
import { TERMINAL_JOURNAL_TAB, type TerminalTabId } from "./terminal-tab-id";

export interface TerminalTabsProps {
  terminals: readonly TerminalInfo[];
  activeTab: TerminalTabId;
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

/** Tabs never shrink past legibility; the surplus collapses behind a caret. */
const VISIBLE_TAB_LIMIT = 5;

function terminalTabDomId(tabListId: string, tab: TerminalTabId): string {
  return `${tabListId}-${tab === TERMINAL_JOURNAL_TAB ? "journal" : tab}`;
}

/**
 * Which tabs are on the strip, and which are behind the caret.
 *
 * The strip keeps its order, with one exception: the terminal you are looking
 * at is always on it. Selecting one from the overflow and leaving it hidden
 * would show a pane no visible tab claims, and no tab marked selected at all —
 * so it takes the last visible slot and the tab it displaces joins the surplus.
 */
function splitTabs(
  terminals: readonly TerminalInfo[],
  activeTab: TerminalTabId
): { visible: TerminalInfo[]; overflow: TerminalInfo[] } {
  const visible = terminals.slice(0, VISIBLE_TAB_LIMIT);
  const overflow = terminals.slice(VISIBLE_TAB_LIMIT);
  const hiddenIndex = overflow.findIndex(terminal => terminal.id === activeTab);
  if (hiddenIndex < 0 || visible.length === 0) return { visible, overflow };
  const promoted = overflow[hiddenIndex];
  const displaced = visible[visible.length - 1];
  return {
    visible: [...visible.slice(0, -1), promoted],
    overflow: [...overflow.slice(0, hiddenIndex), displaced, ...overflow.slice(hiddenIndex + 1)],
  };
}

/**
 * The project's terminals, plus the pinned journal.
 *
 * State lives on the tab and nowhere else: a running dot, a warning dot only
 * while a question is waiting, the log glyph in place of the dot for a pipe
 * terminal, and an exited tab keeping its code as micro mono.
 */
export function TerminalTabs({
  terminals,
  activeTab,
  attentionIds,
  limit,
  onOpenTerminal,
  onSelect,
  onCloseTerminal,
  showOwner = false,
}: TerminalTabsProps) {
  const { visible, overflow } = splitTabs(terminals, activeTab);
  const tabListId = useId();
  const [focusRequest, setFocusRequest] = useState<{ tab: TerminalTabId } | null>(null);
  useEffect(() => {
    if (focusRequest === null) return;
    document.getElementById(terminalTabDomId(tabListId, focusRequest.tab))?.focus();
  }, [focusRequest, tabListId]);
  const atLimit = terminals.length >= limit;
  // Arrow keys move between tabs, as a tablist owes its users; the strip owns
  // the movement because only it knows the order, including the pinned journal.
  // Focus travels with the selection — the tab left behind becomes unreachable
  // by Tab the moment it stops being selected, so leaving focus on it would
  // strand the keyboard on an element nothing can return to.
  const order: TerminalTabId[] = [...visible.map(terminal => terminal.id), TERMINAL_JOURNAL_TAB];
  const moveSelection = (event: React.KeyboardEvent, from: TerminalTabId) => {
    const step = event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
    if (step === 0) return;
    const index = order.indexOf(from);
    if (index < 0) return;
    event.preventDefault();
    const next = order[(index + step + order.length) % order.length];
    onSelect(next);
    setFocusRequest({ tab: next });
  };
  return (
    <div
      aria-label="Terminal tabs"
      className="flex min-h-9 flex-none items-center gap-1 border-line border-b px-2"
      data-testid="terminal-tabs"
      role="tablist"
    >
      <div className="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
        {visible.map(terminal => (
          <TerminalTab
            active={activeTab === terminal.id}
            buttonId={terminalTabDomId(tabListId, terminal.id)}
            key={terminal.id}
            needsAttention={attentionIds?.has(terminal.id) ?? false}
            onClose={onCloseTerminal ? () => onCloseTerminal(terminal.id) : undefined}
            onKeyDown={event => moveSelection(event, terminal.id)}
            onSelect={() => onSelect(terminal.id)}
            terminal={terminal}
            showOwner={showOwner}
          />
        ))}
        {overflow.length > 0 ? (
          <TerminalTabOverflow
            attentionIds={attentionIds}
            onSelect={onSelect}
            showOwner={showOwner}
            terminals={overflow}
          />
        ) : null}
      </div>
      {onOpenTerminal ? (
        <Button
          aria-label="Open a new terminal"
          data-testid="terminal-open"
          onClick={onOpenTerminal}
          size="icon-xs"
          title={atLimit ? `${terminals.length} of ${limit} terminals` : "Open a new terminal"}
          type="button"
          variant="ghost"
        >
          <Plus aria-hidden="true" className="size-3" />
        </Button>
      ) : null}
      <button
        aria-selected={activeTab === TERMINAL_JOURNAL_TAB}
        className={cn(
          "flex h-7 flex-none items-center gap-1.5 rounded-md px-2.5 text-eyebrow",
          activeTab === TERMINAL_JOURNAL_TAB
            ? "bg-row-selected text-fg-strong"
            : "text-muted hover:bg-row-hover hover:text-fg"
        )}
        data-testid="terminal-tab-journal"
        id={terminalTabDomId(tabListId, TERMINAL_JOURNAL_TAB)}
        onClick={() => onSelect(TERMINAL_JOURNAL_TAB)}
        onKeyDown={event => moveSelection(event, TERMINAL_JOURNAL_TAB)}
        role="tab"
        tabIndex={activeTab === TERMINAL_JOURNAL_TAB ? 0 : -1}
        type="button"
      >
        <ScrollText aria-hidden="true" className="size-3" />
        Journal
      </button>
    </div>
  );
}

/**
 * One terminal in the strip.
 *
 * The element that selects is the tab: it carries `role="tab"`, its own
 * `aria-selected`, and roving focus. Closing is a separate control beside it
 * rather than inside it — a button nested in a tab is neither reachable nor
 * announceable, and it is what made the first shape mouse-only.
 */
function TerminalTab({
  terminal,
  active,
  buttonId,
  needsAttention,
  onSelect,
  onClose,
  onKeyDown,
  showOwner,
}: {
  terminal: TerminalInfo;
  active: boolean;
  buttonId: string;
  needsAttention: boolean;
  onSelect: () => void;
  onClose?: () => void;
  onKeyDown: (event: React.KeyboardEvent) => void;
  showOwner: boolean;
}) {
  const exit = terminal.exit ? terminalExitCopy(terminal.exit) : null;
  return (
    <div
      className={cn(
        "group flex h-7 min-w-24 max-w-40 flex-none items-center gap-1.5 rounded-md px-2.5 text-eyebrow",
        active ? "bg-row-selected text-fg-strong" : "text-muted hover:bg-row-hover hover:text-fg"
      )}
      data-mode={terminal.mode}
      data-testid={`terminal-tab-${terminal.id}`}
      role="presentation"
    >
      <button
        aria-selected={active}
        className="flex min-w-0 flex-1 items-center gap-1.5 text-left"
        data-testid={`terminal-tab-select-${terminal.id}`}
        id={buttonId}
        onClick={onSelect}
        onKeyDown={onKeyDown}
        role="tab"
        tabIndex={active ? 0 : -1}
        type="button"
      >
        {terminal.mode === "pipe" ? (
          <FileText aria-hidden="true" className="size-3 shrink-0 text-subtle" />
        ) : (
          <span
            aria-hidden="true"
            className={cn(
              "size-1.5 shrink-0 rounded-full",
              terminal.state === "running" ? "bg-success" : "bg-neutral"
            )}
            data-state={terminal.state}
          />
        )}
        <span className="truncate">
          {terminal.title}
          {showOwner ? ` · ${terminal.profile_name}` : ""}
        </span>
        {needsAttention ? (
          <span
            aria-label="Input requested"
            className="size-1.5 shrink-0 rounded-full bg-warning"
            data-testid={`terminal-tab-attention-${terminal.id}`}
          />
        ) : null}
        {exit && terminal.state === "exited" ? (
          <span className="shrink-0 font-mono text-micro text-subtle">{exit.code}</span>
        ) : null}
      </button>
      {onClose ? (
        <button
          aria-label={`Close ${terminal.title}`}
          className="shrink-0 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
          onClick={onClose}
          type="button"
        >
          <X aria-hidden="true" className="size-3" />
        </button>
      ) : null}
    </div>
  );
}

/**
 * The surplus at the cap.
 *
 * The strip collapses into one affordance rather than shrinking every tab past
 * the point where its name can be read.
 */
function TerminalTabOverflow({
  terminals,
  attentionIds,
  onSelect,
  showOwner,
}: {
  terminals: readonly TerminalInfo[];
  attentionIds?: ReadonlySet<string>;
  onSelect: (tab: TerminalTabId) => void;
  showOwner: boolean;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label={`${terminals.length} more terminals`}
            data-testid="terminal-tab-overflow"
            size="icon-xs"
            type="button"
            variant="ghost"
          />
        }
      >
        <ChevronRight aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        {terminals.map(terminal => (
          <DropdownMenuItem key={terminal.id} onClick={() => onSelect(terminal.id)}>
            {terminal.title}
            {showOwner ? ` · ${terminal.profile_name}` : ""}
            {attentionIds?.has(terminal.id) ? (
              <span
                aria-label="Input requested"
                className="ml-auto size-1.5 rounded-full bg-warning"
              />
            ) : null}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
