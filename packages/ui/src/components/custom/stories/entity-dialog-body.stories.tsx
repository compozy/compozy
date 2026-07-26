import type { Meta, StoryObj } from "@storybook/react-vite";
import { Layers } from "lucide-react";

import { Dialog, DialogContent } from "../../dialog";
import { dialogShellClass } from "../../../lib/dialog-shell";
import { EntityDialogBody } from "../entity-dialog-body";
import { EntityDialogFooter } from "../entity-dialog-footer";
import { EntityDialogHeader } from "../entity-dialog-header";
import { FormSection } from "../form-section";

const meta: Meta<typeof EntityDialogBody> = {
  title: "components/custom/EntityDialogBody",
  component: EntityDialogBody,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          'Modal body region and sole scroll owner. `variant="split"` hosts a body carrying a large expanding component: a 1fr main pane and a 420px side pane on canvas, each scrolling independently, collapsing to one column below 980px.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Placeholder({ label }: { label: string }) {
  return (
    <div className="rounded-md bg-canvas px-3 py-6 text-center text-form-hint text-subtle">
      {label}
    </div>
  );
}

export const Default: Story = {
  args: {},
  render: () => (
    <Dialog defaultOpen>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] ${dialogShellClass("md")}`}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader eyebrow="Workspace" icon={Layers} title="Add workspace" />
        <EntityDialogBody>
          <FormSection title="Location">
            <Placeholder label="Directory browser" />
          </FormSection>
        </EntityDialogBody>
        <EntityDialogFooter onCancel={() => {}} primaryLabel="Add workspace" />
      </DialogContent>
    </Dialog>
  ),
};

export const Split: Story = {
  args: {},
  parameters: {
    docs: {
      description: { story: "The `xl` host pairs with the split body for workspace-scale tasks." },
    },
  },
  render: () => (
    <Dialog defaultOpen>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] ${dialogShellClass("xl", { fill: true })}`}
        data-testid="entity-dialog-body-split"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description="Register a folder as a workspace and set the defaults every session inherits."
          eyebrow="Workspace"
          icon={Layers}
          title="Add workspace"
        />
        <EntityDialogBody
          side={
            <FormSection title="Session defaults">
              <Placeholder label="Default agent" />
              <Placeholder label="Sandbox profile" />
            </FormSection>
          }
          variant="split"
        >
          <FormSection title="Location">
            <Placeholder label="Directory browser" />
          </FormSection>
        </EntityDialogBody>
        <EntityDialogFooter
          hint="The root folder cannot change after the workspace is registered."
          onCancel={() => {}}
          primaryLabel="Add workspace"
        />
      </DialogContent>
    </Dialog>
  ),
};
