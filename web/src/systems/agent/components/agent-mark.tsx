import { AgentAvatar, type AgentAvatarProps } from "@compozy/ui";

import { PROFILE_SPRITE_URL } from "@/systems/profiles/lib/profile-identity";

/** `AgentAvatar` bound to the app's Lucide sprite: every agent mark in `web/` goes through it. */
export function AgentMark(props: Omit<AgentAvatarProps, "spriteUrl">) {
  return <AgentAvatar spriteUrl={PROFILE_SPRITE_URL} {...props} />;
}
