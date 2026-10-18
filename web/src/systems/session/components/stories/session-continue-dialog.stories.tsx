import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import {
  deriveAgentsFixture,
  deriveRoutedAgentsFixture,
  deriveSourceSessionFixture,
  deriveTruncatedPreviewFixture,
} from "../../mocks/derive-fixtures";
import {
  sessionDeriveHandlers,
  type SessionDeriveHandlerOptions,
} from "../../mocks/derive-handlers";
import type { AgentPayload } from "@/systems/agent";
import { SessionContinueDialog } from "../session-continue-dialog";

function deriveParameters(
  options: SessionDeriveHandlerOptions = {},
  agents: AgentPayload[] = deriveAgentsFixture
) {
  return storybookMswParameters({
    agent: [compozyApiMock.get("/api/agents", () => HttpResponse.json({ agents }))],
    session: sessionDeriveHandlers(options),
  });
}

const baseArgs = {
  source: deriveSourceSessionFixture,
  workspaceId: deriveSourceSessionFixture.workspace_id ?? "",
  open: true,
  onOpenChange: fn(),
  placement: { openInNewWindow: fn(), openInThisWindow: fn() },
};

const meta: Meta<typeof SessionContinueDialog> = {
  title: "systems/session/components/SessionContinueDialog",
  component: SessionContinueDialog,
  args: baseArgs,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Continue with another agent (dialogs board §01). The context line is the daemon's preview, measured on open; Continue stays disabled until it is measured and after a failed measurement. The source session is never changed.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-06: another agent preselected, preview ready. */
export const Default: Story = { parameters: deriveParameters() };

/** VC-07: the chosen agent declares routes; colliding routes carry an account suffix. */
export const RouteRow: Story = {
  args: { source: { ...deriveSourceSessionFixture, agent_name: "codex" } },
  parameters: deriveParameters({}, deriveRoutedAgentsFixture),
};

/** VC-08: the budget dropped earlier messages; the line says how many, Continue stays enabled. */
export const TruncatedPreview: Story = {
  parameters: deriveParameters({ preview: deriveTruncatedPreviewFixture }),
};

/** Measuring: the preview is in flight and Continue waits for it. */
export const Measuring: Story = { parameters: deriveParameters({ preview: "pending" }) };

/** VC-09: the preview failed; never "0 messages", and Continue stays disabled. */
export const PreviewError: Story = { parameters: deriveParameters({ preview: "error" }) };

/** A source whose running turn will not travel. */
export const TurnInProgress: Story = {
  parameters: deriveParameters({
    preview: {
      ...deriveTruncatedPreviewFixture,
      source_message_count: 30,
      truncated: false,
      omitted_count: 0,
      source_turn_in_progress: true,
    },
  }),
};

/**
 * VC-10: submitted and waiting on the daemon — fields disabled, close hidden,
 * `Starting the new session…` announced.
 */
export const Pending: Story = {
  parameters: deriveParameters({ result: "pending" }),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const submit = await body.findByTestId("session-continue-submit");
    await waitFor(() => expect(submit).toBeEnabled());
    await userEvent.click(submit);
    await expect(await body.findByTestId("session-continue-pending-status")).toBeVisible();
  },
};

/** Hosts without a session window of their own (sessions modal, agent detail) open a new window only. */
export const WithoutThisWindow: Story = {
  args: { placement: { openInNewWindow: fn() } },
  parameters: deriveParameters(),
};
