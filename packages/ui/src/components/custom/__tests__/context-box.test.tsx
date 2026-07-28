import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ContextBox } from "../context-box";

describe("ContextBox", () => {
  it("Should render label/value pairs as <dt>/<dd>", () => {
    render(
      <ContextBox
        title="Run context"
        entries={[
          { label: "Run id", value: "run-001" },
          { label: "Started", value: "2026-05-09" },
        ]}
      />
    );
    expect(screen.getByText("Run context")).toBeInTheDocument();
    const dts = document.querySelectorAll("dt");
    const dds = document.querySelectorAll("dd");
    expect(dts).toHaveLength(2);
    expect(dds).toHaveLength(2);
    expect(screen.getByText("run-001")).toBeInTheDocument();
  });
});
