import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { LoopEditorCriteria } from "../editor/loop-editor-criteria";

function Harness({
  initial = [] as unknown[],
  allowedTypes,
}: {
  initial?: unknown[];
  allowedTypes?: readonly ("command" | "agent-judge" | "human" | "extension")[];
}) {
  const [value, setValue] = useState<unknown>(initial);
  return (
    <div>
      <LoopEditorCriteria value={value} onChange={setValue} allowedTypes={allowedTypes} />
      <output data-testid="json">{JSON.stringify(value)}</output>
    </div>
  );
}

describe("LoopEditorCriteria", () => {
  it("Should add a default command criterion", () => {
    render(<Harness />);
    fireEvent.click(screen.getByTestId("loop-editor-criteria-add"));
    const json = JSON.parse(screen.getByTestId("json").textContent || "[]");
    expect(json).toHaveLength(1);
    expect(json[0]).toMatchObject({ type: "command", expect: "exit_zero" });
  });

  it("Should switch a criterion type and edit its per-type fields", () => {
    render(
      <Harness
        initial={[{ id: "review", type: "command", check: "make test", expect: "exit_zero" }]}
      />
    );
    fireEvent.change(screen.getByLabelText("Criterion type"), { target: { value: "agent-judge" } });
    fireEvent.change(screen.getByLabelText("Judge agent"), { target: { value: "reviewer" } });
    fireEvent.change(screen.getByLabelText("Rubric"), { target: { value: "Cite evidence." } });
    const json = JSON.parse(screen.getByTestId("json").textContent || "[]");
    expect(json[0]).toMatchObject({
      type: "agent-judge",
      agent: "reviewer",
      rubric: "Cite evidence.",
    });
  });

  it("Should remove a criterion", () => {
    render(<Harness initial={[{ id: "a", type: "human", prompt: "ok?" }]} />);
    fireEvent.click(screen.getByRole("button", { name: /Remove criterion/i }));
    expect(JSON.parse(screen.getByTestId("json").textContent || "x")).toEqual([]);
  });

  it("Should preserve an id-less criterion row while its fields change", () => {
    render(<Harness initial={[{ type: "command", check: "make test", expect: "exit_zero" }]} />);
    const checkInput = screen.getByLabelText("Command check");
    checkInput.focus();

    fireEvent.change(checkInput, { target: { value: "make verify" } });

    expect(screen.getByTestId("json")).toHaveTextContent('"check":"make verify"');
    expect(screen.getByLabelText("Command check")).toBe(checkInput);
    expect(checkInput).toHaveFocus();
  });

  it("Should allocate the next generated id after the highest existing criterion suffix", () => {
    render(
      <Harness
        initial={[
          { id: "criterion_1", type: "command" },
          { id: "criterion_2", type: "command" },
          { id: "criterion_3", type: "command" },
        ]}
      />
    );
    fireEvent.click(screen.getByRole("button", { name: "Remove criterion criterion_1" }));
    fireEvent.click(screen.getByTestId("loop-editor-criteria-add"));
    const json = JSON.parse(screen.getByTestId("json").textContent || "[]");
    expect(json.map((criterion: { id: string }) => criterion.id)).toEqual([
      "criterion_2",
      "criterion_3",
      "criterion_4",
    ]);
  });

  it("Should restrict Goal judge criteria to command, agent-judge, and extension", () => {
    render(
      <LoopEditorCriteria
        value={[{ id: "judge", type: "command", check: "make verify" }]}
        onChange={() => undefined}
        allowedTypes={["command", "agent-judge", "extension"]}
      />
    );
    const options = screen
      .getAllByRole("option")
      .map(option => (option as HTMLOptionElement).value);
    expect(options).toEqual(["command", "agent-judge", "extension", "exit_zero", "stdout_match"]);
    expect(options).not.toContain("human");
  });

  it("Should normalize rendered and added criteria to the allowed type set", () => {
    render(
      <Harness
        initial={[{ id: "judge", type: "human", prompt: "approve?" }]}
        allowedTypes={["agent-judge", "extension"]}
      />
    );

    expect(screen.getByLabelText("Criterion type")).toHaveValue("agent-judge");
    fireEvent.click(screen.getByTestId("loop-editor-criteria-add"));

    const json = JSON.parse(screen.getByTestId("json").textContent || "[]");
    expect(json.map((criterion: { type: string }) => criterion.type)).toEqual([
      "agent-judge",
      "agent-judge",
    ]);
    expect(json[1]).toMatchObject({ agent: "", rubric: "" });
  });
});
