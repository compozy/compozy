import { agentFixtures } from "@/systems/agent/mocks";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  storyAgentNames,
  storyWorkspaceIds,
  storyWorkspaceNames,
} from "@/storybook/fintech-scenario";
import { createAutomationJobDraft, createAutomationTriggerDraft } from "@/systems/automation";
import type {
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
} from "@/systems/automation";
import { AutomationEditorDialog } from "@/systems/automation/components/automation-editor-dialog";

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

const EDITED_JOB_PROMPT =
  "Summarize launch blockers, approvals, and the next cutover milestone for the " +
  `${storyWorkspaceNames.hq} launch room.`;

const EDITED_TRIGGER_PROMPT =
  'Session {{ .Data.session_id }} stopped with reason "{{ .Data.stop_reason }}". ' +
  "Summarize what went wrong and one suggested next step.";

function jobDraft(mode: "create" | "edit"): CreateAutomationJobRequest {
  const base = createAutomationJobDraft(ACTIVE_WORKSPACE_ID);
  if (mode === "create") {
    return base;
  }
  return {
    ...base,
    name: "launch-command-digest",
    agent_name: storyAgentNames.product,
    prompt: EDITED_JOB_PROMPT,
  };
}

function triggerDraft(mode: "create" | "edit"): CreateAutomationTriggerRequest {
  const base = createAutomationTriggerDraft(ACTIVE_WORKSPACE_ID);
  if (mode === "create") {
    return base;
  }
  return {
    ...base,
    name: "summarize-failures",
    agent_name: storyAgentNames.support,
    event: "session.stopped",
    scope: "workspace",
    filter: { "data.stop_reason": "error" },
    prompt: EDITED_TRIGGER_PROMPT,
  };
}

function AutomationEditorJobHarness({ mode }: { mode: "create" | "edit" }) {
  const [draft, setDraft] = useState<CreateAutomationJobRequest>(() => jobDraft(mode));

  return (
    <AutomationEditorDialog
      agents={agentFixtures}
      activeWorkspaceId={ACTIVE_WORKSPACE_ID}
      editor={{
        draft,
        isPending: false,
        kind: "jobs",
        mode,
        onCancel: () => undefined,
        onChange: setDraft,
        onSubmit: () => undefined,
      }}
      workspaces={storyWorkspaces}
    />
  );
}

function AutomationEditorTriggerHarness({ mode }: { mode: "create" | "edit" }) {
  const [draft, setDraft] = useState<CreateAutomationTriggerRequest>(() => triggerDraft(mode));

  return (
    <AutomationEditorDialog
      agents={agentFixtures}
      activeWorkspaceId={ACTIVE_WORKSPACE_ID}
      editor={{
        draft,
        isPending: false,
        kind: "triggers",
        mode,
        onCancel: () => undefined,
        onChange: setDraft,
        onSubmit: () => undefined,
      }}
      workspaces={storyWorkspaces}
    />
  );
}

export const CreateJob: Story = {
  args: {},
  render: () => <AutomationEditorJobHarness mode="create" />,
};

export const EditJob: Story = {
  args: {},
  render: () => <AutomationEditorJobHarness mode="edit" />,
};

export const CreateTrigger: Story = {
  args: {},
  render: () => <AutomationEditorTriggerHarness mode="create" />,
};

export const EditTrigger: Story = {
  args: {},
  render: () => <AutomationEditorTriggerHarness mode="edit" />,
};
