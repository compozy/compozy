import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { HttpResponse } from "msw";

import { CenteredSurface } from "@/storybook/story-layout";
import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import {
  bashToolMessageFixture,
  errorToolMessageFixture,
  multiHunkEditToolMessageFixture,
  readToolMessageFixture,
  runningBashToolMessageFixture,
  searchToolMessageFixture,
} from "@/systems/session/mocks";
import type { UIMessage } from "@/systems/session/types";

import { SessionRuntimeRenderProvider } from "../../lib/session-runtime-render-context";
import { SessionToolCallRow } from "../tool-call-card";

const meta: Meta<typeof SessionToolCallRow> = {
  title: "systems/session/components/SessionToolCallRow",
  component: SessionToolCallRow,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const mcpToolMessageFixture: UIMessage = {
  id: "tool_mcp_context",
  role: "tool_result",
  content: "",
  toolName: "mcp__context7__resolve-library-id",
  toolInput: {
    libraryName: "react",
  },
  toolResult: {
    content: "/websites/react_dev",
  },
  timestamp: Date.parse("2026-04-17T16:07:00Z"),
};

const globToolMessageFixture: UIMessage = {
  id: "tool_glob",
  role: "tool_result",
  content: "",
  toolName: "Glob",
  toolInput: {
    pattern: "apps/launch-site/**/*.test.tsx",
  },
  toolResult: {
    stdout: "apps/launch-site/src/components/hero-banner.test.tsx",
  },
  timestamp: Date.parse("2026-04-17T16:07:30Z"),
};

const webSearchToolMessageFixture: UIMessage = {
  id: "tool_web_search",
  role: "tool_result",
  content: "",
  toolName: "WebSearch",
  toolInput: {
    query: "BR partner-bank checkout timeout copy guidance",
  },
  toolResult: {
    content: "3 results reviewed.",
  },
  timestamp: Date.parse("2026-04-17T16:08:00Z"),
};

const aghMemoryToolMessageFixture: UIMessage = {
  id: "tool_agh_memory",
  role: "tool_result",
  content: "",
  toolName: "agh__memory_note",
  toolInput: {
    note: "Launch cutover blocked on partner-bank timeout copy sign-off.",
  },
  toolResult: {
    content: "Recorded memory note.",
  },
  timestamp: Date.parse("2026-04-17T16:08:30Z"),
};

const pendingToolMessageFixture: UIMessage = {
  id: "tool_pending",
  role: "tool_call",
  content: "",
  toolName: "Read",
  toolInput: {},
  timestamp: Date.parse("2026-04-17T16:09:00Z"),
};

const emptyToolMessageFixture: UIMessage = {
  id: "tool_empty",
  role: "tool_result",
  content: "",
  toolName: "Grep",
  toolInput: {
    pattern: "TODO\\(launch\\)",
  },
  toolResult: {},
  timestamp: Date.parse("2026-04-17T16:09:30Z"),
};

const artifactID = `art_${"d".repeat(64)}`;
const artifactURI = `agh://tool-artifacts/${artifactID}`;
const retainedResult = JSON.stringify(
  {
    content: [
      {
        type: "text",
        text: "Release verification completed across runtime, Web, and documentation surfaces.",
      },
    ],
    structured: {
      checkpoint: "iteration-043",
      status: "verified",
      evidence: ["make verify", "Storybook capture", "workspace isolation"],
    },
    truncated: false,
  },
  null,
  2
);
const retainedResultBytes = new TextEncoder().encode(retainedResult);
const retainedPageBytes = 160;
const retainedResultPreview =
  "Release verification completed across runtime, Web, and documentation surfaces…";

function base64Bytes(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return globalThis.btoa(binary);
}

const artifactHandler = aghApiMock.get(
  "/api/workspaces/{workspace_id}/tool-artifacts/{artifact_id}",
  ({ request }) => {
    const offset = Number(new URL(request.url).searchParams.get("offset") ?? "0");
    const chunk = retainedResultBytes.slice(offset, offset + retainedPageBytes);
    const nextOffset = offset + chunk.byteLength;
    return HttpResponse.json({
      artifact: {
        uri: artifactURI,
        name: "tool-result.json",
        mime_type: "application/vnd.agh.tool-result+json",
        bytes: retainedResultBytes.byteLength,
        sha256: "d".repeat(64),
      },
      offset,
      bytes: chunk.byteLength,
      total_bytes: retainedResultBytes.byteLength,
      data_base64: base64Bytes(chunk),
      next_offset: nextOffset,
      eof: nextOffset === retainedResultBytes.byteLength,
    });
  }
);

const unavailableArtifactHandler = aghApiMock.get(
  "/api/workspaces/{workspace_id}/tool-artifacts/{artifact_id}",
  () =>
    HttpResponse.json(
      { error: { code: "tool_not_found", message: "tool artifact not found" } },
      { status: 404 }
    )
);

const truncatedToolMessageFixture: UIMessage = {
  id: "tool_retained_result",
  role: "tool_result",
  content: "",
  toolName: "agh__memory_recall",
  toolInput: { query: "release verification evidence" },
  toolResult: {
    preview: retainedResultPreview,
    truncated: true,
    artifacts: [
      {
        uri: artifactURI,
        name: "tool-result.json",
        mime_type: "application/vnd.agh.tool-result+json",
        bytes: retainedResultBytes.byteLength,
        sha256: "d".repeat(64),
      },
    ],
  },
  timestamp: Date.parse("2026-07-19T04:32:00Z"),
};

function SessionToolCallRowFrame({ children }: { children: React.ReactNode }) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-3xl">{children}</div>
    </CenteredSurface>
  );
}

function ArtifactResultStory() {
  return (
    <SessionRuntimeRenderProvider sessionId="session-release" workspaceId="ws-release">
      <SessionToolCallRowFrame>
        <SessionToolCallRow message={truncatedToolMessageFixture} defaultExpanded />
      </SessionToolCallRowFrame>
    </SessionRuntimeRenderProvider>
  );
}

/**
 * Running command row with compact progress status.
 */
export const Running: Story = {
  args: {},
  render: () => (
    <SessionToolCallRowFrame>
      <SessionToolCallRow message={runningBashToolMessageFixture} />
    </SessionToolCallRowFrame>
  ),
};

/**
 * Completed command row in its collapsed default state.
 */
export const Done: Story = {
  args: {},
  render: () => (
    <SessionToolCallRowFrame>
      <SessionToolCallRow message={bashToolMessageFixture} />
    </SessionToolCallRowFrame>
  ),
};

/**
 * Expanded Bash, Edit, and dynamic MCP payloads keep their existing output renderers inline.
 */
export const ExpandedOutputs: Story = {
  args: {},
  render: () => (
    <SessionToolCallRowFrame>
      <div className="flex min-w-0 flex-col gap-2">
        <SessionToolCallRow message={bashToolMessageFixture} defaultExpanded />
        <SessionToolCallRow message={multiHunkEditToolMessageFixture} defaultExpanded />
        <SessionToolCallRow message={mcpToolMessageFixture} defaultExpanded />
      </div>
    </SessionToolCallRowFrame>
  ),
};

/**
 * Completed read row with a path preview.
 */
export const DoneRead: Story = {
  args: {},
  render: () => (
    <SessionToolCallRowFrame>
      <SessionToolCallRow message={readToolMessageFixture} />
    </SessionToolCallRowFrame>
  ),
};

/**
 * Failed command row opens the inline error body by default.
 */
export const Error: Story = {
  args: {},
  render: () => (
    <SessionToolCallRowFrame>
      <SessionToolCallRow message={errorToolMessageFixture} />
    </SessionToolCallRowFrame>
  ),
};

/**
 * The unified status matrix: one row-state language across every tool state —
 * `pending` (muted, no glyph), `running` (Spinner), `failed` (X danger, danger
 * heading for the true runtime error), `success` (Check), and `empty` (Minus,
 * neutral mid-stream). No special-case bordered/danger boxes remain.
 */
export const StatusMatrix: Story = {
  args: {},
  render: () => (
    <SessionToolCallRowFrame>
      <div className="flex min-w-0 flex-col gap-0.5">
        <SessionToolCallRow message={pendingToolMessageFixture} />
        <SessionToolCallRow message={runningBashToolMessageFixture} />
        <SessionToolCallRow message={errorToolMessageFixture} />
        <SessionToolCallRow message={readToolMessageFixture} turnSettled />
        <SessionToolCallRow message={emptyToolMessageFixture} turnSettled={false} />
      </div>
    </SessionToolCallRowFrame>
  ),
};

/**
 * Mixed-tool batch: each row shows its per-tool glyph and a visible tense-aware
 * verb + target — terminal (running), file-text, file-pen, search, folder-search,
 * globe, the AGH-native `memory` family glyph, the MCP connector, and a failed
 * command. No two known tools share the generic terminal icon.
 */
export const MixedToolBatch: Story = {
  args: {},
  render: () => (
    <SessionToolCallRowFrame>
      <div className="flex min-w-0 flex-col gap-0.5">
        <SessionToolCallRow message={runningBashToolMessageFixture} />
        <SessionToolCallRow message={readToolMessageFixture} />
        <SessionToolCallRow message={multiHunkEditToolMessageFixture} />
        <SessionToolCallRow message={searchToolMessageFixture} />
        <SessionToolCallRow message={globToolMessageFixture} />
        <SessionToolCallRow message={webSearchToolMessageFixture} />
        <SessionToolCallRow message={aghMemoryToolMessageFixture} />
        <SessionToolCallRow message={mcpToolMessageFixture} />
        <SessionToolCallRow message={errorToolMessageFixture} />
      </div>
    </SessionToolCallRowFrame>
  ),
};

/** A bounded preview keeps the operator oriented before any retained bytes are requested. */
export const TruncatedResultPreview: Story = {
  args: {},
  parameters: {
    layout: "fullscreen",
  },
  render: () => <ArtifactResultStory />,
};

/** The retained JSON opens inline and keeps the next bounded page behind an explicit action. */
export const TruncatedResultOpened: Story = {
  args: {},
  parameters: {
    ...storybookMswParameters({ session: [artifactHandler] }),
    layout: "fullscreen",
  },
  render: () => <ArtifactResultStory />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Open full result" }));
    const loadMore = await canvas.findByRole("button", { name: "Load more" });
    await expect(loadMore).toBeVisible();
    loadMore.scrollIntoView({ block: "nearest" });
  },
};

/** The final page proves the viewer assembles the complete retained JSON before decoding it. */
export const TruncatedResultLoadedTail: Story = {
  args: {},
  parameters: {
    ...storybookMswParameters({ session: [artifactHandler] }),
    layout: "fullscreen",
  },
  render: () => <ArtifactResultStory />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Open full result" }));
    for (let page = 0; page < 2; page += 1) {
      const loadMore = await canvas.findByRole("button", { name: "Load more" });
      await waitFor(() => expect(loadMore).toBeEnabled());
      await userEvent.click(loadMore);
    }
    await waitFor(() =>
      expect(canvas.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument()
    );
    await expect(canvas.getByTestId("full-tool-result")).toHaveTextContent("workspace isolation");
    canvas
      .getByText(`${retainedResultBytes.byteLength} of ${retainedResultBytes.byteLength} bytes`)
      .scrollIntoView({ block: "nearest" });
  },
};

/** Expired and missing references share one terminal state while the bounded preview remains. */
export const TruncatedResultUnavailable: Story = {
  args: {},
  parameters: {
    ...storybookMswParameters({ session: [unavailableArtifactHandler] }),
    layout: "fullscreen",
  },
  render: () => <ArtifactResultStory />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Open full result" }));
    await expect(await canvas.findByText("Full result unavailable")).toBeVisible();
    const unavailableMessage = canvas.getByText("This retained result is no longer available");
    await expect(unavailableMessage).toBeVisible();
    await expect(canvas.getByText(retainedResultPreview)).toBeVisible();
    await expect(
      canvas.queryByRole("button", { name: "Retry full result" })
    ).not.toBeInTheDocument();
    unavailableMessage.scrollIntoView({ block: "nearest" });
  },
};
