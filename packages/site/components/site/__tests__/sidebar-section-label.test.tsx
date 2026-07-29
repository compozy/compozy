import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DocsSidebarSectionLabel } from "../sidebar-section-label";

const mocks = vi.hoisted(() => ({
  depth: 0,
}));

vi.mock("fumadocs-ui/components/sidebar/base", () => ({
  useFolderDepth: () => mocks.depth,
}));

describe("DocsSidebarSectionLabel", () => {
  beforeEach(() => {
    mocks.depth = 0;
  });

  it("Should render a top-level section label at depth 0", () => {
    render(
      <DocsSidebarSectionLabel item={{ type: "separator", name: "Capabilities", $id: "cap" }} />
    );
    const label = screen.getByText("Capabilities");
    expect(label.closest(".docs-sb-label")).not.toBeNull();
    expect(label.closest(".docs-sb-group")).toBeNull();
    expect(label.closest("[data-slot='eyebrow']")).not.toBeNull();
  });

  it("Should render an in-folder group label with a hairline at depth > 0", () => {
    mocks.depth = 1;
    render(<DocsSidebarSectionLabel item={{ type: "separator", name: "Operate", $id: "op" }} />);
    const label = screen.getByText("Operate");
    expect(label.closest(".docs-sb-group")).not.toBeNull();
    expect(label.closest(".docs-sb-label")).toBeNull();
    expect(label.closest(".docs-sb-group")?.querySelector(".docs-sb-group-line")).not.toBeNull();
  });
});
