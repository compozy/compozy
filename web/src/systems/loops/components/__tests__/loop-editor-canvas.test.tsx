import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const setCenter = vi.fn();
const getZoom = vi.fn(() => 1);

vi.mock("@xyflow/react", async importOriginal => {
  const actual = await importOriginal<typeof import("@xyflow/react")>();
  return {
    ...actual,
    Background: () => null,
    ReactFlow: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
    useReactFlow: () => ({ getZoom, setCenter }),
  };
});

import type { EditorNode } from "../../lib/codec";
import { LoopEditorCanvas } from "../editor/loop-editor-canvas";

function editorNode(id: string, label = id): EditorNode {
  return {
    id,
    type: "loopNode",
    position: { x: 40, y: 80 },
    data: {
      raw: { id, class: "action", kind: "run-agent", label },
      nodeClass: "action",
      kind: "run-agent",
      hasError: false,
    },
  };
}

const noop = () => undefined;

describe("LoopEditorCanvas reveal", () => {
  it("Should not re-center when node data changes after a reveal", () => {
    const first = [editorNode("review")];
    const { rerender } = render(
      <LoopEditorCanvas
        edges={[]}
        nodes={first}
        onConnect={noop}
        onEdgesChange={noop}
        onNodesChange={noop}
        onSelectNode={noop}
        readOnly={false}
        revealSeq={1}
        selectedNodeId="review"
      />
    );
    expect(setCenter).toHaveBeenCalledTimes(1);

    rerender(
      <LoopEditorCanvas
        edges={[]}
        nodes={[editorNode("review", "review (renamed)")]}
        onConnect={noop}
        onEdgesChange={noop}
        onNodesChange={noop}
        onSelectNode={noop}
        readOnly={false}
        revealSeq={1}
        selectedNodeId="review"
      />
    );
    expect(setCenter).toHaveBeenCalledTimes(1);
  });
});
