/**
 * The two levels above a prototype: the specs index (`/`) and a spec overview (`?spec=`). Both
 * are shareable views of numbered links, so a reviewer can send "this spec" and the other person
 * lands on the same list: every prototype with its states, and the journey flows.
 */
import { ChevronRightIcon } from "lucide-react";
import type { ReactNode } from "react";

import type { FlowEntry } from "#gallery/flow";
import {
  Eyebrow,
  Item,
  ItemContent,
  ItemDescription,
  ItemTitle,
  Pill,
  SidebarSectionLabel,
} from "@compozy/ui";

import { copyTargets, indexCrumb, specCrumb, type Crumb } from "#gallery/shell/crumbs";
import {
  ordinal,
  plural,
  specCode,
  specPosition,
  specSlug,
  type Entry,
  type Group,
  type Registry,
} from "#gallery/shell/registry-model";
import { ShortcutsHelp } from "#gallery/shell/shortcuts";
import { SelectionLink } from "#gallery/shell/selection-link";
import { CopyLinkMenu, GalleryThemeToggle, ViewHeading } from "#gallery/shell/topbar";

type OverviewProps = {
  registry: Registry;
  dark: boolean;
  helpOpen: boolean;
  onHelp: (open: boolean) => void;
};

function OverviewBar({
  crumbs,
  meta,
  ...props
}: Omit<OverviewProps, "registry"> & { crumbs: readonly Crumb[]; meta: string }) {
  const links = copyTargets(crumbs);
  return (
    <header className="flex min-h-12 items-center gap-2" data-testid="gallery-toolbar">
      <div className="min-w-0 flex-1 pl-1">
        <ViewHeading crumbs={crumbs} meta={meta} />
      </div>
      <GalleryThemeToggle dark={props.dark} />
      {links.length > 0 ? <CopyLinkMenu links={links} /> : null}
      <ShortcutsHelp onOpenChange={props.onHelp} open={props.helpOpen} />
    </header>
  );
}

function Body({
  title,
  aside,
  children,
}: {
  title: string;
  aside: ReactNode;
  children: ReactNode;
}) {
  return (
    <div
      className="min-h-0 flex-1 overflow-y-auto rounded-xl bg-surface-2 p-5"
      data-testid="overview"
    >
      <section aria-label={title} className="mx-auto flex max-w-5xl flex-col gap-3">
        <SidebarSectionLabel className="justify-between px-0">
          <span>{title}</span>
          <span className="tabular-nums">{aside}</span>
        </SidebarSectionLabel>
        {children}
      </section>
    </div>
  );
}

function SpecCard({ registry, group }: { registry: Registry; group: Group }) {
  const position = specPosition(registry, group.id);
  const states = group.entries.reduce((sum, entry) => sum + entry.meta.states.length, 0);
  return (
    <li>
      <Item
        data-testid="overview-spec"
        render={<SelectionLink to={{ kind: "spec", spec: group.id }} />}
        variant="outline"
      >
        <span className="w-5 font-mono text-mono-id tracking-mono-id text-subtle">
          {position === null ? "" : ordinal(position)}
        </span>
        <ItemContent>
          <ItemTitle>{group.title}</ItemTitle>
          <ItemDescription className="font-mono text-mono-id tracking-mono-id">
            {specSlug(group)} · {plural(group.entries.length, "prototype")} ·{" "}
            {plural(states, "state")}
            {group.flows.length > 0 ? ` · ${plural(group.flows.length, "flow")}` : ""}
          </ItemDescription>
        </ItemContent>
        <ChevronRightIcon aria-hidden="true" className="size-4 text-subtle" />
      </Item>
    </li>
  );
}

function PrototypeCard({ entry, index }: { entry: Entry; index: number }) {
  const { meta } = entry;
  return (
    <li
      className="flex min-w-0 flex-col gap-2 rounded-lg border border-line p-3"
      data-slug={entry.slug}
    >
      <Item
        data-testid="overview-prototype"
        render={<SelectionLink to={{ kind: "prototype", slug: entry.slug }} />}
        size="xs"
      >
        <span className="w-5 font-mono text-mono-id tracking-mono-id text-subtle">
          {ordinal(index)}
        </span>
        <ItemContent>
          <ItemTitle>
            {meta.surface ? <Eyebrow className="text-accent">{meta.surface}</Eyebrow> : null}
            {meta.title}
            {entry.kind === "draft" ? <Pill>Draft</Pill> : null}
          </ItemTitle>
          <ItemDescription className="font-mono text-mono-id tracking-mono-id">
            {meta.route ? `${meta.route} · ` : ""}/{entry.slug}
          </ItemDescription>
        </ItemContent>
      </Item>
      <ol aria-label={`States of ${meta.title}`} className="flex min-w-0 flex-wrap gap-1.5">
        {meta.states.map((option, position) => (
          <li className="max-w-full min-w-0" key={option.id}>
            <Pill
              data-testid="overview-state"
              form="hollow"
              render={
                <SelectionLink to={{ kind: "prototype", slug: entry.slug, state: option.id }} />
              }
            >
              <span className="font-mono text-subtle">{ordinal(position)}</span>
              <span className="truncate">{option.label}</span>
            </Pill>
          </li>
        ))}
      </ol>
    </li>
  );
}

function FlowCards({ title, flows }: { title: string; flows: readonly FlowEntry[] }) {
  if (flows.length === 0) return null;
  return (
    <section aria-label={title} className="flex flex-col gap-3 pt-3">
      <SidebarSectionLabel className="justify-between px-0">
        <span>{title}</span>
        <span className="tabular-nums">{flows.length}</span>
      </SidebarSectionLabel>
      <ul className="grid gap-2 md:grid-cols-2">
        {flows.map((flow, index) => (
          <li key={flow.id}>
            <Item
              data-testid="overview-flow"
              render={<SelectionLink to={{ kind: "flow", flow: flow.id }} />}
              variant="outline"
            >
              <span className="w-5 font-mono text-mono-id tracking-mono-id text-subtle">
                {ordinal(index)}
              </span>
              <ItemContent>
                <ItemTitle>{flow.title}</ItemTitle>
                <ItemDescription className="font-mono text-mono-id tracking-mono-id">
                  {flow.id} · {plural(flow.nodes.length, "step")} ·{" "}
                  {plural(flow.edges.length, "transition")}
                </ItemDescription>
              </ItemContent>
              <ChevronRightIcon aria-hidden="true" className="size-4 text-subtle" />
            </Item>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Problems({ registry }: { registry: Registry }) {
  if (registry.broken.length === 0) return null;
  return (
    <ul className="flex flex-col gap-1 font-mono text-mono-id tracking-mono-id text-danger">
      {registry.broken.map(message => (
        <li key={message}>{message}</li>
      ))}
    </ul>
  );
}

export function IndexOverview(props: OverviewProps) {
  const { registry } = props;
  return (
    <>
      <OverviewBar
        {...props}
        crumbs={[indexCrumb]}
        meta={`${plural(registry.groups.length, "spec")} · ${plural(registry.entries.length, "prototype")} · ${plural(registry.flows.length, "flow")}`}
      />
      <Body aside={registry.groups.length} title="Specs">
        <ul className="grid gap-2 md:grid-cols-2">
          {registry.groups.map(group => (
            <SpecCard group={group} key={group.id} registry={registry} />
          ))}
        </ul>
        <FlowCards flows={registry.flows} title="Journeys" />
        <Problems registry={registry} />
      </Body>
    </>
  );
}

export function SpecOverview(props: OverviewProps & { group: Group }) {
  const { registry, group } = props;
  return (
    <>
      <OverviewBar
        {...props}
        crumbs={[indexCrumb, specCrumb(group)]}
        meta={`${specCode(registry, group)} · ${plural(group.entries.length, "prototype")} · ${plural(group.flows.length, "flow")}`}
      />
      <Body aside={group.entries.length} title="Prototypes">
        <ul className="grid gap-2 lg:grid-cols-2">
          {group.entries.map((entry, index) => (
            <PrototypeCard entry={entry} index={index} key={entry.slug} />
          ))}
        </ul>
        <FlowCards flows={group.flows} title="Flows" />
        <FlowCards flows={group.crossing} title="Crossing this spec" />
      </Body>
    </>
  );
}
