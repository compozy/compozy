import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  Questionnaire,
  QuestionnaireActions,
  QuestionnaireChoice,
  QuestionnaireChoices,
  QuestionnaireDescription,
  QuestionnaireError,
  QuestionnaireInput,
  QuestionnaireItem,
  QuestionnaireNext,
  QuestionnairePrevious,
  QuestionnaireProgress,
  QuestionnaireSkip,
  QuestionnaireSubmit,
  QuestionnaireTitle,
} from "../questionnaire";

/**
 * Canonical suite for the Questionnaire flow.
 *
 * Invariant: the root is a real form — the answer lives in the named DOM input
 * until submit reads it through FormData, never mirrored into state — and a
 * choice answers through its fieldset name.
 */

describe("Questionnaire", () => {
  it("Should submit a free-text answer through the form, from the named input", async () => {
    const values: Array<string | null> = [];
    const onSubmit = vi.fn((event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      const answer = data.get("database-password");
      values.push(typeof answer === "string" ? answer : null);
    });
    render(
      <Questionnaire onSubmit={onSubmit}>
        <QuestionnaireItem name="database-password" required>
          <QuestionnaireTitle>A password is needed</QuestionnaireTitle>
          <QuestionnaireInput aria-label="Hidden input" type="password" />
          <QuestionnaireActions>
            <QuestionnaireSubmit>Send</QuestionnaireSubmit>
          </QuestionnaireActions>
        </QuestionnaireItem>
      </Questionnaire>
    );

    await userEvent.type(screen.getByLabelText("Hidden input"), "hunter2-long");
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    expect(onSubmit).toHaveBeenCalledOnce();
    expect(values).toEqual(["hunter2-long"]);
  });

  it("Should answer a choice question through its fieldset name", async () => {
    const values: Array<string | null> = [];
    const onSubmit = vi.fn((event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      const answer = data.get("restart");
      values.push(typeof answer === "string" ? answer : null);
    });
    render(
      <Questionnaire onSubmit={onSubmit}>
        <QuestionnaireItem name="restart" required>
          <QuestionnaireTitle>Restart the dev server?</QuestionnaireTitle>
          <QuestionnaireChoices>
            <QuestionnaireChoice value="now">Restart now</QuestionnaireChoice>
            <QuestionnaireChoice value="later">Restart later</QuestionnaireChoice>
          </QuestionnaireChoices>
          <QuestionnaireActions>
            <QuestionnaireSubmit>Send</QuestionnaireSubmit>
          </QuestionnaireActions>
        </QuestionnaireItem>
      </Questionnaire>
    );

    await userEvent.click(screen.getByRole("radio", { name: "Restart now" }));
    await userEvent.click(screen.getByRole("button", { name: "Send" }));

    expect(onSubmit).toHaveBeenCalledOnce();
    expect(values).toEqual(["now"]);
  });

  it("Should expose validation and navigation through every flow slot", async () => {
    render(
      <Questionnaire
        items={[
          {
            name: "scope",
            required: true,
            choices: [{ value: "web" }, { value: "all" }],
          },
          { name: "note" },
        ]}
      >
        <QuestionnaireProgress />
        <QuestionnaireItem name="scope" required>
          <QuestionnaireTitle>Which files should change?</QuestionnaireTitle>
          <QuestionnaireDescription>Choose the narrowest useful scope.</QuestionnaireDescription>
          <QuestionnaireChoices>
            <QuestionnaireChoice value="web">Only web/</QuestionnaireChoice>
            <QuestionnaireChoice value="all">The whole repo</QuestionnaireChoice>
          </QuestionnaireChoices>
          <QuestionnaireError />
        </QuestionnaireItem>
        <QuestionnaireItem name="note">
          <QuestionnaireTitle>Anything else?</QuestionnaireTitle>
          <QuestionnaireInput aria-label="Optional note" />
          <QuestionnaireError />
        </QuestionnaireItem>
        <QuestionnaireActions>
          <QuestionnairePrevious />
          <QuestionnaireSkip />
          <QuestionnaireNext />
          <QuestionnaireSubmit />
        </QuestionnaireActions>
      </Questionnaire>
    );

    expect(screen.getByRole("progressbar")).toHaveTextContent("Question 1 of 2");
    expect(screen.getByText("Choose the narrowest useful scope.")).toBeVisible();
    const next = screen.getByRole("button", { name: "Next" });
    expect(next).toHaveAttribute("data-size", "sm");
    expect(next).toHaveAttribute("data-variant", "default");

    await userEvent.click(next);
    expect(screen.getByRole("alert")).toHaveTextContent("Choose an answer to continue.");

    await userEvent.click(screen.getByRole("radio", { name: "Only web/" }));
    await userEvent.click(next);
    expect(screen.getByText("Anything else?")).toBeVisible();
    const previous = screen.getByRole("button", { name: "Previous" });
    expect(previous).toHaveAttribute("data-size", "sm");
    expect(previous).toHaveAttribute("data-variant", "outline");
    expect(screen.getByRole("button", { name: "Skip" })).toHaveAttribute("data-variant", "ghost");
    expect(screen.getByRole("button", { name: "Submit" })).toHaveAttribute(
      "data-variant",
      "default"
    );

    await userEvent.click(previous);
    expect(screen.getByText("Which files should change?")).toBeVisible();
  });
});
