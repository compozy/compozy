import { agentFixtures } from "@/systems/agent/mocks";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { userEvent, within } from "storybook/test";

import {
  storyAgentNames,
  storyWorkspaceIds,
  storyWorkspaceNames,
} from "@/storybook/fintech-scenario";
import type { AutomationEditorSection } from "@/systems/automation";
import { AutomationEditorDialog } from "@/systems/automation/components/automation-editor-dialog";
import {
  automationCondition,
  createAutomationFormDraft,
  type AutomationFormDraft,
} from "@/systems/automation/lib/automation-form-draft";

const meta: Meta<typeof AutomationEditorDialog> = {
  title: "systems/automation/components/AutomationEditorDialog",
  component: AutomationEditorDialog,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const ACTIVE_WORKSPACE_ID = storyWorkspaceIds.hq;

const storyWorkspaces = [
  { id: storyWorkspaceIds.hq, name: storyWorkspaceNames.hq },
  { id: storyWorkspaceIds.risk, name: storyWorkspaceNames.risk },
  { id: storyWorkspaceIds.growth, name: storyWorkspaceNames.growth },
];

function readySchedule(): AutomationFormDraft {
  return {
    ...createAutomationFormDraft(ACTIVE_WORKSPACE_ID),
    name: "morning-digest",
    agent_name: storyAgentNames.product,
    prompt: "Summarize yesterday's sessions for the team channel.",
    schedule: { mode: "cron", expr: "0 9 * * 1-5" },
  };
}

function readyEvent(): AutomationFormDraft {
  return {
    ...createAutomationFormDraft(ACTIVE_WORKSPACE_ID, { start: "event" }),
    name: "summarize-failures",
    agent_name: storyAgentNames.support,
    conditions: [automationCondition("data.stop_reason", "error")],
    prompt:
      'Session {{ .Data.session_id }} stopped with reason "{{ .Data.stop_reason }}". ' +
      "Summarize what went wrong and one suggested next step.",
  };
}

function readyLink(): AutomationFormDraft {
  return {
    ...createAutomationFormDraft(ACTIVE_WORKSPACE_ID, { start: "webhook" }),
    name: "deploy-webhook",
    agent_name: storyAgentNames.product,
    endpoint_slug: "deploy",
    webhook_id: "wbh_abc123",
    webhook_secret_value: "whsec_demo",
    prompt: "A deploy started: {{ .Data.payload }}. Watch it and report back.",
  };
}

interface HarnessProps {
  initialDraft: () => AutomationFormDraft;
  mode?: "create" | "edit";
  lockedLoop?: string;
  section?: AutomationEditorSection;
  submitError?: string;
  submitErrorField?: "name";
}

function AutomationEditorHarness({
  initialDraft,
  mode = "create",
  lockedLoop,
  section,
  submitError,
  submitErrorField,
}: HarnessProps) {
  const [draft, setDraft] = useState<AutomationFormDraft>(initialDraft);

  return (
    <AutomationEditorDialog
      agents={agentFixtures}
      activeWorkspaceId={ACTIVE_WORKSPACE_ID}
      editor={{
        draft,
        isPending: false,
        lockedLoop,
        mode,
        onCancel: () => undefined,
        onChange: setDraft,
        onSubmit: () => undefined,
        section,
        submitError,
        submitErrorField,
      }}
      workspaces={storyWorkspaces}
    />
  );
}

/** editor VC-01: a new schedule, ready to create. */
export const NewSchedule: Story = {
  args: {},
  render: () => <AutomationEditorHarness initialDraft={readySchedule} />,
};

/** editor VC-01: an event start with a condition. */
export const NewOnEvent: Story = {
  args: {},
  render: () => <AutomationEditorHarness initialDraft={readyEvent} />,
};

/** editor VC-01: a link start, always Global. */
export const NewOnLink: Story = {
  args: {},
  render: () => <AutomationEditorHarness initialDraft={readyLink} />,
};

/** editor VC-05: a blank draft reads "Needs a fix" with the missing parts dashed. */
export const NeedsAFix: Story = {
  args: {},
  render: () => (
    <AutomationEditorHarness initialDraft={() => createAutomationFormDraft(ACTIVE_WORKSPACE_ID)} />
  ),
};

/** editor VC-02: the preview swap for a schedule (next runs + request). */
export const Preview: Story = {
  args: {},
  render: () => <AutomationEditorHarness initialDraft={readySchedule} />,
  play: async ({ canvasElement }) => {
    // The dialog portals out of the story root.
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("automation-preview-toggle"));
  },
};

/** editor VC-03: Options open with retries on. */
export const OptionsOpen: Story = {
  args: {},
  render: () => (
    <AutomationEditorHarness
      initialDraft={() => ({
        ...readySchedule(),
        retry: { strategy: "backoff", max_retries: 3, base_delay: "2s" },
      })}
      section="options"
    />
  ),
};

/** editor VC-04: edit mode locks Starts and Does and says why. */
export const EditLocked: Story = {
  args: {},
  render: () => <AutomationEditorHarness initialDraft={readySchedule} mode="edit" />,
};

/** US-027: the Loop page seed fixes Does to its Loop. */
export const FromLoopPage: Story = {
  args: {},
  render: () => (
    <AutomationEditorHarness
      initialDraft={() => ({
        ...createAutomationFormDraft(ACTIVE_WORKSPACE_ID, {
          start: "event",
          loop: "review-and-fix",
        }),
        name: "fix-on-failure",
      })}
      lockedLoop="review-and-fix"
    />
  ),
};

/** US-019 EC-1: the daemon's name conflict lands on the Name field. */
export const NameConflict: Story = {
  args: {},
  render: () => (
    <AutomationEditorHarness
      initialDraft={readySchedule}
      submitError="An automation named morning-digest already exists."
      submitErrorField="name"
    />
  ),
};
