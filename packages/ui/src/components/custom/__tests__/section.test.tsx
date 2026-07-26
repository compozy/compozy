import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Section } from "../section";

describe("Section", () => {
  it("Should render the label + optional right slot + children", () => {
    const { container } = render(
      <Section label="Members" right={<span data-testid="section-right">filter</span>}>
        <p>Body content</p>
      </Section>
    );

    const label = container.querySelector('[data-slot="section-label"]');
    expect(label?.textContent).toBe("Members");
    expect(screen.getByTestId("section-right")).toBeInTheDocument();
    expect(screen.getByText("Body content")).toBeInTheDocument();
  });

  it("Should render note and divided section chrome when requested", () => {
    const { container } = render(
      <Section label="Runtime" note="Read-only daemon state" divided>
        <p>Body content</p>
      </Section>
    );

    expect(container.querySelector('[data-slot="section-note"]')).toHaveTextContent(
      "Read-only daemon state"
    );
  });

  it("Should omit the header when neither label nor right are provided", () => {
    const { container } = render(
      <Section>
        <p>body only</p>
      </Section>
    );
    expect(container.querySelector('[data-slot="section-head"]')).toBeNull();
    expect(container.querySelector('[data-slot="section-body"]')?.textContent).toBe("body only");
  });

  it.each([false, null])("Should omit the header when label is %p", label => {
    const { container } = render(
      <Section label={label}>
        <p>body only</p>
      </Section>
    );
    expect(container.querySelector('[data-slot="section-head"]')).toBeNull();
  });

  it("Should still render numeric zero labels", () => {
    const { container } = render(
      <Section label={0}>
        <p>body only</p>
      </Section>
    );
    expect(container.querySelector('[data-slot="section-label"]')?.textContent).toBe("0");
  });

  it("Should expose data-bordered=null when bordered is not set", () => {
    const { container } = render(<Section label="Members">body</Section>);
    const head = container.querySelector('[data-slot="section-head"]');
    expect(head?.getAttribute("data-bordered")).toBeNull();
  });

  it("Should expose data-bordered=true when bordered is true", () => {
    const { container } = render(
      <Section label="Members" bordered>
        body
      </Section>
    );
    const head = container.querySelector('[data-slot="section-head"]');
    expect(head?.getAttribute("data-bordered")).toBe("true");
  });
});
