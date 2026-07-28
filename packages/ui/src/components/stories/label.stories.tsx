import type { Meta, StoryObj } from "@storybook/react-vite";

import { Input } from "../input";
import { Label } from "../label";

const meta: Meta<typeof Label> = {
  title: "components/ui/Label",
  component: Label,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Form label that pairs with inputs via htmlFor and respects peer-disabled state.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[320px] bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { children: "Session name", htmlFor: "session-name" },
};

export const WithInput: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor="session-field">Session name</Label>
      <Input id="session-field" placeholder="onboarding-run-1" />
    </div>
  ),
};
