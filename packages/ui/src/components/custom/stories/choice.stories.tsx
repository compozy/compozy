import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../../button";
import {
  ChoiceFree,
  ChoiceFreeBar,
  ChoiceFreeTextarea,
  ChoiceHint,
  ChoiceKey,
  ChoiceList,
  ChoiceRow,
} from "../choice";
import { Dock } from "../dock";

const meta: Meta<typeof ChoiceList> = {
  title: "components/custom/Choice",
  component: ChoiceList,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Clarification answers on the decision dock: bounded questions render 30px rows with numeric shortcut chips; a question with no choices renders the quiet free-text form. Shells only — submission behavior belongs to the host.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const BoundedChoices: Story = {
  render: () => (
    <div className="max-w-xl">
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Question</Dock.Eyebrow>
          <Dock.Title>Where should the marker stories live?</Dock.Title>
          <Dock.Deadline>times out 14:32</Dock.Deadline>
        </Dock.Head>
        <ChoiceList>
          <ChoiceRow>
            <ChoiceKey>1</ChoiceKey>
            packages/ui (shared primitive)
            <ChoiceHint>recommended</ChoiceHint>
          </ChoiceRow>
          <ChoiceRow>
            <ChoiceKey>2</ChoiceKey>
            web/src/systems/session
          </ChoiceRow>
          <ChoiceRow>
            <ChoiceKey>3</ChoiceKey>
            skip stories for now
          </ChoiceRow>
        </ChoiceList>
      </Dock>
    </div>
  ),
};

export const FreeText: Story = {
  render: () => (
    <div className="max-w-xl">
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Question</Dock.Eyebrow>
          <Dock.Title>What should the release be called?</Dock.Title>
        </Dock.Head>
        <ChoiceFree>
          <ChoiceFreeTextarea placeholder="Type your answer…" />
          <ChoiceFreeBar>
            <span className="flex-1" />
            <Button size="sm">Send answer</Button>
          </ChoiceFreeBar>
        </ChoiceFree>
      </Dock>
    </div>
  ),
};
