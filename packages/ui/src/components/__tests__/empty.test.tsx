import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Empty } from "../empty";

function DummyIcon({ className }: { className?: string }) {
  return <svg data-testid="empty-custom-icon" className={className} />;
}

describe("Empty", () => {
  it("Should render the centered icon well + title + description + action slot", () => {
    const { container } = render(
      <Empty
        icon={DummyIcon}
        title="No tasks"
        description="Create a task to see it here."
        action={<button type="button">New task</button>}
      />
    );

    const empty = container.querySelector('[data-slot="empty"]');
    expect(empty).not.toBeNull();
    expect(screen.getByText("No tasks")).toBeInTheDocument();
    expect(screen.getByText("Create a task to see it here.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New task" })).toBeInTheDocument();
    expect(screen.getByTestId("empty-custom-icon")).toBeInTheDocument();
    expect(container.querySelector('[data-slot="empty-title"]')?.tagName).toBe("H3");

    const slots = Array.from(empty?.children ?? []).map(node => node.getAttribute("data-slot"));
    expect(slots).toEqual(["empty-icon", "empty-title", "empty-description", "empty-action"]);
  });

  it("Should omit the description and action slots when those props are absent", () => {
    const { container } = render(<Empty title="Nothing here" />);
    expect(container.querySelector('[data-slot="empty-description"]')).toBeNull();
    expect(container.querySelector('[data-slot="empty-action"]')).toBeNull();
  });

  it("Should fall back to a default icon when none is provided", () => {
    const { container } = render(<Empty title="Nothing here" />);
    const iconSlot = container.querySelector('[data-slot="empty-icon"]');
    expect(iconSlot).not.toBeNull();
    expect(iconSlot?.querySelector("svg")).not.toBeNull();
  });

  it("Should accept a pre-rendered ReactNode as the icon", () => {
    const { container } = render(
      <Empty title="Nothing here" icon={<svg data-testid="inline-icon" viewBox="0 0 16 16" />} />
    );
    const iconSlot = container.querySelector('[data-slot="empty-icon"]');
    expect(iconSlot).not.toBeNull();
    expect(iconSlot?.querySelector('[data-testid="inline-icon"]')).not.toBeNull();
  });

  it("Should avoid wrapping composed title content in a heading by default", () => {
    const { container } = render(
      <Empty
        title={
          <div data-testid="empty-composed-title">
            <span>Disconnected</span>
          </div>
        }
      />
    );

    const titleSlot = container.querySelector('[data-slot="empty-title"]');
    expect(titleSlot?.tagName).toBe("DIV");
    expect(screen.getByTestId("empty-composed-title")).toBeInTheDocument();
  });

  it("Should allow callers to override the title element explicitly", () => {
    const { container } = render(<Empty title="Nothing here" titleAs="h2" />);
    expect(container.querySelector('[data-slot="empty-title"]')?.tagName).toBe("H2");
  });

  it("Should expose data-fill=true by default", () => {
    const { container } = render(<Empty title="Nothing here" />);
    const empty = container.querySelector('[data-slot="empty"]');
    expect(empty?.getAttribute("data-fill")).toBe("true");
  });

  it("Should expose data-fill=false when fill is disabled", () => {
    const { container } = render(<Empty title="Nothing here" fill={false} />);
    const empty = container.querySelector('[data-slot="empty"]');
    expect(empty?.getAttribute("data-fill")).toBe("false");
  });

  it("Should render a framed, non-filling card with the cause slot in order", () => {
    const { container } = render(
      <Empty framed title="Unable to load" description="It broke." cause="stack: boom" />
    );
    const empty = container.querySelector('[data-slot="empty"]');
    expect(empty?.getAttribute("data-framed")).toBe("true");
    expect(empty?.getAttribute("data-fill")).toBe("false");
    expect(empty?.className).toContain("border-line");
    const slots = Array.from(empty?.children ?? []).map(node => node.getAttribute("data-slot"));
    expect(slots).toEqual(["empty-icon", "empty-title", "empty-description", "empty-cause"]);
  });
});
