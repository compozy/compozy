import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "../dialog";
import { Button } from "../button";
import { Input } from "../input";
import { Label } from "../label";
import { UIProvider } from "../custom/ui-provider";

const meta: Meta<typeof Dialog> = {
  title: "components/ui/Dialog",
  component: Dialog,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Modal dialog built on Base UI with motion-driven enter/exit animations via `AnimatePresence`. Respects the `reducedMotion` setting from `UIProvider`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function BasicDialog() {
  return (
    <Dialog>
      <DialogTrigger render={<Button>Rename task</Button>} />
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rename task</DialogTitle>
          <DialogDescription>
            Update the display name of the currently selected task.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-2 px-0 py-2">
          <Label htmlFor="task-name">Name</Label>
          <Input id="task-name" defaultValue="Spin up agent" />
        </div>
        <DialogFooter>
          <DialogClose render={<Button variant="ghost">Cancel</Button>} />
          <DialogClose render={<Button>Save</Button>} />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export const Default: Story = {
  args: {},
  render: () => <BasicDialog />,
};

export const ReducedMotion: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          "With `UIProvider reducedMotion='always'`, motion drops the scale transform and animates only opacity on open/close.",
      },
    },
  },
  render: () => (
    <UIProvider reducedMotion="always">
      <BasicDialog />
    </UIProvider>
  ),
};

export const OpenAndFocus: Story = {
  args: {},
  render: () => <BasicDialog />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = await canvas.findByRole("button", { name: "Rename task" });
    await userEvent.click(trigger);
    const dialog = await waitFor(() => within(document.body).getByRole("dialog"));
    await expect(dialog).toBeInTheDocument();
    const input = await within(document.body).findByLabelText("Name");
    await expect(input).toHaveValue("Spin up agent");
    await userEvent.keyboard("{Escape}");
    await waitFor(
      () => expect(within(document.body).queryByRole("dialog")).not.toBeInTheDocument(),
      { timeout: 2000 }
    );
  },
};

export const HiddenCloseButton: Story = {
  args: {},
  render: () => (
    <Dialog>
      <DialogTrigger render={<Button variant="outline">Open</Button>} />
      <DialogContent showCloseButton={false}>
        <DialogTitle>Confirm</DialogTitle>
        <DialogDescription>The close button is hidden.</DialogDescription>
        <DialogFooter>
          <DialogClose render={<Button>Got it</Button>} />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ),
};

export const RuledHeader: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          '`DialogHeader variant="ruled"` adds the panel-tone background and `border-b border-line` rule with the canonical `px-5 py-4` rhythm. Pair with `DialogContent unframed` so the dialog body owns its own padding.',
      },
    },
  },
  render: () => (
    <Dialog defaultOpen>
      <DialogContent unframed className="max-w-md" data-testid="ruled-header-dialog">
        <DialogHeader variant="ruled">
          <DialogTitle>Add workspace</DialogTitle>
          <DialogDescription>
            Pick the global home directory or register a manual workspace path.
          </DialogDescription>
        </DialogHeader>
        <div className="p-5 text-sm text-muted">Body content lives below the rule.</div>
      </DialogContent>
    </Dialog>
  ),
};

export const RuledFooter: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          '`DialogFooter variant="ruled"` matches the header rule with `border-t border-line` and `px-5 py-3` rhythm so callers can compose flush primary/secondary actions inside `DialogContent unframed`.',
      },
    },
  },
  render: () => (
    <Dialog defaultOpen>
      <DialogContent unframed className="max-w-md" data-testid="ruled-footer-dialog">
        <div className="p-5">
          <DialogTitle className="text-base font-medium">Discard draft?</DialogTitle>
          <DialogDescription className="mt-2">
            This task draft has not been saved. Discarding will remove it from the inbox.
          </DialogDescription>
        </div>
        <DialogFooter variant="ruled">
          <DialogClose render={<Button variant="ghost">Cancel</Button>} />
          <DialogClose render={<Button>Discard</Button>} />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ),
};

export const Unframed: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          "`DialogContent unframed` removes the default `gap-4 p-4` chrome. Useful when the consumer composes its own header, body, and footer padding (typically alongside the ruled header/footer variants).",
      },
    },
  },
  render: () => (
    <Dialog defaultOpen>
      <DialogContent unframed className="max-w-md" data-testid="unframed-dialog">
        <div data-testid="unframed-body" className="flex flex-col gap-3 p-5 text-sm text-muted">
          <DialogTitle className="text-base font-medium text-fg">Composed body</DialogTitle>
          <p>The host owns gutters and rhythm -- the dialog ships only the canvas + border.</p>
        </div>
      </DialogContent>
    </Dialog>
  ),
};

export const RuledFull: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story: "Ruled header + ruled footer + unframed body -- the canonical AGH dialog chrome.",
      },
    },
  },
  render: () => (
    <Dialog defaultOpen>
      <DialogContent unframed className="max-w-md" data-testid="ruled-full-dialog">
        <DialogHeader variant="ruled">
          <DialogTitle>Connect workspace</DialogTitle>
          <DialogDescription>
            Confirm the workspace path before AGH starts the daemon.
          </DialogDescription>
        </DialogHeader>
        <div className="p-5">
          <Label htmlFor="ruled-full-name">Workspace path</Label>
          <Input id="ruled-full-name" defaultValue="/Users/pedro/Dev/agh" />
        </div>
        <DialogFooter variant="ruled">
          <DialogClose render={<Button variant="ghost">Cancel</Button>} />
          <DialogClose render={<Button>Connect</Button>} />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ),
};
