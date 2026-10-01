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

  it("Should render the composer card with an inverted round send button", () => {
    const { container } = render(
      <InputGroup variant="composer">
        <InputGroupTextarea aria-label="Message" />
        <InputGroupAddon align="block-end">
          <InputGroupButton size="tool" aria-label="Attach files" />
          <InputGroupButton size="send" aria-label="Send message" />
        </InputGroupAddon>
      </InputGroup>
    );
    const group = container.querySelector('[data-slot="input-group"]');
    expect(group).toHaveAttribute("data-variant", "composer");
    expect(group).toHaveClass("bg-card", "shadow-card");
    expect(group).not.toHaveClass("bg-canvas");
    const send = screen.getByRole("button", { name: "Send message" });
    expect(send).toHaveClass(
      "bg-primary",
      "text-primary-foreground",
      "size-button-icon-default!",
      "rounded-pill"
    );
    expect(screen.getByRole("button", { name: "Attach files" })).not.toHaveClass("bg-primary");
  });
});
