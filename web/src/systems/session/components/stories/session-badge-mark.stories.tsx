import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import {
  SESSION_BADGE_SIGNAL,
  SESSION_BADGES,
  SessionBadgeGlyph,
  SessionBadgeMark,
  sessionBadgeWordClass,
} from "@/systems/session";

/**
 * The one badge dictionary, rendered at both of its scales with the shared
 * state glyphs. Badges that share a glyph speak their word wherever the mark
 * shows, so no state is ever carried by colour alone.
 */
function BadgeDictionary() {
  return (
    <div className="flex w-140 flex-col">
      {SESSION_BADGES.map(badge => {
        const signal = SESSION_BADGE_SIGNAL[badge];
        return (
          <div
            key={badge}
            className="grid grid-cols-[--spacing(6)_--spacing(6)_10rem_minmax(0,1fr)] items-center gap-3 border-t border-line-soft py-2 first:border-t-0"
          >
            <span className="grid place-items-center">
              <SessionBadgeMark badge={badge} />
            </span>
            <SessionBadgeGlyph badge={badge} />
            <span className={`font-mono text-micro ${sessionBadgeWordClass(badge)}`}>{badge}</span>
            <span className="text-small-body text-subtle">
              {signal.attention === "needs-you"
                ? "needs you"
                : signal.attention === "finished"
                  ? "finished, clears on focus"
                  : "no attention"}
              {signal.speaks ? ` · says "${signal.displayLabel}"` : null}
            </span>
          </div>
        );
      })}
    </div>
  );
}

const meta: Meta<typeof BadgeDictionary> = {
  title: "systems/session/components/SessionBadgeMark",
  component: BadgeDictionary,
  parameters: {
    docs: {
      description: {
        component:
          "The exported badge dictionary at both scales (12 px row marks, 18 px footprints), drawn with StateGlyph. Needs-you asks share the orange attention dot and failures share the danger ring, so those badges always speak their state word; `done` never counts toward needs-you; `unknown` names itself instead of resting on the idle dot, so the shell never fakes liveness.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof BadgeDictionary>;

/** All eleven badges — the visual contract for the dictionary. */
export const AllStates: Story = {};
