import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import type { SkillExposeModel } from "../../hooks/use-skill-expose";
import { skillExposeResultViews, toSkillExposureView } from "../../lib/skill-exposure-view";
import { skillExposePartialFailureFixture, skillExposuresFixture } from "../../mocks";
import type { SkillExposeResult } from "../../types";
import { SkillExposePanel } from "../skill-expose-panel";
import type { SkillExposeTarget } from "../skill-expose-target-picker";

const noop = () => undefined;

const TARGETS: SkillExposeTarget[] = [
  { slug: "agents", label: "Universal (.agents)", hint: ".agents/skills" },
  { slug: "claude", label: "Claude (.claude)", hint: ".claude/skills" },
];

function exposeModel(overrides: Partial<SkillExposeModel> = {}): SkillExposeModel {
  return {
    pendingTargets: [],
    isPending: false,
    results: [],
    failure: null,
    rolledBack: false,
    expose: noop,
    unexpose: noop,
    dismiss: noop,
    ...overrides,
  };
}

const labelForTarget = (slug: string) =>
  TARGETS.find(target => target.slug === slug)?.label ?? slug;

const meta = {
  title: "systems/skill/components/SkillExposePanel",
  component: SkillExposePanel,
  parameters: { layout: "padded" },
  args: { labelForTarget, targets: TARGETS, targetsError: null, targetsLoading: false },
  render: args => (
    <PanelSurface className="w-full max-w-md p-4">
      <SkillExposePanel {...args} />
    </PanelSurface>
  ),
} satisfies Meta<typeof SkillExposePanel>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Not exposed anywhere: the action, and nothing else. */
export const NotExposed: Story = { args: { exposures: [], model: exposeModel() } };

export const Healthy: Story = {
  args: {
    exposures: [toSkillExposureView(skillExposuresFixture[0])],
    model: exposeModel(),
  },
};

/**
 * Links we created that stopped working get a repair action; a file another app
 * put at the same path gets a sentence and nothing else.
 */
export const BrokenLinks: Story = {
  args: {
    exposures: skillExposuresFixture.slice(1, 3).map(toSkillExposureView),
    model: exposeModel(),
  },
};

export const ForeignConflict: Story = {
  args: {
    exposures: [toSkillExposureView(skillExposuresFixture[3])],
    model: exposeModel(),
  },
};

export const Exposing: Story = {
  args: {
    exposures: [],
    model: exposeModel({ pendingTargets: ["agents"], isPending: true }),
  },
};

/** One target refused, the completed one undone — every target accounted for. */
export const PartialFailure: Story = {
  args: {
    exposures: [],
    model: exposeModel({
      failure: skillExposePartialFailureFixture.error.message,
      rolledBack: true,
      results: skillExposeResultViews(
        skillExposePartialFailureFixture.results as SkillExposeResult[]
      ),
    }),
  },
};

/** No source is turned on, so there is no target to offer. */
export const NoEnabledTargets: Story = {
  args: { exposures: [], model: exposeModel(), targets: [] },
};

export const TargetsUnavailable: Story = {
  args: {
    exposures: [],
    model: exposeModel(),
    onRetryTargets: noop,
    targets: [],
    targetsError: "Source policy unavailable",
  },
};
