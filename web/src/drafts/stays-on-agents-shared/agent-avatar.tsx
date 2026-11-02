/** The drafts' `AgentAvatar`: the `@compozy/ui` primitive, with the prop the catalog holds still. */
import { MotionConfig } from "motion/react";

import { AgentAvatar as UiAgentAvatar, type AgentAvatarProps } from "@compozy/ui";

import { PROFILE_SPRITE_URL } from "@/systems/profiles/lib/profile-identity";

interface DraftAgentAvatarProps extends AgentAvatarProps {
  /** Holds the working pose still, the way reduced motion does. */
  still?: boolean;
}

function DraftAgentAvatar({ still = false, ...props }: DraftAgentAvatarProps) {
  const mark = <UiAgentAvatar spriteUrl={PROFILE_SPRITE_URL} {...props} />;
  return still ? <MotionConfig reducedMotion="always">{mark}</MotionConfig> : mark;
}

export { DraftAgentAvatar as AgentAvatar };
