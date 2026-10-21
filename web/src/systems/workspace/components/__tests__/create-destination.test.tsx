// Suite: the shared creation destination statement.
// Invariant: the profile half of the destination appears only while the aggregate
// is on, and it is a label rather than a control; a destination outside any project
// reads "User" without claiming the item is visible to every project.
// Boundary IN: what the statement renders for a given destination.
// Boundary OUT: which surfaces mount it, and whether their mutation sends the
// same destination — the creation hosts and their mutation hooks own that.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UIProvider } from "@compozy/ui";

import { CreateDestinationStatement } from "../create-destination";

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
