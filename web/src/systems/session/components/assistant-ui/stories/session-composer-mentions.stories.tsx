import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { SessionThread } from "@/components/assistant-ui/session-thread";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { sessionMentionRecentsKey } from "@/systems/session";
import { primarySessionFixture, searchSessionFileEntriesFixture } from "@/systems/session/mocks";

import { baseArgs } from "./session-thread-story-runtime";
import { transcriptPayload } from "./session-thread-story-transcripts";

const storyWorkspaceId = primarySessionFixture.workspace_id ?? "ws_alpha";
const FILES_ROUTE = "/api/workspaces/{workspace_id}/sessions/{session_id}/files" as const;

const emptyTranscript = compozyApiMock.get(
  "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript",
  () => HttpResponse.json(transcriptPayload([]))
);

function mentionParameters(...handlers: ReturnType<typeof compozyApiMock.get>[]) {
  return storybookMswParameters({ session: [emptyTranscript, ...handlers] });
}

/** Types into the real composer until the picker answers; the cursor plugin may register late. */
async function typeMention(canvasElement: HTMLElement, text: string) {
  const canvas = within(canvasElement);
  const input = await canvas.findByLabelText("Session prompt");
  await waitFor(async () => {
    await userEvent.clear(input);
    await userEvent.type(input, text);
    await expect(canvas.getByTestId("composer-mention-menu")).toBeVisible();
  });
  return canvas;
}

/**
 * The composer `@` picker (S1, VC-01..VC-10): a second trigger beside `/`
 * listing the session's files and folders, then other sessions of the
 * workspace. Rows, headers and messages are the `/` menu's; each group owns
 * its own empty, error and loading row.
 */
const meta = {
  title: "systems/session/components/assistant-ui/SessionComposerMentions",
  component: SessionThread,
  args: baseArgs,
  parameters: { layout: "fullscreen", ...mentionParameters() },
  decorators: [
    Story => (
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={storyWorkspaceId}
      >
        <div className="flex h-[640px] w-full flex-col border border-line bg-background">
          <Story />
        </div>
      </SessionChatRuntimeProvider>
    ),
  ],
} satisfies Meta<typeof SessionThread>;

export default meta;

type Story = StoryObj<typeof meta>;

/** VC-01 — a query lists matching files (name matches emphasized) and sessions. */
export const Results: Story = {
  play: async ({ canvasElement }) => {
    const canvas = await typeMention(canvasElement, "Review @client");
    await canvas.findByRole("option", { name: /client_prompt\.go/ });
  },
};

/** VC-02 — a bare `@` leads with this workspace's recent mentions, then shallow paths. */
export const Recents: Story = {
  beforeEach: () => {
    const key = sessionMentionRecentsKey(storyWorkspaceId);
    window.localStorage.setItem(
      key,
      JSON.stringify([
        { path: "internal/acp/client_prompt.go", kind: "file" },
        { path: "web/src/", kind: "folder" },
      ])
    );
    return () => window.localStorage.removeItem(key);
  },
  play: async ({ canvasElement }) => {
    const canvas = await typeMention(canvasElement, "Look at @");
    await canvas.findByRole("option", { name: /client_prompt\.go/ });
  },
};

/** VC-04 — the first answer is still loading: the group shows its loading row. */
export const Loading: Story = {
  parameters: mentionParameters(
    compozyApiMock.get(FILES_ROUTE, async () => {
      await delay("infinite");
      return HttpResponse.json({ entries: [], truncated: false });
    })
  ),
  play: async ({ canvasElement }) => {
    const canvas = await typeMention(canvasElement, "@");
    await canvas.findByText("Loading files…");
  },
};

/** VC-05 — nothing matches in either group; Enter would send the prompt as typed. */
export const Empty: Story = {
  play: async ({ canvasElement }) => {
    const canvas = await typeMention(canvasElement, "Check @zzq");
    await canvas.findByText("No matching files");
    await canvas.findByText("No matching sessions");
  },
};

/** VC-06/VC-07 — each failing source reports in its own group. */
export const Errors: Story = {
  parameters: mentionParameters(
    compozyApiMock.get(FILES_ROUTE, () =>
      HttpResponse.json({ error: "index failed" }, { status: 500 })
    ),
    compozyApiMock.get("/api/sessions", () =>
      HttpResponse.json({ error: "catalog failed" }, { status: 500 })
    )
  ),
  play: async ({ canvasElement }) => {
    const canvas = await typeMention(canvasElement, "@login");
    await canvas.findByText("Couldn't search files");
    await canvas.findByText("Couldn't load sessions");
  },
};

/** VC-08 — a working directory past the index cap shows the truncation footer. */
export const Truncated: Story = {
  parameters: mentionParameters(
    compozyApiMock.get(FILES_ROUTE, ({ request }) => {
      const query = new URL(request.url).searchParams.get("q") ?? "";
      return HttpResponse.json({
        entries: searchSessionFileEntriesFixture(query, 50),
        truncated: true,
      });
    })
  ),
  play: async ({ canvasElement }) => {
    const canvas = await typeMention(canvasElement, "@client");
    await canvas.findByText("Showing results from the first 25,000 files");
  },
};

/** VC-11..VC-13 — picking a row inserts a neutral-ink chip; the text keeps the token. */
export const PickedChip: Story = {
  play: async ({ canvasElement }) => {
    const canvas = await typeMention(canvasElement, "Review @client_p");
    await userEvent.click(await canvas.findByRole("option", { name: /client_prompt\.go/ }));
    await expect(await canvas.findByTestId("session-mention-chip")).toHaveTextContent(
      "client_prompt.go"
    );
  },
};
