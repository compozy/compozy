import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "../tabs";

function TabsExample({ orientation }: { orientation?: "horizontal" | "vertical" }) {
  return (
    <Tabs defaultValue="one" orientation={orientation}>
      <TabsList>
        <TabsTrigger value="one">One</TabsTrigger>
        <TabsTrigger value="two">Two</TabsTrigger>
        <TabsTrigger value="three">Three</TabsTrigger>
      </TabsList>
      <TabsContent value="one">Panel one</TabsContent>
      <TabsContent value="two">Panel two</TabsContent>
      <TabsContent value="three">Panel three</TabsContent>
    </Tabs>
  );
}

describe("Tabs", () => {
  it("Should render horizontally by default and activate the initial panel", () => {
    const { container } = render(<TabsExample />);
    const root = container.querySelector("[data-slot=tabs]") as HTMLElement | null;
    expect(root).not.toBeNull();
    expect(root).toHaveAttribute("data-orientation", "horizontal");
    expect(screen.getByText("Panel one")).toBeInTheDocument();
  });

  it("Should honor orientation='vertical'", () => {
    const { container } = render(<TabsExample orientation="vertical" />);
    const root = container.querySelector("[data-slot=tabs]") as HTMLElement | null;
    expect(root).toHaveAttribute("data-orientation", "vertical");
  });

  it("Should swap the active panel when a trigger is clicked", async () => {
    const user = userEvent.setup();
    render(<TabsExample />);
    await user.click(screen.getByRole("tab", { name: "Two" }));
    await waitFor(() => expect(screen.getByRole("tabpanel")).toHaveTextContent("Panel two"));
  });

  it("Should render count and live label slots inside a trigger", () => {
    const { container } = render(
      <Tabs defaultValue="runs">
        <TabsList>
          <TabsTrigger count={3} liveLabel="Live" value="runs">
            Runs
          </TabsTrigger>
        </TabsList>
        <TabsContent value="runs">Panel</TabsContent>
      </Tabs>
    );

    expect(screen.getByRole("tab", { name: /runs3live/i })).toBeInTheDocument();
    expect(container.querySelector('[data-slot="tabs-trigger-count"]')).toHaveTextContent("3");
    expect(container.querySelector('[data-slot="tabs-trigger-live"]')).toHaveTextContent("Live");
  });

  // Invariant: a count that has not been read yet holds its slot with a placeholder that claims
  // no number and is hidden from assistive technology, and the same slot carries the number once
  // it is known, so the tab does not gain a slot (and push its siblings) when the count arrives.
  it("Should hold the count slot with a placeholder while the count is pending", () => {
    const tabs = (count?: "pending" | number) => (
      <Tabs defaultValue="runs">
        <TabsList>
          <TabsTrigger count={count} value="runs">
            Runs
          </TabsTrigger>
        </TabsList>
        <TabsContent value="runs">Panel</TabsContent>
      </Tabs>
    );
    const { container, rerender } = render(tabs("pending"));

    const slot = container.querySelector('[data-slot="tabs-trigger-count"]');
    expect(slot).toHaveAttribute("data-pending");
    expect(slot?.querySelector('[data-slot="skeleton-text"]')).toHaveAttribute(
      "aria-hidden",
      "true"
    );
    expect(slot).toHaveTextContent("");
    expect(screen.getByRole("tab", { name: "Runs" })).toBeInTheDocument();

    rerender(tabs(3));
    const settled = container.querySelector('[data-slot="tabs-trigger-count"]');
    expect(settled).toBe(slot);
    expect(settled).not.toHaveAttribute("data-pending");
    expect(settled).toHaveTextContent("3");
    expect(settled?.querySelector('[data-slot="skeleton-text"]')).toBeNull();

    rerender(tabs());
    expect(container.querySelector('[data-slot="tabs-trigger-count"]')).toBeNull();
  });

  it("Should size the pending placeholder by the digits the caller expects", () => {
    const { container, rerender } = render(
      <Tabs defaultValue="runs">
        <TabsList>
          <TabsTrigger count="pending" value="runs">
            Runs
          </TabsTrigger>
        </TabsList>
      </Tabs>
    );
    const placeholder = () =>
      container.querySelector<HTMLElement>(
        '[data-slot="tabs-trigger-count"] [data-slot="skeleton-text"]'
      );

    expect(placeholder()?.style.width).toBe("2ch");

    rerender(
      <Tabs defaultValue="runs">
        <TabsList>
          <TabsTrigger count="pending" pendingCountDigits={1} value="runs">
            Runs
          </TabsTrigger>
        </TabsList>
      </Tabs>
    );
    expect(placeholder()?.style.width).toBe("1ch");
  });
});
