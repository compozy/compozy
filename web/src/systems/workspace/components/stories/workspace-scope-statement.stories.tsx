import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { WorkspaceScopeStatement } from "../workspace-scope-statement";

const meta: Meta<typeof WorkspaceScopeStatement> = {
  title: "systems/workspace/components/WorkspaceScopeStatement",
  component: WorkspaceScopeStatement,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Read-only landing statement for create, install, session, and edit surfaces. Scope changes happen at the menubar — this never opens a picker.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface className="p-6">
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Chip used in editor toolbars; workspace create omits the trailing period. */
export const CreateWorkspaceChip: Story = {
  args: {
    destination: "alpha",
    kind: "create",
    variant: "chip",
  },
};

/** Note used in dialog footers; Global create keeps the visibility clause. */
export const CreateGlobalNote: Story = {
  args: {
    destination: "Global",
    kind: "create",
    variant: "note",
  },
};

/** Install landing for a Global MCP or extension. */
export const InstallGlobalNote: Story = {
  args: {
    destination: "Global",
    kind: "install",
    variant: "note",
  },
};

/** Session create names the folder the session will run in. */
export const SessionWorkspaceNote: Story = {
  args: {
    destination: "alpha",
    kind: "session",
    root: "/workspace/alpha",
    variant: "note",
  },
};

/** Edit mode states the existing record's destination. */
export const EditChip: Story = {
  args: {
    destination: "alpha",
    kind: "edit",
    variant: "chip",
  },
};
