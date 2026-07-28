import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "../button";
import { ButtonGroup, ButtonGroupSeparator, ButtonGroupText } from "../button-group";

describe("ButtonGroup", () => {
  it("Should render children with role=group and horizontal orientation by default", () => {
    const { container } = render(
      <ButtonGroup>
        <Button>One</Button>
        <Button>Two</Button>
      </ButtonGroup>
    );
    const group = container.querySelector('[data-slot="button-group"]');
    expect(group).not.toBeNull();
    expect(group?.getAttribute("role")).toBe("group");
    expect(group?.querySelectorAll('[data-slot="button"]').length).toBe(2);
  });

  it("Should render a separator between direct children", () => {
    const { container } = render(
      <ButtonGroup>
        <Button>Left</Button>
        <ButtonGroupSeparator />
        <Button>Right</Button>
      </ButtonGroup>
    );
    const separator = container.querySelector('[data-slot="button-group-separator"]');
    expect(separator).not.toBeNull();
    expect(separator?.getAttribute("data-orientation")).toBe("vertical");
  });

  it("Should expose orientation via data attribute", () => {
    const { container } = render(
      <ButtonGroup orientation="vertical">
        <Button>Stack</Button>
      </ButtonGroup>
    );
    const group = container.querySelector('[data-slot="button-group"]');
    expect(group?.getAttribute("data-orientation")).toBe("vertical");
  });

  it("Should render ButtonGroupText alongside buttons", () => {
    const { getByText } = render(
      <ButtonGroup>
        <Button>Delete</Button>
        <ButtonGroupText>3 selected</ButtonGroupText>
      </ButtonGroup>
    );
    expect(getByText("3 selected")).toBeInTheDocument();
  });
});
