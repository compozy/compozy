/**
 * The popover's body. The work leads; where the session runs closes it as one quiet line, so the
 * difference between the two stays in view without standing between you and the list.
 */
import { FolderGit2, FolderIcon, Link2 } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, Icon, PopoverTitle, SearchInput } from "@compozy/ui";

import type { SessionLink, SessionRunsIn } from "@/systems/session/mocks/session-links-fixtures";

import { LinkList } from "./link-list";
import { LIST_SEARCH_MIN } from "./link-model";

export interface LinkedWorkProps {
  links: readonly SessionLink[];
  runsIn: SessionRunsIn;
  openMenuFor?: string;
  openDetailFor?: string;
  onLink: () => void;
  onUnlink: (link: SessionLink) => void;
}

export function LinkedWork({
  links,
  runsIn,
  openMenuFor,
  openDetailFor,
  onLink,
  onUnlink,
}: LinkedWorkProps) {
  const [query, setQuery] = useState("");
  return (
    <div className="flex max-h-[min(78vh,52rem)] min-h-0 flex-col" data-slot="session-linked-work">
      <div className="flex items-center gap-2 px-4 pt-3.5 pb-3">
        <PopoverTitle className="text-small-body font-semibold text-fg-strong">
          Linked work
        </PopoverTitle>
        <span className="font-mono text-mono-id text-faint tabular-nums">{links.length}</span>
        <Button className="ml-auto" onClick={onLink} size="xs" type="button" variant="secondary">
          <Icon as={Link2} data-icon="inline-start" size="sm" />
          Link…
        </Button>
      </div>
      {links.length >= LIST_SEARCH_MIN ? (
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
        <LinkList
          links={links}
          onUnlink={onUnlink}
          openDetailFor={openDetailFor}
          openMenuFor={openMenuFor}
          query={query}
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
