import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  InputGroupTextarea,
} from "../input-group";

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

  it("Should focus the textarea control when a block-end addon row is pressed", () => {
    render(
      <InputGroup variant="composer">
        <InputGroupTextarea aria-label="Message" />
        <InputGroupAddon align="block-end" data-testid="tools" />
      </InputGroup>
    );
    fireEvent.mouseDown(screen.getByTestId("tools"));
    expect(document.activeElement).toBe(screen.getByRole("textbox", { name: "Message" }));
  });

  // Invariant: composer icon-only tools are the 28 px icon-button square and tool
  // pills share that height, so every control in the row lands on one edge (#826).
  it("Should size composer icon-only tools to the icon-button square and pills to its height", () => {
    render(
      <InputGroup variant="composer">
        <InputGroupTextarea aria-label="Message" />
        <InputGroupAddon align="block-end">
          <InputGroupButton size="tool-icon" aria-label="Attach files">
            <svg aria-hidden="true" />
          </InputGroupButton>
          <InputGroupButton size="tool">Agent</InputGroupButton>
        </InputGroupAddon>
      </InputGroup>
    );
    const icon = screen.getByRole("button", { name: "Attach files" });
    expect(icon).toHaveAttribute("data-size", "tool-icon");
    expect(icon.className).toContain("size-button-icon-default!");
    expect(icon.className).not.toMatch(/(^|\s)px-/);
    const pill = screen.getByRole("button", { name: "Agent" });
    expect(pill).toHaveAttribute("data-size", "tool");
    expect(pill.className).toContain("h-(--size-button-icon-default)!");
  });
});
