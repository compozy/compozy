import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { useState } from "react";
import { expect, waitFor, within } from "storybook/test";

import { EntityModeToolbar, type EntityMode } from "../entity-mode-toolbar";

const meta: Meta<typeof EntityModeToolbar> = {
  title: "components/custom/EntityModeToolbar",
  component: EntityModeToolbar,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full-width Simple/Advanced chrome strip for entity editors. Recessed `--color-canvas-tint` against the dialog's `--color-canvas-soft` so the switcher reads as chrome, not as the first form row. Advanced is the only disclosure tier and never hides a required field. The trailing slot is compact status — workspace scope belongs in the footer hint.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: ReactNode }) {
  return (
    <div className="p-6">
      <div className="w-(--width-modal-md) max-w-full overflow-hidden rounded-lg bg-canvas-soft">
        {children}
      </div>
    </div>
  );
}

function Harness({
  initialMode,
  withTrailing,
}: {
  initialMode: EntityMode;
  withTrailing: boolean;
}) {
  const [mode, setMode] = useState<EntityMode>(initialMode);
  return (
    <Frame>
      <EntityModeToolbar
        mode={mode}
        onModeChange={setMode}
        testIdPrefix="entity"
        trailing={
          withTrailing ? (
            <span className="font-mono text-form-label text-muted">overlay draft</span>
          ) : undefined
        }
      />
      <div className="px-6 py-5 text-small-body text-muted">
        {mode === "simple" ? "Common path only." : "Common path plus advanced disclosure."}
      </div>
    </Frame>
  );
}

export const Simple: Story = {
  args: { mode: "simple", onModeChange: () => {} },
  render: () => <Harness initialMode="simple" withTrailing />,
};

export const Advanced: Story = {
  args: { mode: "advanced", onModeChange: () => {} },
  render: () => <Harness initialMode="advanced" withTrailing />,
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
  render: () => <Harness initialMode="simple" withTrailing />,
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
  render: () => <Harness initialMode="simple" withTrailing={false} />,
};
