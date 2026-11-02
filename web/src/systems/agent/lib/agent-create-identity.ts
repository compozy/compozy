import type { AgentAvatarAppearance } from "@compozy/ui";

import type { CreateAgentParams } from "../types";

interface AgentNameFields {
  displayName: string;
  name: string;
}

/** The slug a name reads as: lowercase words joined by hyphens, starting with a letter. */
export function agentSlugFor(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^[^a-z]+|-+$/g, "");
}

/** The id the agent is created with: the Short name, or one derived from the Name. */
export function agentCreateSlug(draft: AgentNameFields) {
  return draft.name.trim() || agentSlugFor(draft.displayName);
}

const FACE_SEED_PLACEHOLDER = "new-agent";

/** The face the screen shows, which is also the one saved: it follows the slug until picked. */
export function agentCreateAppearance(
  draft: AgentNameFields & { appearance: AgentAvatarAppearance | null }
): AgentAvatarAppearance {
  return (
    draft.appearance ?? {
      avatar: "generated",
      seed: agentCreateSlug(draft) || FACE_SEED_PLACEHOLDER,
      generation: 1,
    }
  );
}

/** The saved block: the daemon's contract asks every appearance for a version and a color. */
export function appearancePayload(
  appearance: AgentAvatarAppearance,
  colors: readonly string[]
): NonNullable<CreateAgentParams["agent"]["appearance"]> {
  const color = appearance.color ?? autoAvatarColor(appearance, colors);
  return { version: 1, ...appearance, color };
}

/** Auto keeps a seeded color rather than one shared default, so new agents stay distinct. */
export function autoAvatarColor(appearance: AgentAvatarAppearance, colors: readonly string[]) {
  const key = appearance.avatar === "generated" ? appearance.seed : appearance.avatar;
  const hash = Array.from(key).reduce((sum, char) => (sum * 31 + char.charCodeAt(0)) >>> 0, 7);
  return colors[hash % colors.length] ?? "#7c5cff";
}
