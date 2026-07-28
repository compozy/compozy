import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ActionResultBanner } from "../action-result-banner";

describe("ActionResultBanner", () => {
  it("Should render the title, description, and actions", () => {
    render(
      <ActionResultBanner
        tone="success"
        title="Saved"
        description="Changes are persisted."
        actions={<button type="button">Undo</button>}
      />
    );
    expect(screen.getByText("Saved")).toBeInTheDocument();
    expect(screen.getByText("Changes are persisted.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /undo/i })).toBeInTheDocument();
  });

  it("Should expose role=status and a tone data attribute", () => {
    const { container } = render(<ActionResultBanner tone="danger" title="Failed" />);
    const root = container.querySelector<HTMLElement>('[data-slot="action-result-banner"]');
    expect(root?.getAttribute("role")).toBe("status");
    expect(root?.dataset.tone).toBe("danger");
  });
});
