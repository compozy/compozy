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

import {
  LINK_GROUP_LABEL,
  LIST_FOLD_DONE_MIN,
  LIST_GROUPS_MIN,
  groupLinks,
  linkKey,
  matchesLink,
  type LinkGroup,
  type SessionLinkedWorkItem,
} from "../../lib/session-linked-work";
import { SessionLinkRow, type SessionLinkActions } from "./session-link-row";

export interface SessionLinkListProps {
  items: readonly SessionLinkedWorkItem[];
  query: string;
  agentLabel: string;
  actions: SessionLinkActions;
  /** The item whose menu is pinned open for review, by `linkKey`. */
  openMenuFor?: string;
  /** The item whose details are pinned open for review, by `linkKey`. */
  openDetailFor?: string;
}

const GROUP_HEADER = {
  className: "sticky top-0 z-20 border-b-0 bg-popover px-3 pt-3 pb-1.5",
};

function GroupRows({ group, list }: { group: LinkGroup; list: SessionLinkListProps }) {
  return (
    <>
      {group.items.map(item => {
        const key = linkKey(item);
        return (
          <SessionLinkRow
            actions={list.actions}
            agentLabel={list.agentLabel}
            detailOpen={key === list.openDetailFor}
            item={item}
            key={key}
            menuOpen={key === list.openMenuFor}
          />
        );
      })}
    </>
  );
}

function DoneGroup({ group, list }: { group: LinkGroup; list: SessionLinkListProps }) {
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
        count={group.items.length}
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

export function SessionLinkList(list: SessionLinkListProps) {
  const { items, query } = list;
  const groups = groupLinks(items.filter(item => matchesLink(item, query)));
  const foldDone = items.length >= LIST_FOLD_DONE_MIN && query === "";
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
  if (items.length < LIST_GROUPS_MIN) {
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
        count={group.items.length}
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
