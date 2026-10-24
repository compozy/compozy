// Suite: the shared creation destination statement and its session-only row.
// Invariant: the profile half of the destination appears only while the aggregate
// is on, and it is a label rather than a control; a destination outside any project
// reads "User" without claiming the item is visible to every project. Session
// creation instead asks inline: "In project ▾ · as profile ▾", each picker present
// only for its aggregated axis, and a typed character opens the list filtered by it.
// Boundary IN: what the statement renders for a given destination.
// Boundary OUT: which surfaces mount it, and whether their mutation sends the
// same destination — the creation hosts and their mutation hooks own that.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { CreateDestinationStatement } from "../create-destination";
import { SessionDestinationRow } from "../create-destination-session";

function renderStatement(
  profileDestination: string | null,
  destination: { scope: "project" | "user"; label: string } = {
    scope: "project",
    label: "acme-site",
  }
) {
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      <CreateDestinationStatement
        destination={destination.label}
        kind="create"
        profileDestination={profileDestination}
        scope={destination.scope}
        variant="note"
      />
    </UIProvider>
  );
}

describe("CreateDestinationStatement", () => {
  it("Should state the workspace destination on its own when scoped to one profile", () => {
    renderStatement(null);
    expect(screen.getByTestId("workspace-scope-statement")).toHaveTextContent(
      "Creates in acme-site"
    );
    // A scoped view already answers "whose", so repeating it would be noise.
    expect(screen.queryByTestId("profile-destination-chip")).not.toBeInTheDocument();
  });

  it("Should add the profile destination while the aggregate is on", () => {
    renderStatement("default");
    expect(screen.getByTestId("workspace-scope-statement")).toHaveTextContent(
      "Creates in acme-site"
    );
    expect(screen.getByTestId("profile-destination-chip")).toHaveTextContent("default");
  });

  it("Should state User as the destination outside any project", () => {
    renderStatement(null, { scope: "user", label: "User" });
    const statement = screen.getByTestId("workspace-scope-statement");
    expect(statement).toHaveTextContent("Creates in User.");
    expect(statement).not.toHaveTextContent("every project");
  });
});

const PROJECTS = [
  { value: "ws_alpha", label: "alpha" },
  { value: "ws_beta", label: "beta" },
  { value: "@user", label: "User" },
];

describe("SessionDestinationRow", () => {
  it("Should read as today's statement when no axis is aggregated", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <SessionDestinationRow destination="alpha" root="/workspace/alpha" scope="project" />
      </UIProvider>
    );
    expect(screen.getByTestId("workspace-scope-statement")).toHaveTextContent(
      "Runs in alpha — /workspace/alpha"
    );
    expect(screen.queryByTestId("session-destination-row")).not.toBeInTheDocument();
  });

  it("Should pick the project inline, typing to filter, with User listed last", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <SessionDestinationRow
          destination="alpha"
          project={{ options: PROJECTS, value: "ws_alpha", onChange }}
          scope="project"
        />
      </UIProvider>
    );

    const trigger = screen.getByTestId("session-destination-project");
    expect(screen.getByTestId("session-destination-row")).toHaveTextContent("In");
    expect(screen.queryByTestId("session-destination-profile")).not.toBeInTheDocument();
    await user.tab();
    expect(trigger).toHaveFocus();
    await user.keyboard("b");

    expect(screen.getByTestId("session-destination-project-input")).toHaveValue("b");
    await user.click(screen.getByTestId("session-destination-project-item-ws_beta"));
    expect(onChange).toHaveBeenCalledWith("ws_beta");
  });

  it("Should add the profile picker after the project under both aggregates", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <SessionDestinationRow
          destination="alpha"
          profile={{
            options: [{ value: "marketing", label: "marketing" }],
            value: "marketing",
            onChange: () => undefined,
          }}
          project={{ options: PROJECTS, value: "@user", onChange: () => undefined }}
          scope="user"
        />
      </UIProvider>
    );

    expect(screen.getByTestId("session-destination-project")).toHaveAccessibleName("Project: User");
    expect(screen.getByTestId("session-destination-profile")).toHaveAccessibleName(
      "Profile: marketing"
    );
    expect(screen.queryByTestId("profile-destination-chip")).not.toBeInTheDocument();
  });
});
