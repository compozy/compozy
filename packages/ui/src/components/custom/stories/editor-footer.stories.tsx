import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "@agh/ui";
import { EditorFooter } from "../editor-footer";

const meta: Meta<typeof EditorFooter> = {
  title: "components/custom/EditorFooter",
  component: EditorFooter,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Sticky editor footer used inside dialog editors and split panes. Background = `--canvas` (recessed step), top rule = `--line`, meta on the left, secondary actions middle, primary action right. Optional Escape handler for dismissal.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[640px] bg-background">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Save bar pattern — meta on the left, ghost cancel + accent save on the right.
 */
export const SaveBar: Story = {
  args: {},
  render: () => (
    <EditorFooter
      meta="Edited 2 fields. Press ⌘S to save."
      secondary={
        <Button variant="ghost" size="sm">
          Cancel
        </Button>
      }
      primary={<Button size="sm">Save changes</Button>}
    />
  ),
};

/**
 * Read-only footer — only meta + secondary, no primary action.
 */
export const ReadOnly: Story = {
  args: {},
  render: () => (
    <EditorFooter
      meta="Read-only view"
      secondary={
        <Button variant="outline" size="sm">
          Close
        </Button>
      }
    />
  ),
};
