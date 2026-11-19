import { Button, StateGlyph, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { MessagesSquare } from "@compozy/ui/icons";

import { SIDE_CHAT_COPY } from "../../lib/session-side";

/**
 * The session head's "Side chats" toggle (S2): the count of open side chats, and the attention
 * dot when one waits on a permission or an answer. Pressed while the panel shows.
 */
export function SessionSideToggle({
  count,
  attention,
  open,
  onToggle,
  shortcut,
}: {
  count: number;
  attention: boolean;
  open: boolean;
  onToggle: () => void;
  /** The effective `session.side.toggle` chord, when the keymap names one. */
  shortcut?: string;
}) {
  const name =
    count > 0
      ? `${SIDE_CHAT_COPY.toggle}, ${count}${attention ? ", needs you" : ""}`
      : SIDE_CHAT_COPY.toggle;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size={count > 0 ? "sm" : "icon-sm"}
            aria-label={name}
            aria-pressed={open}
            data-state={open ? "open" : "closed"}
            data-attention={attention ? "true" : undefined}
            data-testid="session-side-toggle"
            onClick={onToggle}
          />
        }
      >
        <MessagesSquare aria-hidden="true" className="size-4" />
        {count > 0 ? (
          <span className="text-caption tabular-nums" data-testid="session-side-toggle-count">
            {count}
          </span>
        ) : null}
        {attention ? (
          <StateGlyph
            aria-hidden="true"
            data-testid="session-side-toggle-attention"
            size="sm"
            state="attention"
          />
        ) : null}
      </TooltipTrigger>
      <TooltipContent>
        {shortcut ? `${SIDE_CHAT_COPY.toggle} · ${shortcut}` : SIDE_CHAT_COPY.toggle}
      </TooltipContent>
    </Tooltip>
  );
}
