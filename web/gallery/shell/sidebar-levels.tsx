/**
 * Rows and panels of the gallery's drill-down navigation (`sidebar.tsx`): spec rows (level 1),
 * prototype rows (level 2), the states of one prototype (level 3) and flat search results. Every
 * row is a numbered link with its own URL; a plain click adds a history entry.
 */
import { ChevronLeftIcon, ChevronRightIcon, MessageSquareIcon } from "lucide-react";
import type { ReactNode } from "react";

import {
  Eyebrow,
  Item,
  ItemContent,
  ItemDescription,
  ItemTitle,
  SidebarSectionLabel,
} from "@compozy/ui";

import {
  ordinal,
  plural,
  specPosition,
  specSlug,
  type Entry,
  type Group,
  type Registry,
} from "#gallery/shell/registry-model";
import { SelectionLink } from "#gallery/shell/selection-link";
import type { Selection } from "#gallery/shell/view";

function RowNumber({ index }: { index: number | null }) {
  return (
    <span className="w-5 shrink-0 self-start pt-px font-mono text-mono-id tracking-mono-id text-subtle tabular-nums">
      {index === null ? "" : ordinal(index)}
    </span>
  );
}

/** Open review comments of a row, in the accent ink; nothing when there are none. */
export function CommentCount({ count }: { count: number }) {
  if (count === 0) return null;
  return (
    <span
      className="flex items-center gap-0.5 text-accent"
      data-testid="nav-comment-count"
      title={`${plural(count, "open comment")}`}
    >
      <MessageSquareIcon aria-hidden="true" className="size-3" />
      {count}
      <span className="sr-only">open comments</span>
    </span>
  );
}

/** Open comments of a comment key (prototype slug or `flow-<id>`). */
export type OpenOf = (key: string) => number;

const noComments: OpenOf = () => 0;

/** A two-line sidebar row linking to a selection; the open one is selected. */
export function NavRow({
  target,
  active,
  index,
  title,
  detail,
  aside,
  onNavigate,
  testId,
}: {
  target: Selection;
  active: boolean;
  index: number | null;
  title: ReactNode;
  detail?: ReactNode;
  aside?: ReactNode;
  onNavigate?: () => void;
  testId: string;
}) {
  return (
    <li>
      <Item
        aria-current={active ? "page" : undefined}
        data-testid={testId}
        render={<SelectionLink onNavigate={onNavigate} to={target} />}
        selected={active}
        size="xs"
      >
        <RowNumber index={index} />
        <ItemContent>
          <ItemTitle className="font-normal">{title}</ItemTitle>
          {detail ? (
            <ItemDescription className="truncate font-mono text-mono-id tracking-mono-id">
              {detail}
            </ItemDescription>
          ) : null}
        </ItemContent>
        {aside ? (
          <span className="flex items-center gap-2 self-start font-mono text-mono-id tracking-mono-id text-subtle">
            {aside}
          </span>
        ) : null}
      </Item>
    </li>
  );
}

export function RowGroup({
  label,
  aside,
  children,
}: {
  label: string;
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section aria-label={label}>
      <SidebarSectionLabel className="justify-between">
        <span>{label}</span>
        {aside === undefined ? null : <span className="tabular-nums">{aside}</span>}
      </SidebarSectionLabel>
      <ul className="flex flex-col gap-0.5">{children}</ul>
    </section>
  );
}

/** Link one level up: the parent's own URL, so Back and Esc land on a shareable view. */
export function BackRow({ label, target }: { label: string; target: Selection }) {
  return (
    <SelectionLink
      className="flex h-sidebar-row shrink-0 items-center gap-1.5 rounded-md px-2 text-small-body text-muted transition-colors outline-none hover:bg-hover hover:text-fg focus-visible:shadow-focus-ring"
      data-testid="nav-back"
      to={target}
    >
      <ChevronLeftIcon aria-hidden="true" className="size-4 shrink-0" />
      <span className="truncate">{label}</span>
    </SelectionLink>
  );
}

export function LevelHeader({
  eyebrow,
  title,
  detail,
}: {
  eyebrow?: string;
  title: string;
  detail: string;
}) {
  return (
    <header className="flex shrink-0 flex-col gap-1 px-2">
      {eyebrow ? <Eyebrow className="text-accent">{eyebrow}</Eyebrow> : null}
      <h2 className="text-item-title font-medium text-fg">{title}</h2>
      <span className="font-mono text-mono-id tracking-mono-id break-all text-subtle">
        {detail}
      </span>
    </header>
  );
}

export function SpecRows({
  registry,
  active,
  openOf = noComments,
}: {
  registry: Registry;
  active: string | null;
  openOf?: OpenOf;
}) {
  return (
    <RowGroup aside={registry.groups.length} label="Specs">
      {registry.groups.map(group => (
        <NavRow
          active={group.id === active}
          aside={
            <>
              <CommentCount
                count={group.entries.reduce((sum, entry) => sum + openOf(entry.slug), 0)}
              />
              <ChevronRightIcon aria-hidden="true" className="size-3.5" />
            </>
          }
          detail={specSlug(group)}
          index={specPosition(registry, group.id)}
          key={group.id}
          target={{ kind: "spec", spec: group.id }}
          testId="nav-spec"
          title={group.title}
        />
      ))}
    </RowGroup>
  );
}

function prototypeDetail(entry: Entry) {
  const { meta } = entry;
  return [meta.surface, meta.route ?? `/${entry.slug}`, entry.kind === "draft" ? "draft" : null]
    .filter(Boolean)
    .join(" · ");
}

export function PrototypeRows({
  group,
  current,
  label = "Prototypes",
  numbered = true,
  openOf = noComments,
  onNavigate,
}: {
  group: Pick<Group, "entries">;
  current: Entry | null;
  label?: string;
  numbered?: boolean;
  openOf?: OpenOf;
  onNavigate?: () => void;
}) {
  return (
    <RowGroup aside={group.entries.length} label={label}>
      {group.entries.map((entry, index) => (
        <NavRow
          active={entry.slug === current?.slug}
          aside={
            <>
              <CommentCount count={openOf(entry.slug)} />
              {entry.meta.states.length}
            </>
          }
          detail={prototypeDetail(entry)}
          index={numbered ? index : null}
          key={entry.slug}
          onNavigate={onNavigate}
          target={{ kind: "prototype", slug: entry.slug }}
          testId="nav-prototype"
          title={entry.meta.title}
        />
      ))}
    </RowGroup>
  );
}

export function StateRows({ entry, state }: { entry: Entry; state: string }) {
  return (
    <RowGroup aside={plural(entry.meta.states.length, "state")} label="States">
      {entry.meta.states.map((option, index) => (
        <NavRow
          active={option.id === state}
          index={index}
          key={option.id}
          target={{ kind: "prototype", slug: entry.slug, state: option.id }}
          testId="nav-state"
          title={option.label}
        />
      ))}
    </RowGroup>
  );
}

/** Flat search results: prototypes, states, then `children`; opening one clears the search. */
export function SearchResults({
  prototypes,
  states,
  onNavigate,
  more = false,
  children,
}: {
  prototypes: Entry[];
  states: { entry: Entry; id: string; label: string }[];
  onNavigate: () => void;
  /** Other matches render as `children` (flows), so "Nothing found" only shows without them. */
  more?: boolean;
  children?: ReactNode;
}) {
  if (prototypes.length === 0 && states.length === 0 && !more) {
    return <p className="px-2 py-4 text-small-body text-muted">Nothing found.</p>;
  }
  return (
    <>
      {prototypes.length > 0 ? (
        <PrototypeRows
          current={null}
          group={{ entries: prototypes }}
          numbered={false}
          onNavigate={onNavigate}
        />
      ) : null}
      {states.length > 0 ? (
        <RowGroup aside={states.length} label="States">
          {states.map(item => (
            <NavRow
              active={false}
              detail={item.entry.meta.title}
              index={null}
              key={`${item.entry.slug}:${item.id}`}
              onNavigate={onNavigate}
              target={{ kind: "prototype", slug: item.entry.slug, state: item.id }}
              testId="nav-search-state"
              title={item.label}
            />
          ))}
        </RowGroup>
      ) : null}
      {children}
    </>
  );
}
