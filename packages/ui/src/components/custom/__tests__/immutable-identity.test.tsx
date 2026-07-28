import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ImmutableIdentity } from "../immutable-identity";

describe("ImmutableIdentity", () => {
  it("Should render locked fields as readable rows, never as inputs", () => {
    const { container } = render(
      <ImmutableIdentity
        hint="The update contract cannot change these."
        rows={[
          { label: "Platform", value: "slack" },
          { label: "Scope", value: "workspace" },
        ]}
      />
    );

    expect(container.querySelectorAll("input")).toHaveLength(0);
    expect(container.querySelectorAll("select")).toHaveLength(0);
    expect(container.querySelectorAll("[disabled]")).toHaveLength(0);

    const rows = container.querySelector('[data-slot="immutable-identity-rows"]');
    expect(rows?.tagName).toBe("DL");
    expect(screen.getByText("Platform")).toBeInTheDocument();
    expect(screen.getByText("slack")).toBeInTheDocument();
    expect(screen.getByText("The update contract cannot change these.")).toBeInTheDocument();
  });

  it("Should render mono values in the mono stack", () => {
    const { container } = render(
      <ImmutableIdentity rows={[{ label: "Reference", mono: true, value: "vault:mcp/github" }]} />
    );

    const value = screen.getByText("vault:mcp/github");
    expect(value.className).toContain("font-mono");
    expect(container.querySelector('[data-slot="immutable-identity-hint"]')).toBeNull();
  });
});
