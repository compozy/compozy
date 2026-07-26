import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "../sheet";
import { Button } from "../button";
import { Input } from "../input";
import { Label } from "../label";
import { UIProvider } from "../custom/ui-provider";

const meta: Meta<typeof Sheet> = {
  title: "components/ui/Sheet",
  component: Sheet,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Side sheet for supplementary editing. Choose a side via the Content `side` prop; motion drives the slide-in/out animation and respects the `UIProvider` reduced-motion setting.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function AgentPresetSheet() {
  return (
    <Sheet>
      <SheetTrigger render={<Button variant="outline">Open sheet</Button>} />
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Edit agent preset</SheetTitle>
          <SheetDescription>Adjust the defaults applied to new sessions.</SheetDescription>
        </SheetHeader>
        <div className="grid gap-4 px-4">
          <div className="grid gap-2">
            <Label htmlFor="sheet-model">Model</Label>
            <Input id="sheet-model" defaultValue="claude-opus-4-7" />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="sheet-temp">Temperature</Label>
            <Input id="sheet-temp" defaultValue="0.3" />
          </div>
        </div>
        <SheetFooter>
          <SheetClose render={<Button variant="ghost">Cancel</Button>} />
          <SheetClose render={<Button>Save preset</Button>} />
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

export const RightSide: Story = {
  render: () => <AgentPresetSheet />,
};

export const LeftSide: Story = {
  render: () => (
    <Sheet>
      <SheetTrigger render={<Button variant="outline">Open from left</Button>} />
      <SheetContent side="left">
        <SheetHeader>
          <SheetTitle>Filters</SheetTitle>
          <SheetDescription>Scope the dashboard to a specific workspace.</SheetDescription>
        </SheetHeader>
        <div className="px-4 text-sm text-muted-foreground">
          Filters stay in place until cleared so long runs stay visible.
        </div>
        <SheetFooter>
          <SheetClose render={<Button>Apply</Button>} />
        </SheetFooter>
      </SheetContent>
    </Sheet>
  ),
};

export const BottomSide: Story = {
  render: () => (
    <Sheet>
      <SheetTrigger render={<Button variant="outline">Open from bottom</Button>} />
      <SheetContent side="bottom">
        <SheetHeader>
          <SheetTitle>Session log</SheetTitle>
          <SheetDescription>Review recent agent events.</SheetDescription>
        </SheetHeader>
        <div className="px-4 text-sm text-muted-foreground">
          Detailed log output lives here while the sheet is open.
        </div>
      </SheetContent>
    </Sheet>
  ),
};

export const ReducedMotion: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "With `UIProvider reducedMotion='always'`, motion drops the slide transform so only opacity fades.",
      },
    },
  },
  render: () => (
    <UIProvider reducedMotion="always">
      <AgentPresetSheet />
    </UIProvider>
  ),
};

export const OpenAndClose: Story = {
  render: () => <AgentPresetSheet />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = await canvas.findByRole("button", { name: "Open sheet" });
    await userEvent.click(trigger);
    const dialog = await waitFor(() => within(document.body).getByRole("dialog"));
    await expect(dialog).toHaveAttribute("data-side", "right");
    await userEvent.keyboard("{Escape}");
    await waitFor(
      () => expect(within(document.body).queryByRole("dialog")).not.toBeInTheDocument(),
      { timeout: 2000 }
    );
  },
};
