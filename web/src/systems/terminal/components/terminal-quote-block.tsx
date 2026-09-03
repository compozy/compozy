import { Copy, MessagesSquare, Plus, TerminalSquare, X } from "lucide-react";

import { Button, Eyebrow, cn } from "@compozy/ui";

import { copySourcedTerminalQuote, type TerminalQuote } from "../lib/terminal-quote";
import { holdPendingTerminalQuote } from "../lib/terminal-quote-pending";

export interface TerminalQuoteBlockProps {
  quote: TerminalQuote;
  onRemove: () => void;
}

/**
 * A terminal excerpt waiting in the composer.
 *
 * It carries where it came from and states the one thing that can go stale:
 * line numbers are scrollback-relative and shift as old output is trimmed.
 */
export function TerminalQuoteBlock({ quote, onRemove }: TerminalQuoteBlockProps) {
  return (
    <div
      className="tm-quote overflow-hidden rounded-xs border border-line border-l-2 border-l-line-strong"
      data-testid="terminal-quote-block"
    >
      <div className="flex min-h-6 items-center gap-1.75 bg-canvas-tint px-2.25 font-mono text-micro text-subtle">
        <TerminalSquare aria-hidden="true" className="size-2.5" />
        <span>
          {quote.terminalId} · lines {quote.fromLine}–{quote.toLine}
        </span>
        <Button
          aria-label="Remove quote"
          className="ml-auto size-5"
          onClick={onRemove}
          size="icon-xs"
          type="button"
          variant="ghost"
        >
          <X aria-hidden="true" className="size-2.5" />
        </Button>
      </div>
      {/* A quoted excerpt sits below the live grid's 12.5px — never at it. */}
      <div className="bg-chat-fill-code px-2.25 py-1.5 font-mono text-badge leading-normal text-fg">
        {quote.lines.map((line, index) => (
          <span className="block" key={`${quote.fromLine + index}`}>
            <span aria-hidden="true" className="mr-2.5 text-faint select-none">
              {quote.fromLine + index}
            </span>
            {line}
          </span>
        ))}
      </div>
      <div className="bg-canvas-tint px-2.25 pt-0.75 pb-1.25 text-micro text-faint">
        Line numbers can shift as old output is trimmed.
      </div>
    </div>
  );
}

export interface TerminalSelectionActionsProps {
  /** Absent when no session is active — the gesture then offers a way in. */
  hasActiveSession: boolean;
  /**
   * When present, Copy writes the sourced block and Start holds it for
   * create. Choose passes the quote to the host picker — it must not occupy
   * the create-only pending slot. The window owner should pass the quote
   * built from the current selection.
   */
  quote?: TerminalQuote;
  onSendToConversation: () => void;
  onChooseSession: () => void;
  onStartSession: () => void;
  onCopy: () => void;
}

/**
 * What a selection can become.
 *
 * With no active session the gesture never dead-ends: it offers picking or
 * starting one, and copying the same sourced block is always the fallback.
 */
export function TerminalSelectionActions({
  hasActiveSession,
  quote,
  onSendToConversation,
  onChooseSession,
  onStartSession,
  onCopy,
}: TerminalSelectionActionsProps) {
  const handleCopy = () => {
    if (quote) {
      void copySourcedTerminalQuote(quote.terminalId, {
        startLine: quote.fromLine,
        text: quote.lines.join("\n"),
      });
      return;
    }
    onCopy();
  };
  const handleChoose = () => {
    onChooseSession();
  };
  const handleStart = () => {
    if (quote) holdPendingTerminalQuote(quote);
    onStartSession();
  };

  if (hasActiveSession) {
    return (
      <div
        aria-label="Selection actions"
        className="relative z-10 min-w-36 flex-none rounded-lg bg-canvas-soft p-1 shadow-hairline"
        data-testid="terminal-selection-actions"
        role="group"
      >
        <SelectionAction
          icon={MessagesSquare}
          label="Send to conversation"
          onSelect={onSendToConversation}
        />
        <SelectionAction icon={Copy} label="Copy" onSelect={handleCopy} />
      </div>
    );
  }
  return (
    <div
      aria-label="Selection actions — no active session"
      className="relative z-10 min-w-36 flex-none rounded-lg bg-canvas-soft p-1 shadow-hairline"
      data-testid="terminal-selection-actions-no-session"
      role="group"
    >
      <Eyebrow className="px-1.5 py-1 text-muted">No active session</Eyebrow>
      <SelectionAction icon={MessagesSquare} label="Choose a session…" onSelect={handleChoose} />
      <SelectionAction icon={Plus} label="Start a session with this quote" onSelect={handleStart} />
      <SelectionAction icon={Copy} label="Copy as quoted block" onSelect={handleCopy} />
    </div>
  );
}

function SelectionAction({
  icon: Icon,
  label,
  onSelect,
}: {
  icon: typeof Copy;
  label: string;
  onSelect: () => void;
}) {
  return (
    <button
      className={cn(
        "relative flex w-full cursor-default items-center gap-1.5 rounded-md px-1.5 py-1",
        "text-left text-small-body select-none",
        "hover:bg-elevated hover:text-fg-strong",
        "focus-visible:bg-elevated focus-visible:text-fg-strong",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg]:size-4"
      )}
      onClick={onSelect}
      type="button"
    >
      <Icon aria-hidden="true" />
      {label}
    </button>
  );
}
