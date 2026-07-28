import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { NativeSelect, NativeSelectOptGroup, NativeSelectOption } from "../native-select";

describe("NativeSelect", () => {
  it("Should forward value and emit onChange", () => {
    const handleChange = vi.fn();
    render(
      <NativeSelect aria-label="Env" value="dev" onChange={handleChange}>
        <NativeSelectOption value="dev">dev</NativeSelectOption>
        <NativeSelectOption value="prod">prod</NativeSelectOption>
      </NativeSelect>
    );
    const control = screen.getByLabelText("Env") as HTMLSelectElement;
    expect(control.value).toBe("dev");
    fireEvent.change(control, { target: { value: "prod" } });
    expect(handleChange).toHaveBeenCalledTimes(1);
  });

  it("Should render the chevron icon alongside the control", () => {
    const { container } = render(
      <NativeSelect aria-label="Env" defaultValue="dev">
        <NativeSelectOption value="dev">dev</NativeSelectOption>
      </NativeSelect>
    );
    expect(container.querySelector('[data-slot="native-select-icon"]')).not.toBeNull();
  });

  it("Should update via controlled state when the user picks another option", () => {
    function Harness() {
      const [value, setValue] = useState("dev");
      return (
        <NativeSelect
          aria-label="Env"
          value={value}
          onChange={event => setValue(event.currentTarget.value)}
        >
          <NativeSelectOption value="dev">dev</NativeSelectOption>
          <NativeSelectOption value="prod">prod</NativeSelectOption>
        </NativeSelect>
      );
    }
    render(<Harness />);
    const control = screen.getByLabelText("Env") as HTMLSelectElement;
    fireEvent.change(control, { target: { value: "prod" } });
    expect(control.value).toBe("prod");
  });

  it("Should render grouped options inside NativeSelectOptGroup", () => {
    const { container } = render(
      <NativeSelect aria-label="Agent" defaultValue="claude">
        <NativeSelectOptGroup label="Local">
          <NativeSelectOption value="claude">Claude</NativeSelectOption>
        </NativeSelectOptGroup>
      </NativeSelect>
    );
    const optgroup = container.querySelector('[data-slot="native-select-optgroup"]');
    expect(optgroup?.getAttribute("label")).toBe("Local");
  });

  it("Should apply the compact size variant via data attribute", () => {
    const { container } = render(
      <NativeSelect aria-label="Env" defaultValue="dev" size="sm">
        <NativeSelectOption value="dev">dev</NativeSelectOption>
      </NativeSelect>
    );
    const wrapper = container.querySelector('[data-slot="native-select-wrapper"]');
    expect(wrapper?.getAttribute("data-size")).toBe("sm");
  });
});
