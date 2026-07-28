import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { LoopReferenceInput } from "../editor/loop-reference-input";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";

const suggestions: LoopReferenceSuggestion[] = [
  { path: "inputs.slug", kind: "input", detail: "declared loop input" },
  { path: "nodes.load_tasks.output", kind: "node-output", detail: "file-import output" },
];

function Harness({ initial = "", cel = false }: { initial?: string; cel?: boolean }) {
  const [value, setValue] = useState(initial);
  return (
    <div>
      <LoopReferenceInput
        value={value}
        onChange={setValue}
        suggestions={suggestions}
        cel={cel}
        testId="ref"
      />
      <output data-testid="value">{value}</output>
    </div>
  );
}

describe("LoopReferenceInput", () => {
  it("Should surface namespace suggestions after `{{ .` and insert the selected reference", () => {
    render(<Harness />);
    const input = screen.getByTestId("ref") as HTMLInputElement;
    // Type a `{{ .` trigger with a partial path and place the caret at the end.
    fireEvent.change(input, { target: { value: "post {{ .inp" } });
    input.setSelectionRange(input.value.length, input.value.length);
    fireEvent.keyUp(input);
    const list = screen.getByTestId("loop-reference-suggestions");
    expect(input).toHaveAttribute("role", "combobox");
    expect(input).toHaveAttribute("aria-expanded", "true");
    expect(input).toHaveAttribute("aria-controls", list.id);
    expect(list).toHaveAttribute("role", "listbox");
    expect(list).toHaveTextContent("inputs.slug");
    // Selecting a suggestion inserts the closed reference.
    fireEvent.mouseDown(screen.getByText("inputs.slug"));
    expect(screen.getByTestId("value")).toHaveTextContent("post {{ .inputs.slug }}");
  });

  it("Should not show the picker for plain text with no open reference", () => {
    render(<Harness initial="just a prompt" />);
    const input = screen.getByTestId("ref") as HTMLInputElement;
    input.setSelectionRange(4, 4);
    fireEvent.click(input);
    expect(screen.queryByTestId("loop-reference-suggestions")).not.toBeInTheDocument();
  });

  it("Should autocomplete a CEL identifier (no braces) and insert the bare path", () => {
    render(<Harness cel />);
    const input = screen.getByTestId("ref") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "nod" } });
    input.setSelectionRange(3, 3);
    fireEvent.keyUp(input);
    expect(screen.getByTestId("loop-reference-suggestions")).toHaveTextContent(
      "nodes.load_tasks.output"
    );
    fireEvent.mouseDown(screen.getByText("nodes.load_tasks.output"));
    // CEL inserts the raw path, not a {{ }} template.
    expect(screen.getByTestId("value")).toHaveTextContent("nodes.load_tasks.output");
    expect(screen.getByTestId("value").textContent).not.toContain("{{");
  });

  it("Should replace through an existing template close delimiter", () => {
    render(<Harness initial="post {{ .inp }}" />);
    const input = screen.getByTestId("ref") as HTMLInputElement;
    input.setSelectionRange("post {{ .inp".length, "post {{ .inp".length);
    fireEvent.click(input);
    fireEvent.mouseDown(screen.getByText("inputs.slug"));
    expect(screen.getByTestId("value")).toHaveTextContent("post {{ .inputs.slug }}");
    expect(screen.getByTestId("value").textContent).not.toContain("}} }}");
  });

  it("Should move suggestions from the input with arrow keys and accept with Enter", () => {
    render(<Harness />);
    const input = screen.getByTestId("ref") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "post {{ ." } });
    input.setSelectionRange(input.value.length, input.value.length);
    fireEvent.keyUp(input);
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(screen.getByText("nodes.load_tasks.output").closest('[role="option"]')).toHaveAttribute(
      "data-active",
      "true"
    );
    expect(input).toHaveAttribute(
      "aria-activedescendant",
      screen.getByText("nodes.load_tasks.output").closest('[role="option"]')?.id
    );
    fireEvent.keyDown(input, { key: "Enter" });
    expect(screen.getByTestId("value")).toHaveTextContent("post {{ .nodes.load_tasks.output }}");
  });
});
