import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PageShell } from "../page-shell";

describe("PageShell", () => {
  it("Should default to density=comfortable when no density prop is provided", () => {
    render(
      <PageShell data-testid="shell">
        <div>body</div>
      </PageShell>
    );
    const shell = screen.getByTestId("shell");
    expect(shell).toHaveAttribute("data-density", "comfortable");
    const content = shell.querySelector('[data-slot="page-content"]');
    expect(content).not.toBeNull();
    expect(content).toHaveAttribute("data-density", "comfortable");
    expect(content?.className).toContain("px-9");
    expect(content?.className).toContain("max-w-content-max");
  });

  it("Should emit data-density=compact when density=compact is set", () => {
    render(
      <PageShell density="compact" data-testid="shell">
        <div>body</div>
      </PageShell>
    );
    expect(screen.getByTestId("shell")).toHaveAttribute("data-density", "compact");
  });

  it("Should emit data-density=route when density=route is set", () => {
    render(
      <PageShell density="route" data-testid="shell">
        <div>body</div>
      </PageShell>
    );
    expect(screen.getByTestId("shell")).toHaveAttribute("data-density", "route");
  });

  it("Should render banner content inside the banner slot", () => {
    render(
      <PageShell
        density="route"
        banner={<div data-testid="shell-banner">restart!</div>}
        data-testid="shell"
      >
        <div>body</div>
      </PageShell>
    );
    expect(screen.getByTestId("shell-banner")).toHaveTextContent("restart!");
  });
});
