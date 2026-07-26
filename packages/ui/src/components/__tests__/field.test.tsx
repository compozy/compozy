import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "../field";
import { Input } from "../input";

describe("Field", () => {
  it("Should render a vertical group with label + control + description wired by aria-describedby", () => {
    render(
      <Field>
        <FieldLabel htmlFor="name">Name</FieldLabel>
        <Input id="name" aria-describedby="name-help" defaultValue="Claude" />
        <FieldDescription id="name-help">Display name for the agent.</FieldDescription>
      </Field>
    );
    const input = screen.getByLabelText("Name");
    expect(input).toHaveAttribute("aria-describedby", "name-help");
    expect(screen.getByText("Display name for the agent.")).toBeInTheDocument();
  });

  it("Should expose orientation via data attribute", () => {
    const { container } = render(
      <Field orientation="horizontal">
        <FieldLabel>Name</FieldLabel>
      </Field>
    );
    const field = container.querySelector('[data-slot="field"]');
    expect(field?.getAttribute("data-orientation")).toBe("horizontal");
  });

  it("Should render FieldError with role=alert and single message", () => {
    render(
      <Field data-invalid>
        <FieldLabel htmlFor="token">Token</FieldLabel>
        <Input id="token" aria-invalid aria-describedby="token-error" />
        <FieldError id="token-error">Token is required.</FieldError>
      </Field>
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Token is required.");
    expect(alert.getAttribute("id")).toBe("token-error");
  });

  it("Should deduplicate multiple errors and render them as a list", () => {
    const { container } = render(
      <FieldError
        errors={[
          { message: "Must not be empty" },
          { message: "Must not be empty" },
          { message: "Must be alphanumeric" },
        ]}
      />
    );
    const items = container.querySelectorAll("li");
    expect(items.length).toBe(2);
    expect(items[0]?.textContent).toBe("Must not be empty");
    expect(items[1]?.textContent).toBe("Must be alphanumeric");
  });

  it("Should preserve valid falsy children instead of treating them as missing", () => {
    render(<FieldError>{0}</FieldError>);
    expect(screen.getByRole("alert")).toHaveTextContent("0");
  });

  it("Should ignore empty error entries before deciding between single and list output", () => {
    const { container } = render(
      <FieldError
        errors={[{ message: "Must not be empty" }, {}, { message: "Must not be empty" }]}
      />
    );

    expect(container.querySelectorAll("li")).toHaveLength(0);
    expect(screen.getByRole("alert")).toHaveTextContent("Must not be empty");
  });

  it("Should render nothing when FieldError has no children and no errors", () => {
    const { container } = render(<FieldError />);
    expect(container.firstChild).toBeNull();
  });

  it("Should wrap fieldset + legend + grouped fields", () => {
    render(
      <FieldSet>
        <FieldLegend>Session metadata</FieldLegend>
        <FieldGroup>
          <Field>
            <FieldContent>
              <FieldLabel htmlFor="tag">Tag</FieldLabel>
              <Input id="tag" defaultValue="oncall" />
            </FieldContent>
          </Field>
        </FieldGroup>
      </FieldSet>
    );
    expect(screen.getByRole("group", { name: "Session metadata" })).toBeInTheDocument();
  });
});
