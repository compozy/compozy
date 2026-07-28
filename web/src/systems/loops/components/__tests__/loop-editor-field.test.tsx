import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoopEditorField } from "../editor/loop-editor-field";
import type { NumberFieldSpec, TextFieldSpec } from "../../lib/loop-node-schema";

const numberField: NumberFieldSpec = {
  key: "max_fan_out",
  label: "Max fan-out",
  path: ["max_fan_out"],
  type: "number",
};

const jsonField: TextFieldSpec = {
  json: true,
  key: "payload",
  label: "Payload",
  path: ["payload"],
  type: "textarea",
};

describe("LoopEditorField", () => {
  it("Should ignore non-finite number edits instead of writing them to the draft", () => {
    const onChange = vi.fn();
    render(
      <LoopEditorField
        field={numberField}
        raw={{ max_fan_out: 4 }}
        suggestions={[]}
        disabled={false}
        onChange={onChange}
      />
    );
    const input = screen.getByTestId("loop-field-max_fan_out");
    Object.defineProperty(input, "validity", {
      configurable: true,
      value: { badInput: true },
    });
    fireEvent.change(input, { target: { value: "" } });
    expect(onChange).not.toHaveBeenCalled();

    Object.defineProperty(input, "validity", {
      configurable: true,
      value: { badInput: false },
    });
    fireEvent.change(input, { target: { value: "8" } });
    expect(onChange).toHaveBeenCalledWith(["max_fan_out"], 8);
  });

  it("Should associate and announce invalid JSON without committing it", () => {
    const onChange = vi.fn();
    render(
      <LoopEditorField
        disabled={false}
        field={jsonField}
        onChange={onChange}
        raw={{ payload: { ok: true } }}
        suggestions={[]}
      />
    );

    const input = screen.getByRole("textbox", { name: "Payload" });
    fireEvent.change(input, { target: { value: "{" } });

    const error = screen.getByRole("alert");
    expect(error).toHaveTextContent("Invalid JSON — not saved");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute("aria-describedby", error.id);
    expect(onChange).not.toHaveBeenCalled();
  });
});
