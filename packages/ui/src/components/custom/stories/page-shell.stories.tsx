import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "@agh/ui";
import { ActionResultBanner } from "../action-result-banner";
import { EditorFooter } from "../editor-footer";
import { PageShell } from "../page-shell";

const meta: Meta<typeof PageShell> = {
  title: "components/custom/PageShell",
  component: PageShell,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Route body shell — owns banner / scrollable body / sticky footer composition. The shell-level `<Topbar>` owns the route header (per) so PageShell stays chromeless. Two density modes: `comfortable` (default) and `compact`.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="h-[480px] w-full bg-background border border-line flex flex-col">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Comfortable density with banner + body + sticky save bar footer.
 */
export const Comfortable: Story = {
  args: {},
  render: () => (
    <PageShell
      banner={
        <div className="px-9 pt-5">
          <ActionResultBanner
            tone="warning"
            title="Provider quota nearing limit"
            description="The agent will throttle requests once the daily budget is reached."
          />
        </div>
      }
      footer={
        <EditorFooter
          meta="Edited 2 fields. Press ⌘S to save."
          secondary={
            <Button variant="ghost" size="sm">
              Cancel
            </Button>
          }
          primary={<Button size="sm">Save changes</Button>}
        />
      }
    >
      <h1 className="text-[22px] font-medium tracking-[-0.026em] text-fg-strong">Settings</h1>
      <p className="text-[13px] text-muted max-w-prose">
        Body content stretches to fill available height; the sticky footer stays anchored to the
        bottom even on long forms.
      </p>
      <div className="h-[400px] rounded-lg border border-line bg-canvas-soft p-4 text-[13px] text-muted">
        Tall content placeholder
      </div>
    </PageShell>
  ),
};

/**
 * Compact density tightens the body padding for dense settings panels.
 */
export const Compact: Story = {
  args: {},
  render: () => (
    <PageShell density="compact">
      <h1 className="text-[18px] font-medium tracking-empty-h1 text-fg-strong">Vault</h1>
      <p className="text-[13px] text-muted">Compact density removes a step of vertical padding.</p>
    </PageShell>
  ),
};
