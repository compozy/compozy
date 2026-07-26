import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { PageActionsTopbarSlot } from "../page-actions-topbar-slot";

const meta: Meta<typeof PageActionsTopbarSlot> = {
  title: "components/custom/PageActionsTopbarSlot",
  component: PageActionsTopbarSlot,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Topbar trailing slot carrying save / discard buttons for any page with dirty state. Both controls disable when `dirty === false` or while `saving` is true.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Clean — no unsaved changes. Both buttons disable. */
export const Clean: Story = {
  args: {
    dirty: false,
    onSave: fn(),
    onDiscard: fn(),
  },
};

/** Dirty — unsaved changes; both buttons enable. */
export const Dirty: Story = {
  args: {
    dirty: true,
    onSave: fn(),
    onDiscard: fn(),
  },
};

/** Saving — save button shows the spinner; both disable while the mutation is in-flight. */
export const Saving: Story = {
  args: {
    dirty: true,
    saving: true,
    onSave: fn(),
    onDiscard: fn(),
  },
};

/** Validation-blocked — Save stays focusable with aria-disabled + caption. */
export const SaveBlocked: Story = {
  args: {
    dirty: true,
    saveBlocked: true,
    saveBlockedCaption: "Fix 1 field before saving",
    onSave: fn(),
    onDiscard: fn(),
  },
};
