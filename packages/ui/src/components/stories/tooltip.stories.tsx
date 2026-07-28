import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../tooltip";
import { Button } from "../button";
import { Kbd, KbdGroup } from "../kbd";
import { UIProvider } from "../custom/ui-provider";

const meta: Meta<typeof Tooltip> = {
  title: "components/ui/Tooltip",
  component: Tooltip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Keyboard-friendly tooltip with motion-driven enter/exit. Wrap consumers in `TooltipProvider` and tune `delay` at the provider level.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function BasicTooltip() {
  return (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger render={<Button variant="outline">Hover me</Button>} />
        <TooltipContent>Renames this session locally</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

export const Default: Story = {
  render: () => <BasicTooltip />,
};

export const WithShortcut: Story = {
  render: () => (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger render={<Button variant="ghost">Run command</Button>} />
        <TooltipContent>
          Open palette
          <KbdGroup>
            <Kbd>⌘</Kbd>
            <Kbd>K</Kbd>
          </KbdGroup>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  ),
};

export const ReducedMotion: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "With `UIProvider reducedMotion='always'`, motion drops the scale transform and only opacity animates.",
      },
    },
  },
  render: () => (
    <UIProvider reducedMotion="always">
      <BasicTooltip />
    </UIProvider>
  ),
};

export const FocusOpens: Story = {
  render: () => (
    <TooltipProvider delay={0}>
      <Tooltip>
        <TooltipTrigger render={<Button>Focus me</Button>} />
        <TooltipContent>Focus reveals the tooltip</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = await canvas.findByRole("button", { name: "Focus me" });
    trigger.focus();
    const content = await waitFor(() =>
      within(document.body).getByText("Focus reveals the tooltip")
    );
    await expect(content).toBeInTheDocument();
    await userEvent.tab();
    await waitFor(
      () =>
        expect(
          within(document.body).queryByText("Focus reveals the tooltip")
        ).not.toBeInTheDocument(),
      { timeout: 2000 }
    );
  },
};
