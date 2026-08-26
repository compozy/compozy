import { Copy, MessagesSquare, Plus, TerminalSquare, X } from "lucide-react";

import { Button } from "@compozy/ui";

import type { TerminalQuote } from "../lib/terminal-quote";

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
      className="overflow-hidden rounded-xs border border-line"
      data-testid="terminal-quote-block"
    >
      <div className="flex min-h-6 items-center gap-1.75 bg-canvas-tint px-2.25 font-mono text-micro text-subtle">
        <TerminalSquare aria-hidden="true" className="size-2.5" />
        <span>
          {quote.terminalId} · lines {quote.fromLine}–{quote.toLine}
        </span>
        <button aria-label="Remove quote" className="ml-auto" onClick={onRemove} type="button">
          <X aria-hidden="true" className="size-2.5" />
        </button>
      </div>
      <div className="bg-chat-fill-code px-2.25 py-1.5 font-mono text-form-input leading-relaxed text-fg">
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
  onSendToConversation,
  onChooseSession,
  onStartSession,
  onCopy,
}: TerminalSelectionActionsProps) {
  if (hasActiveSession) {
    return (
      <div className="flex flex-col" data-testid="terminal-selection-actions">
        <SelectionAction
          icon={MessagesSquare}
          label="Send to conversation"
          onSelect={onSendToConversation}
        />
        <SelectionAction icon={Copy} label="Copy" onSelect={onCopy} />
      </div>
    );
  }
  return (
    <div className="flex flex-col" data-testid="terminal-selection-actions-no-session">
      <span className="px-2.5 py-1.5 text-eyebrow text-subtle">No active session</span>
      <SelectionAction icon={MessagesSquare} label="Choose a session…" onSelect={onChooseSession} />
      <SelectionAction
        icon={Plus}
        label="Start a session with this quote"
        onSelect={onStartSession}
      />
      <SelectionAction icon={Copy} label="Copy as quoted block" onSelect={onCopy} />
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
    <Button className="justify-start" onClick={onSelect} size="sm" type="button" variant="ghost">
      <Icon aria-hidden="true" className="size-3" />
      {label}
    </Button>
  );
}
