import type { Meta, StoryObj } from "@storybook/react-vite";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

import {
  canceledCallFixture,
  completedCallFixture,
  createAgentCommsHandlers,
  extractedCallFixture,
  invalidResultCallFixture,
  overBudgetCallFixture,
  repairedCallFixture,
  runningCallFixture,
  silentFinishCallFixture,
  timeoutCallFixture,
} from "../mocks";
import type { CallPayload } from "../types";

/**
 * Serve exactly one call, so each story pins one terminal shape.
 *
 * Its own mock server over its own dataset — nothing is shared between stories,
 * so loading several at once cannot cross their populations. The routes are the
 * workspace ones the product calls, including `/prompt`, which nothing mocked
 * before: `OverBudget` had a fetch button with an empty route behind it.
 */
function oneCall(call: CallPayload) {
  return createAgentCommsHandlers({ calls: [call], messages: [] });
}

function storyFor(call: CallPayload): StoryObj<typeof meta> {
  return {
    args: {},
    parameters: {
      ...appRouteParameters(`/agents/calls/${call.call_id}`),
      ...storybookMswParameters({ "agent-comms": oneCall(call) }),
    },
    render: () => <StorybookWorkspaceSetup />,
  };
}

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/agent-comms/routes/AgentCallDetail",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "One call's record at /agents/calls/$callId — one story per outcome, so every control-visibility rule is captured against a real state.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Completed with a returned answer — the whole story on one page. */
export const Completed: Story = storyFor(completedCallFixture);

/** Recovered from prose. Renders as `extracted`, never as `returned`. */
export const Extracted: Story = storyFor(extractedCallFixture);

/** Fixed on the second try, with the repair round on the timeline. */
export const Repaired: Story = storyFor(repairedCallFixture);

/** Both tries on record, each with the validator's own words. */
export const InvalidResult: Story = storyFor(invalidResultCallFixture);

/** Finished with nothing to show — stated, not decorated. */
export const CompletedWithoutResult: Story = storyFor(silentFinishCallFixture);

/** Stopped on purpose, with a late answer preserved as evidence. */
export const CanceledWithSuperseded: Story = storyFor(canceledCallFixture);

/** Ran past a deadline someone chose to set — the only case with timer chrome. */
export const Timeout: Story = storyFor(timeoutCallFixture);

/** In flight: cancel is the only control, and the idle clock is suspended. */
export const Running: Story = storyFor(runningCallFixture);

/** Bigger than its preview — bounded rows plus the full fetch. */
export const OverBudget: Story = storyFor(overBudgetCallFixture);

/** A call that is not in this profile, or no longer exists. */
export const NotFound: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/agents/calls/call_missing"),
    ...storybookMswParameters({
      "agent-comms": [
        compozyApiMock.get("/api/workspaces/{workspace_id}/calls/{call_id}", ({ response }) =>
          response(404).json({ error: "call not found", code: "call_target_not_found" })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
