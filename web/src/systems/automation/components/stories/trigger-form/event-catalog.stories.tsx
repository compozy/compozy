import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { parseEventSelection } from "@/systems/automation/lib/trigger-event-id";
import type { SubConfigValues } from "@/systems/automation/lib/trigger-sub-config";

import { EventCatalog } from "../../trigger-form/event-catalog";

const meta: Meta<typeof EventCatalog> = {
  title: "systems/automation/components/trigger-form/EventCatalog",
  component: EventCatalog,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function EventCatalogHarness({ initialEvent }: { initialEvent: string }) {
  const [event, setEvent] = useState(initialEvent);
  const [values, setValues] = useState<SubConfigValues>({
    hookName: "transform",
    extExt: "release",
    extEvent: "deploy-started",
    endpointSlug: "ci-deploys",
    webhookId: "wbh_a1b2c3",
    webhookSecret: "whsec_live_8f3a9c2e",
  });

  const selection = parseEventSelection(event);
  const merged = {
    ...selection,
    hookName: values.hookName,
    extExt: values.extExt,
    extEvent: values.extEvent,
  };

  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="w-full max-w-xl">
        <EventCatalog
          onSelectEvent={setEvent}
          onSubConfigChange={patch => setValues(current => ({ ...current, ...patch }))}
          selection={merged}
          subConfigValues={values}
        />
      </div>
    </CenteredSurface>
  );
}

export const SessionSelected: Story = {
  args: {},
  render: () => <EventCatalogHarness initialEvent="session.stopped" />,
};

export const HookSelected: Story = {
  args: {},
  render: () => <EventCatalogHarness initialEvent="hook.transform.completed" />,
};

export const WebhookSelected: Story = {
  args: {},
  render: () => <EventCatalogHarness initialEvent="webhook" />,
};
