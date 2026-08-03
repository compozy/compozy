// Suite: Extension kit inventory panel
// Invariant: The panel distinguishes shipped kit resources from live ones and renders its empty state.
// Boundary IN: ExtensionKitInventoryPanel rendering from one inventory payload.
// Boundary OUT: Inventory transport/query behavior, owned by adapters/__tests__/extensions-api.test.ts and hooks/__tests__/use-extensions.test.tsx.
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ExtensionKitInventoryPanel } from "../extension-kit-inventory-panel";

describe("ExtensionKitInventoryPanel", () => {
  // UT-063: this panel is where operators distinguish shipped kit resources from live ones.
  it("Should render kit resources with their kind and live state", () => {
    render(
      <ExtensionKitInventoryPanel
        error={null}
        isLoading={false}
        items={[
          {
            id: "automation:weekly-audit",
            kind: "automation",
            live: true,
            name: "weekly-audit",
          },
          { id: "agent:dep-reviewer", kind: "agent", live: false, name: "dep-reviewer" },
        ]}
        onRetry={() => undefined}
      />
    );

    const panel = screen.getByTestId("extension-kit-inventory");
    const rows = within(panel).getAllByTestId("extension-kit-inventory-item");
    expect(rows).toHaveLength(2);
    expect(within(panel).getByText("agent")).toBeInTheDocument();
    expect(within(panel).getByText("automation")).toBeInTheDocument();
    expect(within(rows[0]!).getByText("dep-reviewer")).toBeInTheDocument();
    expect(within(rows[0]!).getByText("shipped")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("weekly-audit")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("live")).toBeInTheDocument();
  });

  it("Should state that no kit resources ship rather than render an empty panel", () => {
    render(
      <ExtensionKitInventoryPanel
        error={null}
        isLoading={false}
        items={[]}
        onRetry={() => undefined}
      />
    );

    expect(
      within(screen.getByTestId("extension-kit-inventory")).getByText(
        "This extension ships no static kit resources."
      )
    ).toBeInTheDocument();
  });
});
