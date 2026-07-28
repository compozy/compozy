import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoopRunInputField } from "../run-form/loop-run-input-field";
import type { LoopInputSchemaField } from "../../types";

function field(overrides: Partial<LoopInputSchemaField> = {}): LoopInputSchemaField {
  return { type: "string", ...overrides };
}

describe("LoopRunInputField", () => {
  it("Should render a mono text control for string/file/ref types", () => {
    for (const type of ["string", "file", "ref"] as const) {
      const { unmount } = render(
        <LoopRunInputField name="x" field={field({ type })} value="" onChange={vi.fn()} />
      );
      expect(screen.getByTestId("loop-run-field-x")).toHaveAttribute("data-input-type", type);
      expect(screen.getByTestId("loop-run-field-input-x")).toHaveAttribute("type", "text");
      unmount();
    }
  });

  it("Should render a numeric control that emits numbers and clears to undefined", () => {
    const onChange = vi.fn();
    render(
      <LoopRunInputField
        name="max_files"
        field={field({ type: "number" })}
        value={5}
        onChange={onChange}
      />
    );
    const input = screen.getByTestId("loop-run-field-input-max_files");
    expect(input).toHaveAttribute("type", "number");
    fireEvent.change(input, { target: { value: "12" } });
    expect(onChange).toHaveBeenCalledWith(12);
    fireEvent.change(input, { target: { value: "" } });
    expect(onChange).toHaveBeenCalledWith(undefined);
  });

  it("Should render a switch for boolean types and emit booleans", () => {
    const onChange = vi.fn();
    render(
      <LoopRunInputField
        name="auto_commit"
        field={field({ type: "boolean", default: false })}
        value={false}
        onChange={onChange}
      />
    );
    fireEvent.click(screen.getByTestId("loop-run-switch-auto_commit"));
    expect(onChange).toHaveBeenCalledWith(true);
    expect(screen.getByTestId("loop-run-field-auto_commit")).toHaveAttribute(
      "data-input-type",
      "boolean"
    );
  });

  it("Should render an avatar-prefixed control for agent types", () => {
    render(
      <LoopRunInputField
        name="implementer"
        field={field({ type: "agent", default: "code_implementer" })}
        value="code_implementer"
        onChange={vi.fn()}
      />
    );
    const wrapper = screen.getByTestId("loop-run-field-implementer");
    expect(wrapper).toHaveAttribute("data-input-type", "agent");
    expect(wrapper).toHaveTextContent("co");
  });

  it("Should render the declared type verbatim in the badge (boolean, not bool)", () => {
    render(
      <LoopRunInputField
        name="auto_commit"
        field={field({ type: "boolean" })}
        value={false}
        onChange={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-field-auto_commit")).toHaveTextContent("boolean");
  });

  it("Should surface the required marker and inline error", () => {
    render(
      <LoopRunInputField
        name="slug"
        field={field({ type: "string", required: true })}
        value=""
        error="slug is required to run this loop."
        onChange={vi.fn()}
      />
    );
    expect(screen.getByLabelText("required")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-field-error-slug")).toHaveTextContent("slug is required");
  });
});
