import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { LOOP_PALETTE } from "../../lib/loop-palette";
import { LoopEditorPaletteMenu } from "../editor/loop-editor-palette-menu";

describe("LoopEditorPaletteMenu", () => {
  it("Should render palette items and fire onAddNode", async () => {
    const user = userEvent.setup();
    const onAddNode = vi.fn();
    render(<LoopEditorPaletteMenu onAddNode={onAddNode} />);
    const trigger = screen.getByTestId("loop-editor-add-node");
    fireEvent.pointerDown(trigger, { button: 0, pointerType: "mouse" });
    await user.click(trigger);
    const item = await screen.findByTestId("loop-palette-menu-item-run-agent");
    expect(item).toHaveTextContent("Run agent");
    await user.click(item);
    expect(onAddNode).toHaveBeenCalledTimes(1);
    expect(onAddNode.mock.calls[0]?.[0]).toBe(LOOP_PALETTE[0]?.items[0]);
  });

  it("Should disable the trigger when the editor is read-only", () => {
    render(<LoopEditorPaletteMenu disabled onAddNode={vi.fn()} />);
    expect(screen.getByTestId("loop-editor-add-node")).toBeDisabled();
  });
});
