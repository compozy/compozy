import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "../accordion";

const items = [
  { value: "one", title: "First", body: "First body" },
  { value: "two", title: "Second", body: "Second body" },
] as const;

function AccordionExample({
  multiple,
  defaultValue,
}: {
  multiple?: boolean;
  defaultValue?: string[];
}) {
  return (
    <Accordion multiple={multiple} defaultValue={defaultValue}>
      {items.map(item => (
        <AccordionItem key={item.value} value={item.value}>
          <AccordionTrigger>{item.title}</AccordionTrigger>
          <AccordionContent>{item.body}</AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  );
}

describe("Accordion", () => {
  it("Should render the root, item, trigger, and content slots", () => {
    const { container } = render(<AccordionExample defaultValue={["one"]} />);
    expect(container.querySelector("[data-slot=accordion]")).toBeInTheDocument();
    expect(container.querySelectorAll("[data-slot=accordion-item]").length).toBe(2);
    expect(container.querySelectorAll("[data-slot=accordion-trigger]").length).toBe(2);
  });

  it("Should start with the default item expanded", () => {
    render(<AccordionExample defaultValue={["one"]} />);
    expect(screen.getByRole("button", { name: "First" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("button", { name: "Second" })).toHaveAttribute(
      "aria-expanded",
      "false"
    );
  });

  it("Should close the previously open item when single-selection and a new item opens", async () => {
    const user = userEvent.setup();
    render(<AccordionExample defaultValue={["one"]} />);
    const first = screen.getByRole("button", { name: "First" });
    const second = screen.getByRole("button", { name: "Second" });
    await user.click(second);
    expect(second).toHaveAttribute("aria-expanded", "true");
    expect(first).toHaveAttribute("aria-expanded", "false");
  });

  it("Should keep multiple items open when multiple=true", async () => {
    const user = userEvent.setup();
    render(<AccordionExample multiple defaultValue={["one"]} />);
    const second = screen.getByRole("button", { name: "Second" });
    await user.click(second);
    expect(screen.getByRole("button", { name: "First" })).toHaveAttribute("aria-expanded", "true");
    expect(second).toHaveAttribute("aria-expanded", "true");
  });
});
