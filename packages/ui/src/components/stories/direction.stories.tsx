import type { Meta, StoryObj } from "@storybook/react-vite";

import { DirectionProvider } from "../direction";

const meta: Meta<typeof DirectionProvider> = {
  title: "components/ui/Direction",
  component: DirectionProvider,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Wraps descendants in a text-direction context. Use around feature blocks that need RTL-aware Base UI behavior and DOM `dir` inheritance.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const paragraph =
  "AGH streams every agent turn through the daemon. Events are persisted in SQLite and replayed on reconnect.";

export const LeftToRight: Story = {
  args: {},
  render: () => (
    <DirectionProvider direction="ltr">
      <div dir="ltr" className="max-w-md rounded-lg border p-4 text-sm leading-relaxed">
        {paragraph}
      </div>
    </DirectionProvider>
  ),
};

export const RightToLeft: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          'Same paragraph wrapped with `dir="rtl"` so punctuation, scroll affordances, and Base UI positioning mirror correctly.',
      },
    },
  },
  render: () => (
    <DirectionProvider direction="rtl">
      <div dir="rtl" className="max-w-md rounded-lg border p-4 text-sm leading-relaxed">
        {paragraph}
      </div>
    </DirectionProvider>
  ),
};
