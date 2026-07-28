import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EntityDialogBody } from "../entity-dialog-body";

describe("EntityDialogBody", () => {
  it("Should render a single body region by default", () => {
    const { container } = render(
      <EntityDialogBody>
        <p>Location</p>
      </EntityDialogBody>
    );

    const body = screen.getByText("Location").closest('[data-slot="entity-dialog-body"]');
    expect(body).toHaveAttribute("data-variant", "default");
    expect(container.querySelector('[data-slot="entity-dialog-body-side"]')).toBeNull();
  });

  it("Should give the split variant two independent scroll owners", () => {
    const { container } = render(
      <EntityDialogBody side={<p>Session defaults</p>} variant="split">
        <p>Location</p>
      </EntityDialogBody>
    );

    const body = container.querySelector('[data-slot="entity-dialog-body"]');
    expect(body).toHaveAttribute("data-variant", "split");

    const main = container.querySelector<HTMLElement>('[data-slot="entity-dialog-body-main"]');
    const side = container.querySelector<HTMLElement>('[data-slot="entity-dialog-body-side"]');
    expect(main).toContainElement(screen.getByText("Location"));
    expect(side).toContainElement(screen.getByText("Session defaults"));
  });
});
