/** The real board: a card keeps its lead PR and gains `+N PRs`, a row gains the count. */
import { SessionBoardCardView } from "@/systems/session/components/session-board/session-board-card-view";
import type { SessionBoardCardProps } from "@/systems/session/components/session-board/session-board-card";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import {
  SESSION_LINK_SETS,
  SESSION_LINKS_DOZEN,
  type SessionLink,
} from "@/systems/session/mocks/session-links-fixtures";

import { leadLink, moreLabel } from "../session-links-shared/link-model";
import { GLOBAL_ROW } from "../stays-on-agents-shared/sessions-fixtures";
import { KanbanWindow } from "../stays-on-sessions/kanban";
import { liveColumns } from "../stays-on-sessions/kanban-columns";

const LINKS: Record<string, readonly SessionLink[]> = {
  s1: SESSION_LINKS_DOZEN,
  s2: SESSION_LINK_SETS.mixed,
};

function LinksCard({ actions: _actions, session, ...props }: SessionBoardCardProps) {
  const links = LINKS[session.id] ?? [];
  const lead = leadLink(links);
  const more = moreLabel(links);
  // `+3 PRs` takes the branch's place: with several items, one branch says little.
  const shown = lead
    ? { ...session, ...lead.facts, branch: more ? "" : lead.facts.branch }
    : session;
  return (
    <SessionBoardCardView
      {...props}
      metaExtra={
        more ? (
          <span className="min-w-0 shrink-0 font-sans text-eyebrow tracking-eyebrow text-muted-foreground">
            {more}
          </span>
        ) : undefined
      }
      runtime={boardRuntimeFixture(session)}
      session={shown}
    />
  );
}

export function BoardSurface() {
  const columns = liveColumns();
  return (
    <KanbanWindow
      card={LinksCard}
      lanes={[{ name: "compozy", count: 6, columns }]}
      top={GLOBAL_ROW}
    />
  );
}
