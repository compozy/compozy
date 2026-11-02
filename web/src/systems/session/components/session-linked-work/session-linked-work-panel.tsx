/**
 * The popover's body. The work leads; where the session runs closes it as one quiet line, so the
 * difference between the two stays in view without standing between you and the list.
 */
import { FolderGit2, FolderIcon, Link2 } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, Icon, PopoverTitle, SearchInput } from "@compozy/ui";

import {
  LIST_SEARCH_MIN,
  type SessionLinkedWorkItem,
  type SessionRunsIn,
} from "../../lib/session-linked-work";
import { SessionLinkList } from "./session-link-list";
import type { SessionLinkActions } from "./session-link-row";

export interface SessionLinkedWorkPanelProps {
  items: readonly SessionLinkedWorkItem[];
  runsIn: SessionRunsIn;
  agentLabel: string;
  actions: SessionLinkActions;
  openMenuFor?: string;
  openDetailFor?: string;
}

export function SessionLinkedWorkPanel({
  items,
  runsIn,
  agentLabel,
  actions,
  openMenuFor,
  openDetailFor,
}: SessionLinkedWorkPanelProps) {
  const [query, setQuery] = useState("");
  const searchable = items.length >= LIST_SEARCH_MIN;
  return (
    <div className="flex max-h-[min(78vh,52rem)] min-h-0 flex-col" data-slot="session-linked-work">
      <div className="flex items-center gap-2 px-4 pt-3.5 pb-3">
        <PopoverTitle className="text-small-body font-semibold text-fg-strong">
          Linked work
        </PopoverTitle>
        <span className="font-mono text-mono-id text-faint tabular-nums">{items.length}</span>
        <Button
          className="ml-auto"
          onClick={actions.onLink}
          size="xs"
          type="button"
          variant="secondary"
        >
          <Icon as={Link2} data-icon="inline-start" size="sm" />
          Link…
        </Button>
      </div>
      {searchable ? (
        <div className="px-4 pb-2">
          <SearchInput
            aria-label="Filter linked work"
            containerClassName="min-w-0"
            onChange={setQuery}
            placeholder="Filter by title, number or branch…"
            value={query}
          />
        </div>
      ) : null}
      <div className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-1.5">
        <SessionLinkList
          actions={actions}
          agentLabel={agentLabel}
          items={items}
          openDetailFor={openDetailFor}
          openMenuFor={openMenuFor}
          query={searchable ? query : ""}
        />
      </div>
      <div
        className="flex min-w-0 items-center gap-2 border-t border-line bg-sunken px-4 py-2.5 text-eyebrow text-muted-foreground"
        data-slot="session-runs-in"
      >
        <Icon
          as={runsIn.kind === "worktree" ? FolderGit2 : FolderIcon}
          className="size-3.5 shrink-0 text-subtle"
        />
        <span className="min-w-0 truncate">
          This session runs in <span className="font-medium text-fg">{runsIn.name}</span>. Linking
          never moves it.
        </span>
      </div>
    </div>
  );
}
