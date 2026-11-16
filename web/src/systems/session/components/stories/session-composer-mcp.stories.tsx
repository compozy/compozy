import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { CenteredSurface } from "@/storybook/story-layout";

import { sessionKeys } from "../../lib/query-keys";
import {
  sessionMCPFailedFixture,
  sessionMCPFixture,
  sessionMCPReloadFailedFixture,
  sessionMCPSummaryFixture,
  sessionMCPWaitingFixture,
} from "../../mocks/session-mcp-fixtures";
import { primarySessionFixture } from "../../mocks/fixtures";
import type { SessionPayload } from "../../types";
import { SessionComposerMCP } from "../session-composer-mcp";

const WORKSPACE_ID = primarySessionFixture.workspace_id ?? "";

const RECOVERED = sessionMCPFixture({
  lastReload: { at: "2026-11-14T16:35:00Z", outcome: "applied", reason: "fingerprint" },
});

function session(summary: SessionPayload["mcp_summary"]): SessionPayload {
  return { ...primarySessionFixture, mcp_summary: summary };
}

function sessionMCPHandler(body: typeof sessionMCPReloadFailedFixture) {
  return compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/mcp", () =>
    HttpResponse.json(body)
  );
}

/** A query client holding the session MCP read the way an earlier open of the popover left it. */
function cachedReadClient(data: typeof RECOVERED) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(sessionKeys.mcp(WORKSPACE_ID, primarySessionFixture.id), data);
  return client;
}

const FAILED_READ_CLIENT = cachedReadClient(sessionMCPReloadFailedFixture);

const meta: Meta<typeof SessionComposerMCP> = {
  title: "systems/session/components/SessionComposerMCP",
  component: SessionComposerMCP,
  parameters: {
    docs: {
      description: {
        component:
          "The composer's MCP control. The closed trigger reports attention and the last reload outcome (issue #1758). Row actions open a panel beside the row, inside the popover: Review shows what a project server runs before Allow, and Not working says why with Check again.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex items-center gap-2 rounded-lg border border-line bg-surface-1 p-2">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** A reload failed and the popover was never opened: the closed trigger still says so. */
export const ClosedReloadFailed: Story = {
  parameters: { msw: { handlers: [sessionMCPHandler(sessionMCPReloadFailedFixture)] } },
  args: { session: session(sessionMCPSummaryFixture(sessionMCPReloadFailedFixture)) },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() =>
      expect(
        canvas.getByRole("button", { name: /couldn't start with the new servers/ })
      ).toBeInTheDocument()
    );
    await expect(canvas.getByTestId("composer-mcp-attention")).toBeInTheDocument();
  },
};

/** An earlier read cached the failure; a later reload applied, so the closed trigger clears. */
export const ClosedReloadRecovered: Story = {
  args: { session: session(sessionMCPSummaryFixture(RECOVERED)) },
  decorators: [
    Story => (
      <QueryClientProvider client={FAILED_READ_CLIENT}>
        <Story />
      </QueryClientProvider>
    ),
  ],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("button", { name: "MCP servers: 3 ready" })).toBeInTheDocument();
    await expect(canvas.queryByTestId("composer-mcp-attention")).not.toBeInTheDocument();
  },
};

const STILL_FAILING = sessionMCPFixture({
  compozy: sessionMCPFailedFixture.compozy_servers.map(row =>
    row.name === "pagerduty" ? { ...row, reason: "Connection refused on start" } : row
  ),
});

/** A server is not working: its panel says why, and Check again answers in place. */
export const NotWorkingCheckAgain: Story = {
  tags: ["play-fn"],
  parameters: {
    msw: {
      handlers: [
        sessionMCPHandler(sessionMCPFailedFixture),
        compozyApiMock.post(
          "/api/workspaces/{workspace_id}/sessions/{session_id}/mcp/servers/{name}/check",
          () => HttpResponse.json(STILL_FAILING)
        ),
      ],
    },
  },
  args: { session: session(sessionMCPSummaryFixture(sessionMCPFailedFixture)) },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("composer-mcp-button"));
    await userEvent.click(await screen.findByRole("button", { name: "pagerduty is not working" }));
    const panel = await screen.findByRole("dialog", { name: "pagerduty isn't working" });
    await userEvent.click(within(panel).getByTestId("mcp-issue-panel-check"));
    await expect(await within(panel).findByTestId("mcp-issue-panel-still")).toHaveTextContent(
      "Still not working after the check."
    );
  },
};

/** A project file added a server: Review shows what it runs and where it is defined. */
export const ReviewProjectServer: Story = {
  tags: ["play-fn"],
  parameters: { msw: { handlers: [sessionMCPHandler(sessionMCPWaitingFixture)] } },
  args: { session: session(sessionMCPSummaryFixture(sessionMCPWaitingFixture)) },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("composer-mcp-button"));
    await userEvent.click(await screen.findByRole("button", { name: "Review sentry" }));
    const panel = await screen.findByRole("dialog", { name: "Allow sentry?" });
    await expect(panel).toHaveTextContent("Added by .mcp.json in this project");
    await expect(within(panel).getByTestId("mcp-allow-panel-launch")).toHaveTextContent(
      "npx -y @sentry/mcp-server"
    );
    await expect(within(panel).getByTestId("mcp-allow-panel-source")).toHaveTextContent(
      "/Users/dev/project/.mcp.json"
    );
  },
};
