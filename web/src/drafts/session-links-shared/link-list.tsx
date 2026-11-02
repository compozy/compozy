/** Everything linked to a session, urgent first, findable at any size. */
import { useState } from "react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Empty,
  ListGroup,
  SearchInput,
} from "@compozy/ui";

import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import {
  LINK_GROUP_LABEL,
  LIST_FOLD_DONE_MIN,
  LIST_GROUPS_MIN,
  LIST_SEARCH_MIN,
  groupLinks,
  matchesLink,
  type LinkGroup,
} from "./link-model";
import { LinkRow, type LinkRowLayout } from "./link-row";

export interface LinkListProps {
  links: readonly SessionLink[];
  layout?: LinkRowLayout;
  agentLabel?: string;
  /** The link whose menu is pinned open for review. */
  openMenuFor?: string;
  onUnlink: (link: SessionLink) => void;
}

const GROUP_HEADER = { className: "border-b-0 bg-transparent px-2.5 py-1" };

function GroupRows({ group, list }: { group: LinkGroup; list: LinkListProps }) {
  return (
    <>
      {group.links.map(link => (
        <LinkRow
          agentLabel={list.agentLabel ?? "Claude"}
          key={link.id}
          layout={list.layout ?? "stacked"}
          link={link}
          menuOpen={link.id === list.openMenuFor}
          onUnlink={list.onUnlink}
        />
      ))}
    </>
  );
}

function DoneGroup({ group, list }: { group: LinkGroup; list: LinkListProps }) {
  const [open, setOpen] = useState(false);
  return (
    <Collapsible onOpenChange={setOpen} open={open}>
      <ListGroup.Header
        {...GROUP_HEADER}
        actions={
          <CollapsibleTrigger render={<Button size="xs" type="button" variant="quiet" />}>
            {open ? "Hide" : "Show"}
          </CollapsibleTrigger>
        }
        count={group.links.length}
        label={LINK_GROUP_LABEL.done}
      />
      <CollapsibleContent>
        <ListGroup.Items>
          <GroupRows group={group} list={list} />
        </ListGroup.Items>
      </CollapsibleContent>
    </Collapsible>
  );
}

export function LinkList(list: LinkListProps) {
  const { links } = list;
  const [query, setQuery] = useState("");
  const groups = groupLinks(links.filter(link => matchesLink(link, query)));
  const headed = links.length >= LIST_GROUPS_MIN;
  const foldDone = links.length >= LIST_FOLD_DONE_MIN && query === "";
  return (
    <div className="flex min-w-0 flex-col gap-2" data-slot="session-link-list">
      {links.length >= LIST_SEARCH_MIN ? (
        <SearchInput
          aria-label="Filter linked work"
          containerClassName="min-w-0"
          onChange={setQuery}
          placeholder="Filter by title, number or branch…"
          value={query}
        />
      ) : null}
      {headed ? null : (
        <ListGroup>
          {groups.map(group => (
            <GroupRows group={group} key={group.id} list={list} />
          ))}
        </ListGroup>
      )}
      {(headed ? groups : []).map(group =>
        group.id === "done" && foldDone ? (
          <DoneGroup group={group} key={group.id} list={list} />
        ) : (
          <ListGroup
            count={group.links.length}
            headerProps={GROUP_HEADER}
            key={group.id}
            label={LINK_GROUP_LABEL[group.id]}
          >
            <GroupRows group={group} list={list} />
          </ListGroup>
        )
      )}
      {groups.length === 0 ? (
        <Empty
          description="Try a number, a word from the title, or a branch."
          fill={false}
          size="compact"
          title="Nothing matches"
        />
      ) : null}
    </div>
  );
}
