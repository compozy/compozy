import type { Meta, StoryObj } from "@storybook/react-vite";
import { Check, ClipboardCheck } from "lucide-react";
import { expect, waitFor, within } from "storybook/test";

import { Dialog, DialogContent } from "../../dialog";
import { dialogShellClass } from "../../../lib/dialog-shell";
import { EntityDialogFooter } from "../entity-dialog-footer";
import { EntityDialogHeader } from "../entity-dialog-header";

const meta: Meta<typeof EntityDialogFooter> = {
  title: "components/custom/EntityDialogFooter",
  component: EntityDialogFooter,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Modal footer contract: an optional consequence hint on the left, then Cancel and exactly one verb+object primary action. `EditorFooter` remains the page/sheet editor footer.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function FooterHost({ children }: { children: React.ReactNode }) {
  return (
    <Dialog defaultOpen>
      <DialogContent
        className={dialogShellClass("md")}
        data-testid="entity-dialog-footer-host"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader eyebrow="Autonomy · Task" icon={ClipboardCheck} title="Create task" />
        <div className="px-6 py-5 text-small-body text-muted">Body region.</div>
        {children}
      </DialogContent>
    </Dialog>
  );
}

export const Default: Story = {
  args: { onCancel: () => {}, primaryLabel: "Enqueue task" },
  render: () => (
    <FooterHost>
      <EntityDialogFooter
        hint="The contract is durable; runs descend from this task and respect dependencies."
        onCancel={() => {}}
        primaryIcon={Check}
        primaryLabel="Enqueue task"
      />
    </FooterHost>
  ),
};

export const WithoutHint: Story = {
  args: { onCancel: () => {}, primaryLabel: "Save changes" },
  render: () => (
    <FooterHost>
      <EntityDialogFooter onCancel={() => {}} primaryLabel="Save changes" />
    </FooterHost>
  ),
};

export const Saving: Story = {
  args: { onCancel: () => {}, primaryLabel: "Enqueue task" },
  parameters: {
    docs: { description: { story: "Saving swaps the glyph for a spinner and blocks re-submit." } },
  },
  render: () => (
    <FooterHost>
      <EntityDialogFooter
        cancelDisabled
        hint="The contract is durable; runs descend from this task."
        isSaving
        onCancel={() => {}}
        primaryIcon={Check}
        primaryLabel="Enqueue task"
      />
    </FooterHost>
  ),
};

export const FocusVisibleActions: Story = {
  args: { onCancel: () => {}, primaryLabel: "Enqueue task" },
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story: "Keyboard focus on the primary action renders the 2px focus-visible indicator.",
      },
    },
  },
  render: () => (
    <FooterHost>
      <EntityDialogFooter
        hint="The contract is durable; runs descend from this task."
        onCancel={() => {}}
        primaryIcon={Check}
        primaryLabel="Enqueue task"
        primaryTestId="focus-primary"
      />
    </FooterHost>
  ),
  play: async () => {
    const body = within(document.body);
    const primary = await waitFor(() => body.getByTestId("focus-primary"));
    primary.focus();
    await expect(primary).toHaveFocus();
  },
};

export const Disabled: Story = {
  args: { onCancel: () => {}, primaryLabel: "Create channel" },
  parameters: {
    docs: { description: { story: "Submit stays gated until the draft satisfies its contract." } },
  },
  render: () => (
    <FooterHost>
      <EntityDialogFooter
        hint="Members cannot change after the channel is created."
        onCancel={() => {}}
        primaryDisabled
        primaryLabel="Create channel"
      />
    </FooterHost>
  ),
};
