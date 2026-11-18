import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { fn } from "storybook/test";

import { OsTab, type OsTabProps } from "../os-tab";
import { STORY_WINDOWS as W, storySessionOf } from "./_tab-fixtures";

const meta: Meta = {
  title: "systems/os/components/OsTab",
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "One plate per tab on the bar's own surface: an icon tile per window (a split stacks up to three, the lead in front), the lead window's title, the member count, the loudest signal, the waiting count, and a close button. A narrowing tab drops them in that order from the end; a pinned tab is its tile alone.",
      },
    },
  },
};

export default meta;
type Story = StoryObj;

type SpecimenProps = Omit<OsTabProps, "sessionOf" | "onActivate" | "onClose">;

interface Specimen {
  label: string;
  note?: string;
  props: SpecimenProps;
}

function spec(
  label: string,
  members: SpecimenProps["members"],
  options: { note?: string; width?: number; active?: boolean; pinned?: boolean } = {}
): Specimen {
  const { note, width, active = false, pinned = false } = options;
  return { label, note, props: { tabId: members[0]!.id, members, pinned, active, width } };
}

const FOUR = [W.tasks, W.terminal, W.files, W.agents];

const GROUPS: { title: string; items: Specimen[] }[] = [
  {
    title: "States",
    items: [
      spec("Rest", [W.auth], { width: 200 }),
      spec("Active", [W.auth], { width: 200, active: true }),
    ],
  },
  {
    title: "Splits",
    items: [
      spec("Two windows", [W.tasks, W.terminal], { width: 224 }),
      spec("Three windows", [W.tabs, W.terminal, W.files], { width: 224 }),
      spec("Four or more", FOUR, {
        note: "Three tiles, the count says the rest",
        width: 224,
        active: true,
      }),
    ],
  },
  {
    title: "Signals",
    items: [
      spec("Working", [W.tabs], { width: 224 }),
      spec("Needs you", [W.review], { note: "Glyph and the waiting count", width: 224 }),
    ],
  },
  {
    title: "Pinned",
    items: [
      spec("Pinned", [W.dash], { pinned: true }),
      spec("Pinned, active", [W.agents], { pinned: true, active: true }),
      spec("Pinned split", [W.dash, W.tasks], { pinned: true }),
      spec("Pinned, needs you", [W.review], {
        note: "The signal rides the tile's corner",
        pinned: true,
      }),
    ],
  },
  {
    title: "Shrinking",
    items: [
      spec("224px, the widest", [W.review], { width: 224 }),
      spec("Under 160px", FOUR, { note: "The member count goes", width: 152 }),
      spec("Under 144px", [W.review], { note: "The waiting count goes", width: 136 }),
      spec("Under 120px", [W.review], { note: "Signal moves to the icon, no close", width: 112 }),
      spec("Under 72px", [W.review], { note: "Icon only", width: 64 }),
      spec("40px floor", [W.docs], { width: 40 }),
      spec("Active, narrow", [W.docs], { note: "Keeps its close button", width: 96, active: true }),
      spec("Active, under 88px", [W.docs], { note: "Icon and close", width: 72, active: true }),
      spec("Active at the floor", [W.docs], {
        note: "Icon only, like the rest",
        width: 40,
        active: true,
      }),
    ],
  },
];

function Row({ label, note, children }: { label: string; note?: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-6">
      <div className="w-44 shrink-0">
        <div className="text-body text-fg">{label}</div>
        {note ? <div className="text-meta text-subtle">{note}</div> : null}
      </div>
      <div className="flex h-10 items-center rounded-md bg-rail px-1.5" role="tablist">
        {children}
      </div>
    </div>
  );
}

/** Every tab variant at fixed widths, grouped as the window-tabs tab sheet draws them. */
export const TabSheet: Story = {
  render: () => (
    <div className="flex flex-col gap-8">
      {GROUPS.map(group => (
        <section key={group.title} className="flex flex-col gap-3">
          <h3 className="eyebrow">{group.title}</h3>
          {group.items.map(item => (
            <Row key={item.label} label={item.label} note={item.note}>
              <OsTab {...item.props} sessionOf={storySessionOf} onActivate={fn()} onClose={fn()} />
            </Row>
          ))}
        </section>
      ))}
    </div>
  ),
};
