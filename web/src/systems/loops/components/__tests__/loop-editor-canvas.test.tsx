import { fireEvent, render, screen } from "@testing-library/react";
import type { MouseEventHandler, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const setCenter = vi.fn();
const getZoom = vi.fn(() => 1);
const screenToFlowPosition = vi.fn((point: { x: number; y: number }) => point);

interface CapturedReactFlowProps {
  children?: ReactNode;
  deleteKeyCode?: string | string[] | null;
  onDoubleClickCapture?: MouseEventHandler<HTMLDivElement>;
}

let capturedReactFlowProps: CapturedReactFlowProps = {};

vi.mock("@xyflow/react", async importOriginal => {
  const actual = await importOriginal<typeof import("@xyflow/react")>();
  return {
    ...actual,
    Background: () => null,
    ReactFlow: (props: CapturedReactFlowProps) => {
      capturedReactFlowProps = props;
      return (
        <div onDoubleClickCapture={props.onDoubleClickCapture}>
          <div className="react-flow__pane" data-testid="react-flow-pane" />
          {props.children}
        </div>
      );
    },
    useReactFlow: () => ({ getZoom, screenToFlowPosition, setCenter }),
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

  it("Should open quick-add from a captured double-click on the empty pane", () => {
    const onQuickAdd = vi.fn();
    render(
      <LoopEditorCanvas
        edges={[]}
        nodes={[editorNode("review")]}
        onConnect={noop}
        onEdgesChange={noop}
        onNodesChange={noop}
        onQuickAdd={onQuickAdd}
        onSelectNode={noop}
        readOnly={false}
        revealSeq={0}
        selectedNodeId={null}
      />
    );

    fireEvent.doubleClick(screen.getByTestId("react-flow-pane"), {
      clientX: 400,
      clientY: 300,
    });

    expect(onQuickAdd).toHaveBeenCalledWith({ x: 306, y: 252 });
  });

  it("Should bind both deletion keys only for editable definitions", () => {
    const props = {
      edges: [],
      nodes: [editorNode("review")],
      onConnect: noop,
      onEdgesChange: noop,
      onNodesChange: noop,
      onSelectNode: noop,
      revealSeq: 0,
      selectedNodeId: null,
    };
    const { rerender } = render(<LoopEditorCanvas {...props} readOnly={false} />);
    expect(capturedReactFlowProps.deleteKeyCode).toEqual(["Backspace", "Delete"]);

    rerender(<LoopEditorCanvas {...props} readOnly />);
    expect(capturedReactFlowProps.deleteKeyCode).toBeNull();
  });
});
