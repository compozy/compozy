import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import * as taskTemplates from "../../lib/task-templates";
import { TasksEmptyState } from "../tasks-empty-state";

describe("TasksEmptyState", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should render the headline with the workspace name and exactly three template rows", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    expect(screen.getByRole("heading", { name: "No tasks yet in Polybot" })).toBeInTheDocument();
    expect(screen.getByTestId("tasks-empty-templates")).toBeInTheDocument();
    expect(screen.getByRole("list")).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
  });

  it("Should keep template rows neutral so no benign template reads as a warning", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} workspaceName="Polybot" />);

    const expected: Record<string, string> = {
      one_shot: "neutral",
      recurring: "neutral",
      human_in_loop: "neutral",
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
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("Should fall back to a generic headline when no workspace is provided", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} />);
    expect(screen.getByRole("heading", { name: "No tasks yet" })).toBeInTheDocument();
  });

  it('Should name the active profile so the empty state answers "empty for whom"', () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} profileScopeLabel="Marketing" />);
    expect(screen.getByRole("heading", { name: "No tasks in Marketing yet" })).toBeInTheDocument();
  });

  it("Should prefer the profile over the workspace, which is the wider question", () => {
    render(
      <TasksEmptyState
        onSelectTemplate={vi.fn()}
        profileScopeLabel="Marketing"
        workspaceName="Polybot"
      />
    );
    expect(screen.getByRole("heading", { name: "No tasks in Marketing yet" })).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "No tasks yet in Polybot" })
    ).not.toBeInTheDocument();
  });

  it("Should never name a profile while every profile is on screen", () => {
    // `default` is only where a *new* task would land. Saying the aggregate is
    // empty "in default" would describe one profile while showing all of them.
    render(<TasksEmptyState onSelectTemplate={vi.fn()} profileScopeLabel={null} />);
    const heading = screen.getByRole("heading", { name: /^No tasks/ });
    expect(heading).toHaveTextContent("No tasks in any profile yet");
    expect(heading).not.toHaveTextContent("default");
  });

  it("Should invoke onSelectTemplate from Start from scratch and from Use template", () => {
    const onSelectTemplate = vi.fn();
    render(<TasksEmptyState onSelectTemplate={onSelectTemplate} />);

    fireEvent.click(screen.getByTestId("tasks-empty-cta-new"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("blank");

    fireEvent.click(screen.getByTestId("tasks-empty-template-recurring-use"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("recurring");

    fireEvent.click(screen.getByTestId("tasks-empty-template-human_in_loop-use"));
    expect(onSelectTemplate).toHaveBeenLastCalledWith("human_in_loop");
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
    expect(screen.getByText(/1 attempt/)).toBeVisible();
  });

  it("Should pluralize template attempt counts greater than one", () => {
    const getTaskTemplate = taskTemplates.getTaskTemplate;
    const oneShot = getTaskTemplate("one_shot");
    vi.spyOn(taskTemplates, "getTaskTemplate").mockImplementation(templateID =>
      templateID === "one_shot"
        ? {
            ...oneShot,
            defaults: { ...oneShot.defaults, max_attempts: 3 },
          }
        : getTaskTemplate(templateID)
    );

    render(<TasksEmptyState onSelectTemplate={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /One-shot/ }));

    expect(screen.getByText(/3 attempts/)).toBeVisible();
  });

  it("Should render template-specific facts from the owning template metadata", async () => {
    const user = userEvent.setup();
    render(<TasksEmptyState onSelectTemplate={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /Recurring via automation/ }));
    expect(screen.getByText(/schedule attached in Automation/)).toBeVisible();
  });

  it("Should explain tasks in plain language without CLI commands", () => {
    render(<TasksEmptyState onSelectTemplate={vi.fn()} />);
    expect(screen.getByText(/A task is a piece of work you hand to an agent/)).toBeInTheDocument();
    expect(screen.queryByText(/compozy task create/)).not.toBeInTheDocument();
    expect(screen.getByTestId("tasks-empty-cta-new")).toHaveTextContent("Start from scratch");
  });
});
