import type { Meta, StoryObj } from "@storybook/react-vite";
import { DropdownMenuItem } from "../../dropdown-menu";
import { SplitButton } from "../split-button";

const meta: Meta<typeof SplitButton> = {
  title: "components/custom/SplitButton",
  component: SplitButton,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "A computed primary action paired with the menu of its alternatives. Both segments share one variant and size; the chevron stays operable while the action is blocked, because the alternatives are what an operator needs when the primary cannot run.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function alternatives() {
  return (
    <>
      <DropdownMenuItem>Push</DropdownMenuItem>
      <DropdownMenuItem>Open pull request</DropdownMenuItem>
      <DropdownMenuItem>View pull request</DropdownMenuItem>
    </>
  );
}

/** Default exit action with its available alternatives. */
export const Default: Story = {
  args: {
    label: "Commit",
    menuLabel: "More exit actions",
    onAction: () => {},
    children: alternatives(),
  },
};

/**
 * Blocked keeps the action focusable so its reason is announced, and leaves the
 * chevron operable so an alternative is still one keystroke away.
 */
export const Blocked: Story = {
  args: {
    ...Default.args,
    blocked: true,
    blockedReason: "Git actions are paused while a bound session is running.",
  },
};

/** Hard-disabled: reserved for work already in flight, never for policy. */
export const Disabled: Story = {
  args: { ...Default.args, disabled: true },
};

/** With no alternatives the chevron is absent — an empty menu is fake interactivity. */
export const WithoutMenu: Story = {
  args: { label: "Commit", menuLabel: "More exit actions", onAction: () => {} },
};

export const Sizes: Story = {
  args: Default.args,
  render: args => (
    <div className="flex items-center gap-3">
      <SplitButton {...args} size="xs" />
      <SplitButton {...args} size="sm" />
      <SplitButton {...args} size="default" />
    </div>
  ),
};
