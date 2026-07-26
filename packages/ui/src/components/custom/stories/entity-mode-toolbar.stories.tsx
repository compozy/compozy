import type { Meta, StoryObj } from "@storybook/react-vite";
import { Box, Globe } from "lucide-react";
import { useState } from "react";
import { expect, waitFor, within } from "storybook/test";

import { EntityModeToolbar, type EntityMode } from "../entity-mode-toolbar";
import { PillGroup } from "../pill-group";

const meta: Meta<typeof EntityModeToolbar> = {
  title: "components/custom/EntityModeToolbar",
  component: EntityModeToolbar,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Simple/Advanced toolbar for entity editors. Advanced is the only disclosure tier and never hides a required field. The trailing slot stays domain-free — web surfaces pass their `ScopeSelector` into it.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const SCOPE_ITEMS = [
  { value: "global", label: <Globe aria-hidden="true" className="size-3" /> },
  { value: "workspace", label: <Box aria-hidden="true" className="size-3" /> },
];

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <div className="p-6">
      <div className="w-(--width-modal-md) max-w-full overflow-hidden rounded-lg bg-canvas-soft">
        {children}
      </div>
    </div>
  );
}

function Harness({ initialMode, withScope }: { initialMode: EntityMode; withScope: boolean }) {
  const [mode, setMode] = useState<EntityMode>(initialMode);
  const [scope, setScope] = useState("workspace");
  return (
    <Frame>
      <EntityModeToolbar
        mode={mode}
        onModeChange={setMode}
        testIdPrefix="entity"
        trailing={
          withScope ? (
            <PillGroup
              aria-label="Scope"
              items={SCOPE_ITEMS}
              onChange={setScope}
              size="md"
              value={scope}
            />
          ) : undefined
        }
        trailingLabel={withScope ? "Scope" : undefined}
      />
      <div className="px-6 py-5 text-small-body text-muted">
        {mode === "simple" ? "Common path only." : "Common path plus advanced disclosure."}
      </div>
    </Frame>
  );
}

export const Simple: Story = {
  args: { mode: "simple", onModeChange: () => {} },
  render: () => <Harness initialMode="simple" withScope />,
};

export const Advanced: Story = {
  args: { mode: "advanced", onModeChange: () => {} },
  render: () => <Harness initialMode="advanced" withScope />,
};

export const FocusVisibleMode: Story = {
  args: { mode: "simple", onModeChange: () => {} },
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story: "Keyboard focus on a mode segment renders the 2px focus-visible indicator.",
      },
    },
  },
  render: () => <Harness initialMode="simple" withScope />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const advanced = await waitFor(() => canvas.getByTestId("entity-mode-advanced"));
    advanced.focus();
    await expect(advanced).toHaveFocus();
  },
};

export const WithoutTrailingControl: Story = {
  args: { mode: "simple", onModeChange: () => {} },
  parameters: {
    docs: {
      description: { story: "Editors for unscoped entities leave the trailing slot empty." },
    },
  },
  render: () => <Harness initialMode="simple" withScope={false} />,
};
