import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import {
  Questionnaire,
  QuestionnaireActions,
  QuestionnaireChoice,
  QuestionnaireChoiceDescription,
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
 * A question-and-answer flow: a real form root, one fieldset per question, and
 * shortcut-aware choices. Answers live in the DOM inputs until submit, which is
 * what lets a masked answer stay unstored.
 */
const meta: Meta<typeof Questionnaire> = {
  title: "components/Questionnaire",
  component: Questionnaire,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Question-and-answer form with choice, free-text, and multi-step flows.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Choice question with shortcuts and optional supporting detail. */
export const Choices: Story = {
  args: {},
  render: () => (
    <div className="w-[420px]">
      <Questionnaire onSubmit={event => event.preventDefault()} shortcuts="numbers">
        <QuestionnaireItem name="restart" required>
          <QuestionnaireTitle>Restart the dev server?</QuestionnaireTitle>
          <QuestionnaireDescription>
            The config change only applies after a restart.
          </QuestionnaireDescription>
          <QuestionnaireChoices>
            <QuestionnaireChoice value="now">
              Restart now
              <QuestionnaireChoiceDescription>
                Interrupts anyone watching this terminal.
              </QuestionnaireChoiceDescription>
            </QuestionnaireChoice>
            <QuestionnaireChoice value="later">Restart later</QuestionnaireChoice>
          </QuestionnaireChoices>
          <QuestionnaireActions>
            <QuestionnaireSkip onClick={fn()} />
            <QuestionnaireSubmit>Send</QuestionnaireSubmit>
          </QuestionnaireActions>
        </QuestionnaireItem>
      </Questionnaire>
    </div>
  ),
};

/** Free-text question for a masked answer held only by the DOM input. */
export const FreeText: Story = {
  args: {},
  render: () => (
    <div className="w-[420px]">
      <Questionnaire onSubmit={event => event.preventDefault()}>
        <QuestionnaireItem name="database-password" required>
          <QuestionnaireTitle>A password is needed</QuestionnaireTitle>
          <QuestionnaireDescription>
            The staging database asked for the atlas user&apos;s password.
          </QuestionnaireDescription>
          <QuestionnaireInput placeholder="Password for user atlas" type="password" />
          <QuestionnaireActions>
            <QuestionnaireSkip>Decline</QuestionnaireSkip>
            <QuestionnaireSubmit>Send</QuestionnaireSubmit>
          </QuestionnaireActions>
        </QuestionnaireItem>
      </Questionnaire>
    </div>
  ),
};

/** Multi-step flow showing progress, validation, and every navigation action. */
export const MultiStep: Story = {
  args: {},
  render: () => (
    <div className="w-[420px]">
      <Questionnaire onSubmit={event => event.preventDefault()} shortcuts="letters">
        <QuestionnaireProgress />
        <QuestionnaireItem name="scope" required>
          <QuestionnaireTitle>Which files should the agent touch?</QuestionnaireTitle>
          <QuestionnaireChoices>
            <QuestionnaireChoice value="web">Only web/</QuestionnaireChoice>
            <QuestionnaireChoice value="all">The whole repo</QuestionnaireChoice>
          </QuestionnaireChoices>
          <QuestionnaireError />
        </QuestionnaireItem>
        <QuestionnaireItem name="note">
          <QuestionnaireTitle>Anything else it should know?</QuestionnaireTitle>
          <QuestionnaireInput placeholder="Optional note" type="text" />
        </QuestionnaireItem>
        <QuestionnaireActions>
          <QuestionnairePrevious />
          <QuestionnaireSkip />
          <QuestionnaireNext />
          <QuestionnaireSubmit>Send</QuestionnaireSubmit>
        </QuestionnaireActions>
      </Questionnaire>
    </div>
  ),
};
