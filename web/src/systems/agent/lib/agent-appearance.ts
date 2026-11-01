import type { AgentAvatarAppearance } from "@compozy/ui";

import type { AgentAppearancePayload } from "../types";
import { agentAvatarAppearance } from "./agent-instance";

/** The open wire object as the avatar reads it; anything malformed falls back to the slug. */
export function agentAppearanceFromPayload(
  appearance: unknown,
  name: string
): AgentAvatarAppearance {
  return agentAvatarAppearance(appearance as AgentAppearancePayload | null | undefined, name);
}
