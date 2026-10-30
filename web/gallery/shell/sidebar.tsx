/**
 * Gallery navigation on `@compozy/ui`'s `Sidebar` (collapsible from its rail or with Cmd/Ctrl+B),
 * with the search on top and a three-level drill-down that slides horizontally:
 * 1. specs (plus "No spec"), then every journey flow;
 * 2. the prototypes and flows of the spec on screen;
 * 3. the states of the open prototype.
 * The level follows the URL (`use-selection.ts`), so every row and Back link is a shareable view.
 * All levels stay mounted (hidden ones are `inert`), so each keeps its scroll; Back or Esc goes up
 * one level.
 */
import type { KeyboardEvent, ReactNode, RefObject } from "react";

import { Eyebrow, Logo, SearchInput, Sidebar, cn } from "@compozy/ui";

import type { FlowEntry } from "#gallery/flow";
import { useCommentCounts } from "#gallery/shell/comments/use-comments";
import {
  flowMatches,
  matches,
  plural,
  specCode,
  stateMatches,
  type Entry,
  type Group,
  type Registry,
} from "#gallery/shell/registry-model";
import {
  BackRow,
  LevelHeader,
  PrototypeRows,
  SearchResults,
  SpecRows,
  StateRows,
  type OpenOf,
} from "#gallery/shell/sidebar-levels";
import { FlowResults, JourneyRows, SpecFlowRows } from "#gallery/shell/sidebar-flows";
import type { NavLevel } from "#gallery/shell/use-selection";
import { select, type Selection } from "#gallery/shell/view";

export type GallerySidebarProps = {
  registry: Registry;
  /** Prototype on screen (null on the index or a spec overview). */
  current: Entry | null;
  /** Flow on screen. */
  flow: FlowEntry | null;
  /** Spec of what is on screen. */
  group: Group | undefined;
  state: string;
  level: NavLevel;
  query: string;
  onQuery: (query: string) => void;
  searchRef: RefObject<HTMLInputElement | null>;
  collapsed: boolean;
  onCollapse: (collapsed: boolean) => void;
};

const offsets: Record<NavLevel, string> = {
  specs: "translate-x-0",
  prototypes: "-translate-x-1/3",
  states: "-translate-x-2/3",
};

/** Where Back (and Esc) leads from each level. */
function parentOf(level: NavLevel, group: Group | undefined): Selection | null {
  if (level === "states" && group) return { kind: "spec", spec: group.id };
  return level === "specs" ? null : { kind: "index" };
}

function NavPanel({
  level,
  current,
  label,
  children,
}: {
  level: NavLevel;
  current: NavLevel;
  label: string;
  children: ReactNode;
}) {
  return (
    <section
      aria-label={label}
      className="flex h-full w-1/3 flex-col gap-3 overflow-y-auto px-2 pb-3"
      data-testid={`nav-${level}`}
      inert={level !== current}
    >
      {children}
    </section>
  );
}

function SearchField({ searchRef, ...props }: GallerySidebarProps) {
  const prototypes = props.registry.entries.filter(entry => matches(entry, props.query));
  const states = stateMatches(props.registry.entries, props.query);
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape") {
      props.onQuery("");
      event.currentTarget.blur();
    }
    if (event.key !== "Enter" || !props.query.trim()) return;
    // Opens the first hit: a prototype, then a state, then a flow.
    const entry = prototypes[0];
    const hit = states[0];
    const flow = props.registry.flows.find(item => flowMatches(item, props.query));
    const target: Selection | null = entry
      ? { kind: "prototype", slug: entry.slug }
      : hit
        ? { kind: "prototype", slug: hit.entry.slug, state: hit.id }
        : flow
          ? { kind: "flow", flow: flow.id }
          : null;
    if (!target) return;
    event.preventDefault();
    select(target);
    props.onQuery("");
  };
  return (
    <SearchInput
      aria-label="Search prototypes, states and flows"
      containerClassName="w-full"
      data-testid="gallery-search"
      kbd="/"
      onChange={props.onQuery}
      onKeyDown={onKeyDown}
      placeholder="Search prototypes"
      ref={searchRef}
      value={props.query}
    />
  );
}

function Levels(props: GallerySidebarProps & { openOf: OpenOf }) {
  const { registry, group, current, level, openOf } = props;
  const activeFlow = props.flow?.id ?? null;
  const onEscape = (event: KeyboardEvent) => {
    const parent = parentOf(level, group);
    if (event.key !== "Escape" || !parent) return;
    event.preventDefault();
    select(parent);
  };
  return (
    <div
      className={cn(
        "flex h-full w-[300%] pt-1 transition-transform duration-base ease-out motion-reduce:transition-none",
        offsets[level]
      )}
      onKeyDown={onEscape}
      role="presentation"
    >
      <NavPanel current={level} label="Specs" level="specs">
        <SpecRows active={group?.id ?? null} openOf={openOf} registry={registry} />
        <JourneyRows active={activeFlow} openOf={openOf} registry={registry} />
        {registry.broken.length > 0 ? (
          <p className="px-2 font-mono text-mono-id tracking-mono-id text-danger">
            {plural(registry.broken.length, "problem")}: {registry.broken.join(" · ")}
          </p>
        ) : null}
      </NavPanel>
      <NavPanel
        current={level}
        label={group ? `Prototypes of ${group.title}` : "Prototypes"}
        level="prototypes"
      >
        <BackRow label="Specs" target={{ kind: "index" }} />
        {group ? (
          <>
            <LevelHeader
              detail={`${plural(group.entries.length, "prototype")} · ${plural(group.flows.length, "flow")}`}
              eyebrow={specCode(registry, group)}
              title={group.title}
            />
            {group.entries.length > 0 ? (
              <PrototypeRows current={current} group={group} openOf={openOf} />
            ) : null}
            <SpecFlowRows active={activeFlow} group={group} openOf={openOf} registry={registry} />
          </>
        ) : null}
      </NavPanel>
      <NavPanel
        current={level}
        label={current ? `States of ${current.meta.title}` : "States"}
        level="states"
      >
        <BackRow
          label={group?.title ?? "Specs"}
          target={group ? { kind: "spec", spec: group.id } : { kind: "index" }}
        />
        {current ? (
          <>
            <LevelHeader
              detail={`${current.meta.route ? `${current.meta.route} · ` : ""}/${current.slug}`}
              eyebrow={current.meta.surface}
              title={current.meta.title}
            />
            <StateRows entry={current} state={props.state} />
          </>
        ) : null}
      </NavPanel>
    </div>
  );
}

export function GallerySidebar(props: GallerySidebarProps) {
  const searching = props.query.trim().length > 0;
  const counts = useCommentCounts();
  const openOf: OpenOf = key => counts[key]?.open ?? 0;
  const flows = props.registry.flows.filter(flow => flowMatches(flow, props.query));
  return (
    <Sidebar
      collapseLabel="Show or hide the sidebar (⌘B)"
      collapsed={props.collapsed}
      data-testid="gallery-sidebar"
      header={
        <span className="flex min-w-0 items-center gap-2 px-1">
          <Logo className="h-4 w-auto text-fg" decorative variant="symbol" />
          <Eyebrow variant="caps">Prototypes</Eyebrow>
        </span>
      }
      nav={
        <nav aria-label="Prototypes" className="flex h-full min-h-0 flex-col gap-2 pt-2">
          <div className="px-2">
            <SearchField {...props} />
          </div>
          <div className="relative min-h-0 flex-1 overflow-hidden">
            {searching ? (
              <div className="flex h-full flex-col gap-3 overflow-y-auto px-2 pb-3">
                <SearchResults
                  more={flows.length > 0}
                  onNavigate={() => props.onQuery("")}
                  prototypes={props.registry.entries.filter(entry => matches(entry, props.query))}
                  states={stateMatches(props.registry.entries, props.query)}
                >
                  <FlowResults flows={flows} onNavigate={() => props.onQuery("")} />
                </SearchResults>
              </div>
            ) : (
              <Levels {...props} openOf={openOf} />
            )}
          </div>
        </nav>
      }
      onCollapse={props.onCollapse}
      panelWidth={280}
    />
  );
}
