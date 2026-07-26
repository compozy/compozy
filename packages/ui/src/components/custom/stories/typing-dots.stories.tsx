import type { Meta, StoryObj } from "@storybook/react-vite";

import { TypingDots } from "@agh/ui";

const meta: Meta<typeof TypingDots> = {
  title: "components/custom/TypingDots",
  component: TypingDots,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Three-dot typing indicator, mirrors `.typing-dots` in `docs/design/web-inspiration/styles/app.css`. Relies on the `typing-bounce` keyframes in `packages/ui/src/tokens.css`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <span className="inline-flex items-center gap-2 font-mono text-eyebrow text-subtle">
      <TypingDots />
      <span>codex@laptop is typing...</span>
    </span>
  ),
};
