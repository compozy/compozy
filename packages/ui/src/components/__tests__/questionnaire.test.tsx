import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  Questionnaire,
  QuestionnaireActions,
  QuestionnaireChoice,
  QuestionnaireChoices,
  QuestionnaireInput,
  QuestionnaireItem,
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
});
