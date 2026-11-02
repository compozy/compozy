/** Everything linked to a session, urgent first. Past a handful, grouped and with Done folded. */
import { useState } from "react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Empty,
  ListGroup,
} from "@compozy/ui";

import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import {
  LINK_GROUP_LABEL,
  LIST_FOLD_DONE_MIN,
  LIST_GROUPS_MIN,
  groupLinks,
  matchesLink,
  type LinkGroup,
} from "./link-model";
import { LinkRow } from "./link-row";

export interface LinkListProps {
  links: readonly SessionLink[];
  query: string;
  agentLabel?: string;
  /** The link whose menu is pinned open for review. */
  openMenuFor?: string;
  /** The link whose details are pinned open for review. */
  openDetailFor?: string;
  onUnlink: (link: SessionLink) => void;
}

const GROUP_HEADER = {
  className: "sticky top-0 z-20 border-b-0 bg-popover px-3 pt-3 pb-1.5",
};

function GroupRows({ group, list }: { group: LinkGroup; list: LinkListProps }) {
  return (
    <>
      {group.links.map(link => (
        <LinkRow
          agentLabel={list.agentLabel ?? "Claude"}
          key={link.id}
          link={link}
          detailOpen={link.id === list.openDetailFor}
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
        <ListGroup.Items className="gap-0.5">
          <GroupRows group={group} list={list} />
        </ListGroup.Items>
      </CollapsibleContent>
    </Collapsible>
  );
}

export function LinkList(list: LinkListProps) {
  const { links, query } = list;
  const groups = groupLinks(links.filter(link => matchesLink(link, query)));
  const foldDone = links.length >= LIST_FOLD_DONE_MIN && query === "";
  if (groups.length === 0) {
    return (
      <Empty
        className="py-8"
        description="Try a number, a word from the title, or a branch."
        fill={false}
        size="compact"
        title="Nothing matches"
      />
    );
  }
  if (links.length < LIST_GROUPS_MIN) {
    return (
      <ListGroup itemsProps={{ className: "gap-0.5" }}>
        {groups.map(group => (
          <GroupRows group={group} key={group.id} list={list} />
        ))}
      </ListGroup>
    );
  }
  return groups.map(group =>
    group.id === "done" && foldDone ? (
      <DoneGroup group={group} key={group.id} list={list} />
    ) : (
      <ListGroup
        count={group.links.length}
        headerProps={GROUP_HEADER}
        itemsProps={{ className: "gap-0.5" }}
        key={group.id}
        label={LINK_GROUP_LABEL[group.id]}
      >
        <GroupRows group={group} list={list} />
      </ListGroup>
    )
  );
}
