// Suite: DiffCard rendering
// Invariant: every line keeps its kind, sign, and exact text in order, and the
// header stat names additions and deletions.
// Boundary IN: DiffCard. Boundary OUT: diff computation (owned by callers).
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DiffCard } from "../diff-card";

describe("DiffCard", () => {
  it("Should render the path, the stat, and each line with its kind in order", () => {
    const { container } = render(
      <DiffCard
        path="src/app.ts"
        additions={1}
        deletions={1}
        lines={[
          { id: "c0", kind: "context", text: "function a() {" },
          { id: "d1", kind: "del", text: "  return 1;" },
          { id: "a1", kind: "add", text: "  return 2;" },
        ]}
      />
    );

    expect(screen.getByText("src/app.ts")).toBeInTheDocument();
    expect(container.querySelector('[data-slot="diff-card-stat"]')).toHaveTextContent("+1−1");
    const lines = [...container.querySelectorAll('[data-slot="diff-card-line"]')];
    expect(lines.map(line => line.getAttribute("data-kind"))).toEqual(["context", "del", "add"]);
    expect(lines.map(line => line.textContent)).toEqual([
      " function a() {",
      "−  return 1;",
      "+  return 2;",
    ]);
  });

  it("Should omit the header when neither a path nor a stat is given", () => {
    const { container } = render(<DiffCard lines={[{ id: "a0", kind: "add", text: "x" }]} />);
    expect(container.querySelector('[data-slot="diff-card-header"]')).toBeNull();
  });
});
