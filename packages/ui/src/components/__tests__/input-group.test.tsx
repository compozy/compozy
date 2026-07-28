import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { InputGroup, InputGroupAddon, InputGroupInput, InputGroupTextarea } from "../input-group";

describe("InputGroup", () => {
  it("Should place inline-start addon before the input without clipping", () => {
    const { container } = render(
      <InputGroup>
        <InputGroupAddon>@</InputGroupAddon>
        <InputGroupInput placeholder="handle" />
      </InputGroup>
    );
    const group = container.querySelector('[data-slot="input-group"]');
    expect(group).not.toBeNull();
    const addon = container.querySelector('[data-slot="input-group-addon"]');
    expect(addon?.getAttribute("data-align")).toBe("inline-start");
    const input = container.querySelector('[data-slot="input-group-control"]');
    expect(input).not.toBeNull();
  });

  it("Should place inline-end addon after the input", () => {
    const { container } = render(
      <InputGroup>
        <InputGroupInput defaultValue="2123" />
        <InputGroupAddon align="inline-end">TCP</InputGroupAddon>
      </InputGroup>
    );
    const addon = container.querySelector('[data-slot="input-group-addon"]');
    expect(addon?.getAttribute("data-align")).toBe("inline-end");
  });

  it("Should focus the input when the addon container receives mouse down", () => {
    render(
      <InputGroup>
        <InputGroupAddon data-testid="addon">@</InputGroupAddon>
        <InputGroupInput placeholder="handle" />
      </InputGroup>
    );
    const addon = screen.getByTestId("addon");
    fireEvent.mouseDown(addon);
    const input = document.querySelector<HTMLInputElement>('[data-slot="input-group-control"]');
    expect(document.activeElement).toBe(input);
  });

  it("Should render InputGroupTextarea as the control when multi-line", () => {
    const { container } = render(
      <InputGroup>
        <InputGroupTextarea placeholder="prompt" />
      </InputGroup>
    );
    const control = container.querySelector("textarea[data-slot='input-group-control']");
    expect(control).not.toBeNull();
  });
});
