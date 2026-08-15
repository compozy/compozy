import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TasksEmptyState } from "../tasks-empty-state";

describe("TasksEmptyState", () => {
  it("Should render the headline with the workspace name and exactly four template rows", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    expect(screen.getByRole("heading", { name: "No tasks yet in Polybot" })).toBeInTheDocument();
    expect(screen.getByTestId("tasks-empty-templates")).toBeInTheDocument();
    expect(screen.getByRole("list")).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(4);
  });

  it("Should paint each template row with the accent / info / warning / neutral tone vocabulary", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    const expected: Record<string, string> = {
      one_shot: "accent",
      recurring: "info",
      human_in_loop: "warning",
      remote_peer: "neutral",
    };

    for (const [templateId, tone] of Object.entries(expected)) {
      expect(screen.getByTestId(`tasks-empty-template-${templateId}`)).toHaveAttribute(
        "data-tone",
        tone
      );
    }
  });

  it("Should label the templates panel with a live count", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    expect(screen.getByRole("heading", { name: /Start from a template/i })).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
  });

  it("Should fall back to a generic headline when no workspace is provided", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} />);
    expect(screen.getByRole("heading", { name: "No tasks yet" })).toBeInTheDocument();
  });

  it("Should invoke onSelectTemplate from Blank task and from Use template", () => {
    const onSelectTemplate = vi.fn();
    render(<TasksEmptyState onSelectTemplate={onSelectTemplate} />);

    fireEvent.click(screen.getByTestId("tasks-empty-cta-new"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("blank");

    fireEvent.click(screen.getByTestId("tasks-empty-template-recurring-use"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("recurring");

    fireEvent.click(screen.getByTestId("tasks-empty-template-remote_peer-use"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("remote_peer");
  });

  it("Should reveal template review details only after the opener is expanded", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} />);

    expect(
      screen.queryByText(/A single task with one run. Good default for ad-hoc work./)
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /One-shot/ }));
    expect(
      screen.getByText(/A single task with one run. Good default for ad-hoc work./)
    ).toBeVisible();
  });

  it("Should only render the copy CLI command when the handler is provided", () => {
    const onCopyCli = vi.fn();
    const { rerender } = render(<TasksEmptyState onSelectTemplate={vi.fn()} />);
    expect(screen.queryByTestId("tasks-empty-cta-cli")).not.toBeInTheDocument();
    expect(screen.getByText(/compozy tasks new/)).toBeInTheDocument();

    rerender(<TasksEmptyState onCopyCli={onCopyCli} onSelectTemplate={vi.fn()} />);
    fireEvent.click(screen.getByTestId("tasks-empty-cta-cli"));
    expect(onCopyCli).toHaveBeenCalledTimes(1);
  });
});
