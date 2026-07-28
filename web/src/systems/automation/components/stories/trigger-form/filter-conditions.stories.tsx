import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { filterKeyOptions, getEventDef } from "@/systems/automation/lib/trigger-catalog";
import type { AutomationTriggerFilter } from "@/systems/automation";

import { FilterConditions } from "../../trigger-form/filter-conditions";

const meta: Meta<typeof FilterConditions> = {
  title: "systems/automation/components/trigger-form/FilterConditions",
  component: FilterConditions,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const sessionKeys = filterKeyOptions(getEventDef("session.stopped")!);
const webhookKeys = filterKeyOptions(getEventDef("webhook")!);

function FilterConditionsHarness({
  initialFilter,
  eventKind,
  keyOptions,
  openPayload,
}: {
  initialFilter: AutomationTriggerFilter;
  eventKind: string;
  keyOptions: string[];
  openPayload: boolean;
}) {
  const [filter, setFilter] = useState(initialFilter);

  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="w-full max-w-xl">
        <FilterConditions
          eventKind={eventKind}
          filter={filter}
          keyOptions={keyOptions}
          onChange={setFilter}
          openPayload={openPayload}
        />
      </div>
    </CenteredSurface>
  );
}

export const Empty: Story = {
  args: {},
  render: () => (
    <FilterConditionsHarness
      eventKind="session.stopped"
      initialFilter={{}}
      keyOptions={sessionKeys}
      openPayload={false}
    />
  ),
};

export const FixedEventSelect: Story = {
  args: {},
  render: () => (
    <FilterConditionsHarness
      eventKind="session.stopped"
      initialFilter={{ "data.stop_reason": "error", source: "observer" }}
      keyOptions={sessionKeys}
      openPayload={false}
    />
  ),
};

export const OpenPayloadCombobox: Story = {
  args: {},
  render: () => (
    <FilterConditionsHarness
      eventKind="webhook"
      initialFilter={{ "data.action": "deploy_started" }}
      keyOptions={webhookKeys}
      openPayload
    />
  ),
};
