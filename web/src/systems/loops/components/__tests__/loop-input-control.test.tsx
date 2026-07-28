import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoopInputControl } from "../target/loop-input-control";
import type { LoopInputSchemaField } from "../../types";

function field(overrides: Partial<LoopInputSchemaField> = {}): LoopInputSchemaField {
  return { type: "string", ...overrides };
}

describe("LoopInputControl", () => {
  it("Should render a text input for string types and emit the raw value", () => {
    const onChange = vi.fn();
    render(
      <LoopInputControl
        name="slug"
        field={field({ type: "string", required: true, default: "main" })}
        value=""
        onChange={onChange}
      />
    );
    const input = screen.getByTestId("loop-input-field-slug");
    expect(input).toHaveAttribute("placeholder", "main");
    fireEvent.change(input, { target: { value: "billing" } });
    expect(onChange).toHaveBeenCalledWith("billing");
  });

  it("Should render a numeric input and emit numbers, clearing to undefined when blank", () => {
    const onChange = vi.fn();
    render(
      <LoopInputControl
        name="max_files"
        field={field({ type: "number" })}
        value={5}
        onChange={onChange}
      />
    );
    const input = screen.getByTestId("loop-input-field-max_files");
    expect(input).toHaveAttribute("type", "number");
    fireEvent.change(input, { target: { value: "12" } });
    expect(onChange).toHaveBeenCalledWith(12);
    fireEvent.change(input, { target: { value: "" } });
    expect(onChange).toHaveBeenCalledWith(undefined);
  });

  it("Should render a switch for boolean types and emit booleans", () => {
    const onChange = vi.fn();
    render(
      <LoopInputControl
        name="auto_commit"
        field={field({ type: "boolean", default: false })}
        value={false}
        onChange={onChange}
      />
    );
    fireEvent.click(screen.getByTestId("loop-input-switch-auto_commit"));
    expect(onChange).toHaveBeenCalledWith(true);
  });
});
