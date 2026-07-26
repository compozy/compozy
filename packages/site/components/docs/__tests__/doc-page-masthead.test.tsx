import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DocPageMasthead } from "../doc-page-masthead";

describe("DocPageMasthead", () => {
  it("uses the nested core section label for runtime concept pages", () => {
    render(
      <DocPageMasthead
        kind="runtime"
        slug={["core", "sessions", "lifecycle"]}
        title="Session Lifecycle"
        description="Durable runtime unit."
      />
    );

    expect(screen.getByText("AGH Runtime")).toBeTruthy();
    expect(screen.getByText("Sessions")).toBeTruthy();
    expect(
      screen.getByText(
        "Sessions guidance shaped for scanability, day-two clarity, and operator context."
      )
    ).toBeTruthy();
  });

  it("uses the cli subgroup label for runtime reference pages", () => {
    render(
      <DocPageMasthead
        kind="runtime"
        slug={["cli-reference", "agent", "info"]}
        title="agh agent info"
        description="Inspect one agent."
      />
    );

    expect(screen.getByText("Agent")).toBeTruthy();
  });

  it("uses the root runtime overview label on the runtime landing page", () => {
    render(
      <DocPageMasthead
        kind="runtime"
        slug={[]}
        title="Runtime Documentation"
        description="AGH Runtime overview."
      />
    );

    expect(screen.getByText("Runtime Overview")).toBeTruthy();
  });

  it("skips empty slug parts when building labels", () => {
    render(
      <DocPageMasthead
        kind="runtime"
        slug={["cli-reference", "agent--info"]}
        title="agh agent info"
        description="Inspect one agent."
      />
    );

    expect(screen.getByText("Agent Info")).toBeTruthy();
    expect(screen.queryByText(/undefined/i)).toBeNull();
  });
});
