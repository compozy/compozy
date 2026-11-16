import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, waitFor, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { CenteredSurface } from "@/storybook/story-layout";

import { sessionKeys } from "../../lib/query-keys";
import {
  sessionMCPFixture,
  sessionMCPReloadFailedFixture,
  sessionMCPSummaryFixture,
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
          "Issue #1758: the composer's closed MCP control reports the last reload outcome. A failed reload shows the attention dot and names it; once a later reload applies, both clear without opening the popover.",
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
