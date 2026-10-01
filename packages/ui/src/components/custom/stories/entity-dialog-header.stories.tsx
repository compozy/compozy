import type { Meta, StoryObj } from "@storybook/react-vite";
import { CalendarClock, ClipboardCheck, KeyRound, Trash2 } from "lucide-react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { Dialog, DialogContent } from "../../dialog";
import { dialogShellClass } from "../../../lib/dialog-shell";
import { UIProvider } from "../ui-provider";
import { EntityDialogHeader } from "../entity-dialog-header";

const meta: Meta<typeof EntityDialogHeader> = {
  title: "components/custom/EntityDialogHeader",
  component: EntityDialogHeader,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Canonical entity-editor modal header: the mint identity well beside a muted eyebrow, the dialog title, an optional description, and a quiet close control. `tone` follows the shared dialog tone model (`neutral` default; `danger` / `warning` keep semantic wells), so an entity editor never reads as needing attention.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function HeaderHost({ children }: { children: React.ReactNode }) {
  return (
    <Dialog defaultOpen>
      <DialogContent
        className={dialogShellClass("md")}
        data-testid="entity-dialog-header-host"
        showCloseButton={false}
        unframed
      >
        {children}
        <div className="px-6 py-5 text-small-body text-muted">Body region.</div>
      </DialogContent>
    </Dialog>
  );
}

export const Default: Story = {
  args: { eyebrow: "Autonomy · Task", icon: ClipboardCheck, title: "Create task" },
  render: () => (
    <HeaderHost>
      <EntityDialogHeader
        description="A task is a durable contract — a unit of work that gets claimed and run by an owner. Runs descend from it and respect its dependencies."
        eyebrow="Autonomy · Task"
        icon={ClipboardCheck}
        title="Create task"
      />
    </HeaderHost>
  ),
};

export const WithoutDescription: Story = {
  args: { eyebrow: "System · Vault", icon: KeyRound, title: "Add vault secret" },
  render: () => (
    <HeaderHost>
      <EntityDialogHeader eyebrow="System · Vault" icon={KeyRound} title="Add vault secret" />
    </HeaderHost>
  ),
};

/** No eyebrow: the title already names the entity, so the header stays one line. */
export const WithoutEyebrow: Story = {
  args: { icon: KeyRound, title: "New secret" },
  render: () => (
    <HeaderHost>
      <EntityDialogHeader icon={KeyRound} title="New secret" />
    </HeaderHost>
  ),
};

export const FocusVisibleClose: Story = {
  args: { eyebrow: "Autonomy · Task", icon: ClipboardCheck, title: "Create task" },
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story: "Keyboard focus on the close control renders the 2px focus-visible indicator.",
      },
    },
  },
  render: () => (
    <HeaderHost>
      <EntityDialogHeader
        description="A task is a durable contract."
        eyebrow="Autonomy · Task"
        icon={ClipboardCheck}
        onClose={() => {}}
        title="Create task"
      />
    </HeaderHost>
  ),
  play: async () => {
    const body = within(document.body);
    const close = await waitFor(() => body.getByRole("button", { name: "Close" }));
    close.focus();
    await userEvent.keyboard("{Tab}{Shift>}{Tab}{/Shift}");
    await expect(close).toHaveFocus();
  },
};

export const ReducedMotion: Story = {
  args: { eyebrow: "System · Vault", icon: KeyRound, title: "Add vault secret" },
  parameters: {
    docs: {
      description: {
        story:
          'Open state under `reducedMotion="always"` — transforms are dropped and durations collapse without losing state clarity.',
      },
    },
  },
  render: () => (
    <UIProvider reducedMotion="always">
      <HeaderHost>
        <EntityDialogHeader
          description="Store a write-only value under a stable reference."
          eyebrow="System · Vault"
          icon={KeyRound}
          onClose={() => {}}
          title="Add vault secret"
        />
      </HeaderHost>
    </UIProvider>
  ),
};

export const WithClose: Story = {
  args: { eyebrow: "Automation · Job", icon: CalendarClock, title: "Create job" },
  parameters: {
    docs: {
      description: {
        story: "Hosts that suppress the dialog close button can opt into a header-owned control.",
      },
    },
  },
  render: () => (
    <HeaderHost>
      <EntityDialogHeader
        description={
          <>
            A job runs an agent, materializes a task, or starts a Loop on a schedule.{" "}
            <b className="font-medium text-muted">Choose the target and when it should run.</b>
          </>
        }
        eyebrow="Automation · Job"
        icon={CalendarClock}
        onClose={() => {}}
        title="Create job"
      />
    </HeaderHost>
  ),
};

/** A destructive subject keeps its semantic well and eyebrow through `tone`. */
export const DangerTone: Story = {
  args: { eyebrow: "System · Vault", icon: Trash2, title: "Remove vault secret", tone: "danger" },
  render: () => (
    <HeaderHost>
      <EntityDialogHeader
        description="Sessions that reference this secret stop resolving it."
        eyebrow="System · Vault"
        icon={Trash2}
        onClose={() => {}}
        title="Remove vault secret"
        tone="danger"
      />
    </HeaderHost>
  ),
};
