import type { Meta, StoryObj } from "@storybook/react-vite";
import { LayoutList, Package, ShoppingBag } from "lucide-react";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { PillGroup, type PillGroupItem } from "../pill-group";

const meta: Meta<typeof PillGroup> = {
  title: "components/custom/PillGroup",
  component: PillGroup,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Canonical segmented control. Rewritten — borderless `--canvas-soft` track at `--radius-md`, Inter sentence-case 12/510/-0.005em segments (no mono-uppercase), active state lifts to `--elevated` plus the `--highlight` inset shadow. Count badges render at 3px corners on the neutral `--badge-fill`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

type ViewMode = "list" | "kanban" | "dashboard" | "inbox";

const VIEW_ITEMS: ReadonlyArray<PillGroupItem<ViewMode>> = [
  { value: "list", label: "List", testId: "mode-list" },
  { value: "kanban", label: "Kanban", testId: "mode-kanban" },
  { value: "dashboard", label: "Dashboard", testId: "mode-dashboard" },
  { value: "inbox", label: "Inbox", badge: 3, testId: "mode-inbox" },
];

function PillGroupHarness({ initial = "list" as ViewMode }: { initial?: ViewMode }) {
  const [value, setValue] = useState<ViewMode>(initial);
  return <PillGroup value={value} onChange={setValue} items={VIEW_ITEMS} aria-label="Task views" />;
}

function SmallPillGroupHarness() {
  const [value, setValue] = useState<"a" | "b" | "c">("a");
  return (
    <PillGroup
      value={value}
      onChange={setValue}
      size="sm"
      items={[
        { value: "a", label: "Alpha" },
        { value: "b", label: "Beta" },
        { value: "c", label: "Gamma" },
      ]}
    />
  );
}

export const Default: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Default `md` size with the first segment active. Renders the new sentence-case Inter label and the lifted `--elevated` + `--highlight` active surface.",
      },
    },
  },
  render: () => <PillGroupHarness />,
};

export const ActiveSecond: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Active state on the middle segment — confirms the `--highlight` inset shadow lifts the chip evenly regardless of position.",
      },
    },
  },
  render: () => <PillGroupHarness initial="kanban" />,
};

export const WithCounts: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Segments with count badges. Badges render as 3px-radius neutral chips on `--badge-fill` with `--muted` text and tabular-nums (replaces the prior solid-accent treatment).",
      },
    },
  },
  render: () => <PillGroupHarness initial="inbox" />,
};

export const Selection: Story = {
  render: () => <PillGroupHarness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const list = await canvas.findByTestId("mode-list");
    const kanban = await canvas.findByTestId("mode-kanban");

    await expect(list).toHaveAttribute("aria-pressed", "true");
    await expect(kanban).toHaveAttribute("aria-pressed", "false");

    await userEvent.click(kanban);
    await waitFor(() => expect(kanban).toHaveAttribute("aria-pressed", "true"));
    await expect(list).toHaveAttribute("aria-pressed", "false");
  },
};

export const SizeSm: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "`size='sm'` shrinks the segment height to `--height-pill-group-segment-sm` (20px) while keeping the same Inter type ramp.",
      },
    },
  },
  render: () => <SmallPillGroupHarness />,
};

type ScopeMode = "installed" | "market" | "list";

function IconsPillGroupHarness() {
  const [value, setValue] = useState<ScopeMode>("installed");
  return (
    <PillGroup
      value={value}
      onChange={setValue}
      aria-label="Scope with icons"
      items={[
        {
          value: "installed",
          label: (
            <>
              <Package aria-hidden="true" />
              Installed
            </>
          ),
          badge: 2,
          testId: "scope-installed",
        },
        {
          value: "market",
          label: (
            <>
              <ShoppingBag aria-hidden="true" />
              Marketplace
            </>
          ),
          testId: "scope-market",
        },
        {
          value: "list",
          label: (
            <>
              <LayoutList aria-hidden="true" />
              List
            </>
          ),
          testId: "scope-list",
        },
      ]}
    />
  );
}

export const WithIcons: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Icon + label (+ optional badge) composition. Segment uses `leading-none` and `[&_svg]` sizing so Lucide icons sit on the same optical center as the Inter label.",
      },
    },
  },
  render: () => <IconsPillGroupHarness />,
};

export const DisabledItem: Story = {
  parameters: {
    docs: {
      description: {
        story: "A disabled segment is non-interactive and dimmed; `onChange` will not fire.",
      },
    },
  },
  render: () => (
    <PillGroup
      value="list"
      onChange={() => {}}
      items={[
        { value: "list", label: "List" },
        { value: "kanban", label: "Kanban", disabled: true },
      ]}
    />
  ),
};
