// Suite: docs MDX blocks (UT-008)
// Invariant: the redrawn blocks keep rendering the props content authors pass in MDX.
// Boundary IN: OperatorNote, Callout, RouteList/RouteRow, Workflow/WorkflowStep markup.
// Boundary OUT: page layout and styling.
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Callout, OperatorNote, RouteList, RouteRow, Workflow, WorkflowStep } from "../mdx-blocks";

describe("docs MDX blocks", () => {
  it("labels an operator note, defaulting to Note", () => {
    render(
      <>
        <OperatorNote>
          <p>Default label body</p>
        </OperatorNote>
        <OperatorNote label="Before you start">
          <p>Custom label body</p>
        </OperatorNote>
      </>
    );

    const [first, second] = screen.getAllByRole("note");
    expect(within(first).getByText("Note")).toBeDefined();
    expect(within(first).getByText("Default label body")).toBeDefined();
    expect(within(second).getByText("Before you start")).toBeDefined();
  });

  it("renders a callout's title and body", () => {
    render(
      <Callout type="warn" title="event_key is required">
        <p>Set it on every trigger.</p>
      </Callout>
    );

    const note = screen.getByRole("note");
    expect(within(note).getByText("event_key is required")).toBeDefined();
    expect(within(note).getByText("Set it on every trigger.")).toBeDefined();
  });

  it("links each route row with its label, title, description and meta", () => {
    render(
      <RouteList>
        <RouteRow
          href="/docs/sessions/lifecycle"
          label="Lifecycle"
          title="Create, activate, stop, and classify sessions"
          description="The state machine."
          meta="State model"
        />
      </RouteList>
    );

    const link = screen.getByRole("link");
    expect(link.getAttribute("href")).toBe("/docs/sessions/lifecycle");
    for (const text of [
      "Lifecycle",
      "Create, activate, stop, and classify sessions",
      "The state machine.",
      "State model",
    ]) {
      expect(within(link).getByText(text)).toBeDefined();
    }
  });

  it("renders workflow steps in order with their titles and bodies", () => {
    render(
      <Workflow>
        <WorkflowStep title="Register the webhook">
          <p>Point the sender at the trigger URL.</p>
        </WorkflowStep>
        <WorkflowStep title="Watch the run">
          <p>Open the session it starts.</p>
        </WorkflowStep>
      </Workflow>
    );

    const steps = within(screen.getByRole("list")).getAllByRole("listitem");
    expect(steps.map(step => step.querySelector("p")?.textContent)).toEqual([
      "Register the webhook",
      "Watch the run",
    ]);
    expect(within(steps[0]).getByText("Point the sender at the trigger URL.")).toBeDefined();
  });
});
