import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TasksEmptyState } from "../tasks-empty-state";

describe("TasksEmptyState", () => {
  it("Should render the headline with the workspace name and exactly four template cards", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    expect(screen.getByRole("heading", { name: "No tasks yet in Polybot" })).toBeInTheDocument();
    expect(screen.getByTestId("tasks-empty-templates")).toBeInTheDocument();

    const grid = screen.getByTestId("tasks-empty-templates");
    const cards = grid.querySelectorAll("[data-testid^=tasks-empty-template-]");
    expect(cards).toHaveLength(4);
  });

  it("Should paint each template card with the accent / info / warning / neutral tone vocabulary (drop violet/amber)", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    const expected: Record<string, string> = {
      one_shot: "accent",
      recurring: "info",
      human_in_loop: "warning",
      remote_peer: "neutral",
    };

    for (const [templateId, tone] of Object.entries(expected)) {
      const card = screen.getByTestId(`tasks-empty-template-${templateId}`);
      expect(card).toHaveAttribute("data-tone", tone);
    }

    const grid = screen.getByTestId("tasks-empty-templates");
    expect(grid.querySelector('[data-tone="violet"]')).toBeNull();
    expect(grid.querySelector('[data-tone="amber"]')).toBeNull();
  });

  it("Should use the Eyebrow primitive for the templates header", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    const eyebrow = screen.getByTestId("tasks-empty-templates-eyebrow");
    expect(eyebrow).toHaveAttribute("data-slot", "eyebrow");
    expect(eyebrow.className).toContain("eyebrow");
  });

  it("Should fall back to a generic headline when no workspace is provided", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} />);
    expect(screen.getByRole("heading", { name: "No tasks yet" })).toBeInTheDocument();
  });

  it("Should invoke onSelectTemplate from the primary CTA and from any template card", () => {
    const onSelectTemplate = vi.fn();
    render(<TasksEmptyState onSelectTemplate={onSelectTemplate} />);

    fireEvent.click(screen.getByTestId("tasks-empty-cta-new"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("one_shot");

    fireEvent.click(screen.getByTestId("tasks-empty-template-recurring"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("recurring");

    fireEvent.click(screen.getByTestId("tasks-empty-template-remote_peer"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("remote_peer");
  });

  it("Should only render the copy CLI command when the handler is provided", () => {
    const onCopyCli = vi.fn();
    const { rerender } = render(<TasksEmptyState onSelectTemplate={vi.fn()} />);
    expect(screen.queryByTestId("tasks-empty-cta-cli")).not.toBeInTheDocument();

    rerender(<TasksEmptyState onCopyCli={onCopyCli} onSelectTemplate={vi.fn()} />);
    fireEvent.click(screen.getByTestId("tasks-empty-cta-cli"));
    expect(onCopyCli).toHaveBeenCalledTimes(1);
  });
});
