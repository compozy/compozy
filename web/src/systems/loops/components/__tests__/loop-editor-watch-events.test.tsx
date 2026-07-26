import { fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { LOOP_WATCH_EVENT_KINDS } from "@/generated/loop-enums";

import { LoopEditorWatchEvents } from "../editor/loop-editor-watch-events";

function Harness({ initial = [] as unknown[] }: { initial?: unknown[] }) {
  const [value, setValue] = useState<unknown>(initial);
  return (
    <div>
      <LoopEditorWatchEvents value={value} onChange={setValue} />
      <output data-testid="json">{JSON.stringify(value)}</output>
    </div>
  );
}

describe("LoopEditorWatchEvents", () => {
  it("Should add a subscription defaulting to the first supported kind", () => {
    render(<Harness />);
    fireEvent.click(screen.getByTestId("loop-editor-watch-events-add"));
    const json = JSON.parse(screen.getByTestId("json").textContent || "[]");
    expect(json).toHaveLength(1);
    expect(json[0]).toEqual({ kind: LOOP_WATCH_EVENT_KINDS[0] });
  });

  it("Should offer exactly the generated supported-kind matrix in the kind select", () => {
    // The options MUST derive from the generated LOOP_WATCH_EVENT_KINDS (Go
    // SupportedWatchEvents registry) — never a hand-authored TS list. When phases B/C
    // add registry rows, the generated array grows and this assertion tracks it.
    render(<Harness initial={[{ kind: LOOP_WATCH_EVENT_KINDS[0] }]} />);
    const select = screen.getByTestId("loop-watch-event-kind-0");
    const options = within(select)
      .getAllByRole("option")
      .map(option => (option as HTMLOptionElement).value);
    expect(options).toEqual([...LOOP_WATCH_EVENT_KINDS]);
  });

  it("Should edit a subscription's kind and CEL filter", () => {
    render(<Harness initial={[{ kind: LOOP_WATCH_EVENT_KINDS[0] }]} />);
    fireEvent.change(screen.getByTestId("loop-watch-event-kind-0"), {
      target: { value: "loop.terminal" },
    });
    fireEvent.change(screen.getByLabelText("Subscription 1 filter"), {
      target: { value: "event.payload.to == 'done'" },
    });
    const json = JSON.parse(screen.getByTestId("json").textContent || "[]");
    expect(json[0]).toEqual({ kind: "loop.terminal", filter: "event.payload.to == 'done'" });
  });

  it("Should remove a subscription", () => {
    render(<Harness initial={[{ kind: "task.status_changed" }]} />);
    fireEvent.click(screen.getByRole("button", { name: /Remove subscription/i }));
    expect(JSON.parse(screen.getByTestId("json").textContent || "x")).toEqual([]);
  });

  it("Should preserve the following editor instance when removing a middle subscription", () => {
    render(
      <Harness
        initial={[
          { kind: "task.status_changed", filter: "event.task_id == 'first'" },
          { kind: "task.status_changed", filter: "event.task_id == 'second'" },
          { kind: "task.status_changed", filter: "event.task_id == 'third'" },
        ]}
      />
    );
    const thirdEditor = screen.getByTestId("loop-watch-event-filter-2");
    thirdEditor.focus();

    fireEvent.click(screen.getByRole("button", { name: "Remove subscription 2" }));

    expect(screen.getByTestId("loop-watch-event-filter-1")).toBe(thirdEditor);
    expect(document.activeElement).toBe(thirdEditor);
  });
});
