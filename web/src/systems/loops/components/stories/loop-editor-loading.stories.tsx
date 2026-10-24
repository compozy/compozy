import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import { LoopEditor } from "../editor/loop-editor";
import { handlers as loopHandlers } from "../../mocks";
import { EditorHarness } from "./loop-editor-story-harness";

const WS = "ws_default";

const meta: Meta<typeof LoopEditor> = {
  title: "systems/loops/components/LoopEditorLoading",
  component: LoopEditor,
  parameters: { layout: "fullscreen" },
  render: args => <EditorHarness {...args} />,
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The Loop has not been read: the editor's own chrome (toolbar, node palette, validation bar),
 * inert, over a canvas of placeholders. The read never answers, so the state holds still.
 */
export const Loading: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: {
    msw: {
      handlers: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/loops/{name}", async () => {
          await delay("infinite");
          throw new Error("unreachable");
        }),
        ...loopHandlers,
      ],
    },
  },
};
