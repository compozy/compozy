import type { Meta, StoryObj } from "@storybook/react-vite";
import { Clock3 } from "lucide-react";

import { Button } from "../../button";
import { CatalogEmptyDisclosureRow } from "../catalog-empty-disclosure-row";
import { Pill } from "../pill";

const meta: Meta<typeof CatalogEmptyDisclosureRow> = {
  title: "components/custom/CatalogEmptyDisclosureRow",
  component: CatalogEmptyDisclosureRow,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Compact catalog row with a disclosure opener, an independent action rail, and optional status feedback.",
      },
    },
  },
  decorators: [
    Story => (
      <ul className="w-full max-w-180 overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <Story />
      </ul>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Shows the default collapsed row; open it to inspect the detail panel. */
export const Default: Story = {
  args: {},
  render: () => (
    <CatalogEmptyDisclosureRow
      actions={
        <Button size="sm" type="button" variant="neutral">
          Use suggestion
        </Button>
      }
      description="Runs every weekday after the stand-up."
      detail={
        <div className="rounded-md border border-line-soft bg-input-fill px-3 py-2.5 text-form-label text-fg">
          Summarize decisions, blockers, and owners from the latest session.
        </div>
      }
      icon={<Clock3 aria-hidden="true" className="size-3.5" />}
      pill={
        <Pill mono size="sm" tone="neutral">
          Weekdays
        </Pill>
      }
      title="Daily stand-up summary"
      tone="info"
    />
  ),
};
