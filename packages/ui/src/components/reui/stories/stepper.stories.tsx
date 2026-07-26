import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Button } from "../../button";
import {
  Stepper,
  StepperBody,
  StepperContent,
  StepperDescription,
  StepperIndicator,
  StepperItem,
  StepperNav,
  StepperPanel,
  StepperRail,
  StepperSeparator,
  StepperTitle,
  StepperTrigger,
} from "../stepper";

const STEPS = [
  { step: 1, title: "Runtime", description: "Provider, model and how the agent signs in." },
  { step: 2, title: "Workspace", description: "The folders the agent is allowed to open." },
  { step: 3, title: "Review", description: "Confirm before anything is written." },
] as const;

const LAST = STEPS.length;

/**
 * Horizontal: the separator is a sibling of the trigger and fills the gap
 * between indicators. `StepperRail` is vertical-only, so a horizontal nav puts
 * the indicator directly inside the trigger.
 */
function HorizontalStepper() {
  const [value, setValue] = useState(2);

  return (
    <div className="w-[34rem]">
      <Stepper value={value} onValueChange={setValue}>
        <StepperNav>
          {STEPS.map(item => (
            <StepperItem key={item.step} step={item.step}>
              <StepperTrigger>
                <StepperIndicator>{item.step}</StepperIndicator>
                <StepperTitle>{item.title}</StepperTitle>
              </StepperTrigger>
              {item.step < LAST ? <StepperSeparator /> : null}
            </StepperItem>
          ))}
        </StepperNav>
        <StepperPanel className="mt-6">
          {STEPS.map(item => (
            <StepperContent key={item.step} value={item.step}>
              <p className="text-small-body text-muted">{item.description}</p>
            </StepperContent>
          ))}
        </StepperPanel>
      </Stepper>
      <div className="mt-6 flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={value === 1}
          onClick={() => setValue(current => Math.max(1, current - 1))}
        >
          Back
        </Button>
        <Button
          size="sm"
          disabled={value === LAST}
          onClick={() => setValue(current => Math.min(LAST, current + 1))}
        >
          Continue
        </Button>
      </div>
    </div>
  );
}

/** Vertical: `StepperRail` stacks the indicator over the connecting separator. */
function VerticalStepper({ activeStep, maxStep }: { activeStep: number; maxStep: number }) {
  return (
    <div className="w-80">
      <Stepper value={activeStep} orientation="vertical">
        <StepperNav>
          {STEPS.map(item => (
            <StepperItem key={item.step} step={item.step} disabled={item.step > maxStep}>
              <StepperTrigger>
                <StepperRail>
                  <StepperIndicator>{item.step}</StepperIndicator>
                  {item.step < LAST ? <StepperSeparator /> : null}
                </StepperRail>
                <StepperBody>
                  <StepperTitle>{item.title}</StepperTitle>
                  <StepperDescription>{item.description}</StepperDescription>
                </StepperBody>
              </StepperTrigger>
            </StepperItem>
          ))}
        </StepperNav>
      </Stepper>
    </div>
  );
}

const meta: Meta = {
  title: "components/reui/Stepper",
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Multi-step progress with an indicator rail, per-step body copy and panel content. A step before the active one renders completed; `disabled` locks a step the flow has not reached yet.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Rail across the top — short labels, width available, panel below. */
export const Horizontal: Story = {
  render: () => <HorizontalStepper />,
};

/** Rail down the side — each step carries a sentence of context. */
export const Vertical: Story = {
  render: () => <VerticalStepper activeStep={2} maxStep={3} />,
};

/** Steps beyond the flow's reach render disabled rather than absent. */
export const LockedAhead: Story = {
  render: () => <VerticalStepper activeStep={1} maxStep={1} />,
};
