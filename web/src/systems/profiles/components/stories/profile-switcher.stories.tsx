import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import { toProfileRows } from "../../lib/profile-rows";
import {
  consultingProfileFixture,
  defaultProfileFixture,
  growthProfileFixture,
  marketingProfileFixture,
} from "../../mocks/fixtures";
import { ProfileSwitcher } from "../profile-switcher";

const PLURAL = toProfileRows(
  [defaultProfileFixture, marketingProfileFixture, consultingProfileFixture, growthProfileFixture],
  "marketing"
);

const meta: Meta<typeof ProfileSwitcher> = {
  title: "systems/profiles/components/ProfileSwitcher",
  component: ProfileSwitcher,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The menubar profile switcher. Quiet until a second profile exists; once plural it carries the identity and answers the boundary question in one sentence.",
      },
    },
  },
  args: {
    onSelectProfile: fn(),
    onSelectAggregate: fn(),
    onCreate: fn(),
    onOpenSettings: fn(),
  },
  decorators: [
    Story => (
      <StorySurface>
        <div className="flex justify-end">
          <Story />
        </div>
      </StorySurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Only `default` exists: a neutral icon button, no ceremony anywhere. */
export const Quiet: Story = {
  args: {
    rows: toProfileRows([defaultProfileFixture], "default"),
    activeName: "default",
    aggregate: false,
    quiet: true,
    archivedCount: 0,
  },
};

/** Plural: glyph plus name, with the archived count demoted to a Settings link. */
export const Plural: Story = {
  args: {
    rows: PLURAL,
    activeName: "marketing",
    aggregate: false,
    quiet: false,
    archivedCount: 2,
  },
};

/** The aggregate is a way of looking, so it renders the neutral layered mark. */
export const AllProfiles: Story = {
  args: {
    rows: PLURAL,
    activeName: "marketing",
    aggregate: true,
    quiet: false,
    archivedCount: 2,
  },
};
