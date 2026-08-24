// Suite: the shared creation destination statement.
// Invariant: the profile half of the destination appears only while the aggregate
// is on, and it is a label rather than a control.
// Boundary IN: what the statement renders for a given destination.
// Boundary OUT: which surfaces mount it, and whether their mutation sends the
// same destination — the creation hosts and their mutation hooks own that.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UIProvider } from "@compozy/ui";

import { CreateDestinationStatement } from "../create-destination";

function renderStatement(profileDestination: string | null) {
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      <CreateDestinationStatement
        destination="acme-site"
        kind="create"
        profileDestination={profileDestination}
        scope="workspace"
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
});
