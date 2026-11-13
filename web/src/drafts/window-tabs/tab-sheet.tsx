/** S2 specimen: every tab variant on the bar's own surface, at fixed widths. */
import type { ReactNode } from "react";

import { TAB, WINDOWS } from "../window-tabs-shared/fixtures";
import { OsTab, type OsTabProps } from "../window-tabs-shared/os-tab";

interface Specimen {
  label: string;
  note?: string;
  props: OsTabProps;
}

const base = { windows: WINDOWS } as const;

const GROUPS: { title: string; items: Specimen[] }[] = [
  {
    title: "States",
    items: [
      { label: "Rest", props: { ...base, tab: TAB.auth, width: 200 } },
      { label: "Hover", props: { ...base, tab: TAB.auth, width: 200, hovered: true } },
      { label: "Active", props: { ...base, tab: TAB.auth, width: 200, active: true } },
      {
        label: "Keyboard focus",
        props: { ...base, tab: TAB.auth, width: 200, focused: true },
      },
      {
        label: "Being dragged",
        note: "Stays in place, dimmed",
        props: { ...base, tab: TAB.auth, width: 200, dragging: true },
      },
      {
        label: "Drop target",
        note: "A dragged window rests on it",
        props: { ...base, tab: TAB.auth, width: 200, dwelling: true },
      },
    ],
  },
  {
    title: "Splits",
    items: [
      { label: "Two windows", props: { ...base, tab: TAB.twoUp, width: 224 } },
      { label: "Three windows", props: { ...base, tab: TAB.threeWay, width: 224 } },
      {
        label: "Four or more",
        note: "Three tiles, the count says the rest",
        props: { ...base, tab: TAB.fourUp, width: 224, active: true },
      },
    ],
  },
  {
    title: "Signals",
    items: [
      { label: "Working", props: { ...base, tab: TAB.tabs, width: 224 } },
      {
        label: "Needs you",
        note: "Glyph and the waiting count",
        props: { ...base, tab: TAB.review, width: 224 },
      },
    ],
  },
  {
    title: "Pinned",
    items: [
      { label: "Pinned", props: { ...base, tab: TAB.dash } },
      { label: "Pinned, active", props: { ...base, tab: TAB.agents, active: true } },
      { label: "Pinned split", props: { ...base, tab: TAB.pinnedSplit } },
    ],
  },
  {
    title: "Shrinking",
    items: [
      { label: "224px, the widest", props: { ...base, tab: TAB.review, width: 224 } },
      {
        label: "Under 160px",
        note: "The member count goes",
        props: { ...base, tab: TAB.fourUp, width: 152 },
      },
      {
        label: "Under 144px",
        note: "The waiting count goes",
        props: { ...base, tab: TAB.review, width: 136 },
      },
      {
        label: "Under 120px",
        note: "Signal moves to the icon, no close",
        props: { ...base, tab: TAB.review, width: 112, hovered: true },
      },
      {
        label: "Under 72px",
        note: "Icon only",
        props: { ...base, tab: TAB.review, width: 64 },
      },
      { label: "40px floor", props: { ...base, tab: TAB.docs, width: 40 } },
      {
        label: "Active, narrow",
        note: "Keeps its close button",
        props: { ...base, tab: TAB.docs, width: 96, active: true },
      },
      {
        label: "Active, under 88px",
        note: "Icon and close",
        props: { ...base, tab: TAB.docs, width: 72, active: true },
      },
      {
        label: "Active at the floor",
        note: "Icon only, like the rest",
        props: { ...base, tab: TAB.docs, width: 40, active: true },
      },
    ],
  },
];

function Row({ label, note, children }: { label: string; note?: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-6">
      <div className="w-44 shrink-0">
        <div className="text-body text-fg">{label}</div>
        {note ? <div className="text-meta text-muted-foreground">{note}</div> : null}
      </div>
      <div className="flex h-10 items-center rounded-md bg-rail px-1.5">{children}</div>
    </div>
  );
}

export function TabSheet() {
  return (
    <div className="min-h-dvh bg-canvas p-10 text-fg">
      <div className="grid grid-cols-[repeat(auto-fill,minmax(460px,1fr))] gap-x-12 gap-y-10">
        {GROUPS.map(group => (
          <section className="flex flex-col gap-3" key={group.title}>
            <h2 className="text-meta font-medium text-muted-foreground">{group.title}</h2>
            {group.items.map(item => (
              <Row key={item.label} label={item.label} note={item.note}>
                <OsTab {...item.props} />
              </Row>
            ))}
          </section>
        ))}
      </div>
    </div>
  );
}
