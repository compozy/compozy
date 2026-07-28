import { render, screen } from "@testing-library/react";
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
    expect(screen.getByRole("tabpanel")).toHaveTextContent("Panel two");
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

  it("Should render bare count text from the count prop", () => {
    const { container } = render(
      <Tabs defaultValue="lane-a">
        <TabsList>
          <TabsTrigger count={4} value="lane-a">
            Lane A
          </TabsTrigger>
          <TabsTrigger count={2} value="lane-b">
            Lane B
          </TabsTrigger>
        </TabsList>
        <TabsContent value="lane-a">Panel</TabsContent>
        <TabsContent value="lane-b">Panel</TabsContent>
      </Tabs>
    );
    const count = container.querySelector('[data-slot="tabs-trigger-count"]') as HTMLElement | null;
    expect(count?.textContent).toBe("4");
  });
});
