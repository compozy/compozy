import { cn } from "@/lib/utils";
import {
  SESSION_BOARD_CARD_TITLE_ROW,
  SESSION_BOARD_CARD_TITLE_TYPE,
} from "./sessions-board-frame";

export function SessionBoardCardTitle({
  archived,
  sessionId,
  title,
  unread,
  untitled,
}: {
  archived: boolean;
  sessionId: string;
  title: string;
  unread: boolean;
  untitled: boolean;
}) {
  return (
    <span className={SESSION_BOARD_CARD_TITLE_ROW}>
      {unread ? (
        <span
          aria-hidden="true"
          className="mt-1.5 size-1.5 shrink-0 rounded-full bg-fg-strong"
          data-testid={`session-board-card-unread-${sessionId}`}
        />
      ) : null}
      <span
        className={cn(
          "line-clamp-2 min-w-0",
          SESSION_BOARD_CARD_TITLE_TYPE,
          untitled && "font-normal text-subtle",
          archived && "text-muted-foreground"
        )}
      >
        {title}
      </span>
    </span>
  );
}
