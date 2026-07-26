import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

let initializeAttempts = 0;

const mermaidMock = {
  initialize: vi.fn(),
  render: vi.fn(),
};

vi.mock("mermaid", () => ({
  default: mermaidMock,
}));

import { Mermaid } from "../mermaid";

describe("Mermaid", () => {
  beforeEach(() => {
    initializeAttempts = 0;
    mermaidMock.initialize.mockReset();
    mermaidMock.initialize.mockImplementation(() => {
      initializeAttempts += 1;
      if (initializeAttempts === 1) {
        throw new Error("transient initialization failure");
      }
    });
    mermaidMock.render.mockReset();
    mermaidMock.render.mockImplementation(async (id: string) => ({
      svg: `<svg id="${id}"><title>diagram</title></svg>`,
    }));
  });

  it("retries loading after a transient initialization failure", async () => {
    const { rerender } = render(<Mermaid chart="graph TD; A-->B" />);

    await screen.findByText(
      "Mermaid could not render this diagram in the current browser session."
    );
    expect(mermaidMock.initialize).toHaveBeenCalledTimes(1);
    expect(mermaidMock.render).not.toHaveBeenCalled();

    rerender(<Mermaid chart="graph TD; B-->C" />);

    await waitFor(() => {
      expect(screen.getByLabelText("Mermaid diagram")).toBeTruthy();
    });
    expect(mermaidMock.initialize).toHaveBeenCalledTimes(2);
    expect(mermaidMock.initialize).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        securityLevel: "strict",
        theme: "base",
        themeVariables: expect.objectContaining({
          background: "var(--color-rail)",
          primaryBorderColor: "var(--color-accent)",
          primaryTextColor: "var(--color-fg)",
          lineColor: "var(--color-muted)",
          actorBorder: "var(--color-accent)",
          fontFamily: "var(--font-sans)",
        }),
      })
    );
    expect(mermaidMock.render).toHaveBeenCalledWith(
      expect.stringContaining("agh-mermaid-"),
      "graph TD; B-->C"
    );
  });
});
