import type { Meta, StoryObj } from "@storybook/react-vite";

import { storyWorkspaceIds, storyWorkspaceNames } from "@/storybook/fintech-scenario";
import { CenteredSurface } from "@/storybook/story-layout";
import { createAutomationTriggerDraft } from "@/systems/automation";
import type { CreateAutomationTriggerRequest } from "@/systems/automation";
import { buildTriggerPreview } from "@/systems/automation/lib/trigger-preview";

import { TriggerPreview } from "../../trigger-form/preview/trigger-preview";

const meta: Meta<typeof TriggerPreview> = {
  title: "systems/automation/components/trigger-form/TriggerPreview",
  component: TriggerPreview,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const workspaces = [{ id: storyWorkspaceIds.hq, name: storyWorkspaceNames.hq }];

function previewFor(overrides: Partial<CreateAutomationTriggerRequest>) {
  const draft: CreateAutomationTriggerRequest = {
    ...createAutomationTriggerDraft(storyWorkspaceIds.hq),
    event: "session.stopped",
    name: "summarize-failures",
    agent_name: "summarizer",
    prompt: "Session {{ .Data.session_id }} stopped: {{ .Data.stop_reason }}.",
    ...overrides,
  };
  return buildTriggerPreview(draft, { workspaces });
}

function PreviewHarness({ preview }: { preview: ReturnType<typeof buildTriggerPreview> }) {
  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="grid h-[760px] w-(--width-right-rail-default) grid-rows-[minmax(0,1fr)] overflow-hidden rounded-xl border border-line">
        <TriggerPreview preview={preview} />
      </div>
    </CenteredSurface>
  );
}

export const Matches: Story = {
  args: {},
  render: () => (
    <PreviewHarness preview={previewFor({ filter: { "data.stop_reason": "error" } })} />
  ),
};

export const WontFire: Story = {
  args: {},
  render: () => (
    <PreviewHarness preview={previewFor({ filter: { "data.stop_reason": "completed" } })} />
  ),
};

export const LoopTarget: Story = {
  args: {},
  render: () => (
    <PreviewHarness
      preview={buildTriggerPreview(
        {
          ...createAutomationTriggerDraft(storyWorkspaceIds.hq),
          event: "session.stopped",
          name: "delivery-on-stop",
          agent_name: "",
          prompt: "",
          target_kind: "loop",
          loop_target: {
            workspace_id: storyWorkspaceIds.hq,
            loop_name: "software-delivery",
            inputs: { slug: "helix-v1-launch" },
            input_mapping: { branch: "data.branch" },
          },
        },
        { mode: "edit", workspaces }
      )}
    />
  ),
};

export const Webhook: Story = {
  args: {},
  render: () => (
    <PreviewHarness
      preview={previewFor({
        event: "webhook",
        scope: "global",
        workspace_id: undefined,
        endpoint_slug: "ci-deploys",
        webhook_id: "wbh_a1b2c3",
        prompt: "A deploy webhook arrived: {{ .Data.payload }}.",
      })}
    />
  ),
};
