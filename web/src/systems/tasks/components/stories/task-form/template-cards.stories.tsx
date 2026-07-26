import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { DEFAULT_TASK_TEMPLATE_ID } from "@/systems/tasks";
import type { TaskTemplateId } from "@/systems/tasks";

import { TemplateCards, type TaskFormMode } from "../../task-form/template-cards";

const meta: Meta<typeof TemplateCards> = {
  title: "systems/tasks/components/task-form/TemplateCards",
  component: TemplateCards,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function TemplateCardsHarness({
  mode,
  initialTemplateId = DEFAULT_TASK_TEMPLATE_ID,
}: {
  mode: TaskFormMode;
  initialTemplateId?: TaskTemplateId;
}) {
  const [templateId, setTemplateId] = useState<TaskTemplateId>(initialTemplateId);

  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="w-full max-w-(--width-modal-md) rounded-xl border border-line bg-canvas-soft p-5">
        <TemplateCards mode={mode} onSelect={setTemplateId} templateId={templateId} />
      </div>
    </CenteredSurface>
  );
}

export const SimpleMode: Story = {
  args: {},
  render: () => <TemplateCardsHarness mode="simple" />,
};

export const AdvancedMode: Story = {
  args: {},
  render: () => <TemplateCardsHarness initialTemplateId="human_in_loop" mode="advanced" />,
};
