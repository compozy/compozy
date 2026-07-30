// Suite: PathGrid audience cards
// Invariant: Every PathCard title is a level-three heading within the docs landing outline.
// Boundary IN: PathCard's rendered semantic structure.
// Boundary OUT: The MDX page's content and route rendering.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PathCard } from "../path-grid";

describe("PathCard", () => {
  it("renders the audience title as a level-three heading", () => {
    render(
      <PathCard
        label="People running agent work"
        title="Start and steer agent work"
        description="Install the runtime and watch durable work finish."
      >
        <a href="/docs/getting-started/installation">Install the runtime</a>
      </PathCard>
    );

    expect(
      screen.getByRole("heading", { level: 3, name: "Start and steer agent work" })
    ).toBeDefined();
  });
});
