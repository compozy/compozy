import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { Button } from "../button";
import { UIProvider } from "../custom/ui-provider";
import { HoverCard, HoverCardContent, HoverCardTrigger } from "../hover-card";
import { PropertyRow } from "../custom/property-row";

const meta: Meta<typeof HoverCard> = {
  title: "components/ui/HoverCard",
  component: HoverCard,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Read-only preview for the thing under the pointer: opens after a 200 ms hover or at once on focus, closes on leave, blur or Escape. `Tooltip` stays text-only; `Popover` stays click-owned.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function PreviewBody() {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-transcript-body font-semibold text-fg-strong">
        Audit payment webhooks for retry safety
      </span>
      <div className="flex flex-col">
        <PropertyRow label="Model" mono>
          opus-5.5
        </PropertyRow>
        <PropertyRow label="Effort">high</PropertyRow>
      </div>
    </div>
  );
}

export const Default: Story = {
  render: () => (
    <HoverCard>
      <HoverCardTrigger render={<Button variant="outline">Hover me</Button>} />
      <HoverCardContent>
        <PreviewBody />
      </HoverCardContent>
    </HoverCard>
  ),
};

export const Open: Story = {
  parameters: {
    docs: { description: { story: "The surface: elevated, line-strong, shadow-overlay, 300px." } },
  },
  render: () => (
    <HoverCard defaultOpen>
      <HoverCardTrigger render={<Button variant="outline">Pinned open</Button>} />
      <HoverCardContent>
        <PreviewBody />
      </HoverCardContent>
    </HoverCard>
  ),
};

export const ReducedMotion: Story = {
  render: () => (
    <UIProvider reducedMotion="always">
      <HoverCard>
        <HoverCardTrigger render={<Button variant="outline">Hover me</Button>} />
        <HoverCardContent>
          <PreviewBody />
        </HoverCardContent>
      </HoverCard>
    </UIProvider>
  ),
};

export const FocusOpensEscapeCloses: Story = {
  tags: ["play-fn"],
  render: () => (
    <HoverCard>
      <HoverCardTrigger render={<Button>Focus me</Button>} />
      <HoverCardContent>
        <PreviewBody />
      </HoverCardContent>
    </HoverCard>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const body = within(document.body);
    const trigger = await canvas.findByRole("button", { name: "Focus me" });
    trigger.focus();
    await waitFor(() =>
      expect(body.getByText("Audit payment webhooks for retry safety")).toBeInTheDocument()
    );
    await userEvent.keyboard("{Escape}");
    await waitFor(() =>
      expect(body.queryByText("Audit payment webhooks for retry safety")).not.toBeInTheDocument()
    );
  },
};
