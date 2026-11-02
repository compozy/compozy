import type { AgentAvatarAppearance } from "@compozy/ui";

import type { AgentAppearancePayload, AgentPayload } from "../types";
import { agentAvatarAppearance } from "./agent-instance";

/** What a surface needs to speak about an agent; `name` stays the id for routes and test ids. */
export interface AgentIdentity {
  name: string;
  label: string;
  appearance: AgentAvatarAppearance;
  /** False for a deleted agent, an old payload, or a catalog that hasn't loaded. */
  known: boolean;
  /** The catalog answered and doesn't hold this name: a deleted agent, not a slow read. */
  missing: boolean;
}

export type AgentIdentitySource = Pick<AgentPayload, "name" | "label" | "resolved_appearance">;

/** `AgentDef.Label` in `internal/config` for an agent with no display name; keep them in step. */
export function humanizeAgentSlug(slug: string): string {
  return slug
    .split(/[-_.\s]+/u)
    .filter(Boolean)
    .map(word => {
      const [first = "", ...rest] = Array.from(word);
      return first.toUpperCase() + rest.join("");
    })
    .join(" ");
}

const INDEXES = new WeakMap<readonly AgentIdentitySource[], Map<string, AgentIdentitySource>>();

/** Every row of a list asks for one catalog, so the lookup is built once per catalog read. */
function agentIndex(agents: readonly AgentIdentitySource[]) {
  let index = INDEXES.get(agents);
  if (!index) {
    index = new Map(agents.map(agent => [agent.name, agent]));
    INDEXES.set(agents, index);
  }
  return index;
}

function identityOf(name: string, agent: AgentIdentitySource | undefined, loaded: boolean) {
  return {
    name,
    label: agent?.label?.trim() || humanizeAgentSlug(name),
    appearance: agentAvatarAppearance(
      agent?.resolved_appearance as AgentAppearancePayload | null | undefined,
      name
    ),
    known: agent !== undefined,
    missing: loaded && agent === undefined,
  } satisfies AgentIdentity;
}

/** The identity of a definition already in hand, e.g. a picker's row. */
export function agentIdentityOf(agent: AgentIdentitySource): AgentIdentity {
  return identityOf(agent.name, agent, true);
}

/** A name the catalog doesn't hold still reads as a person's name, on its slug-derived face. */
export function resolveAgentIdentity(
  name: string,
  agents?: readonly AgentIdentitySource[] | null
): AgentIdentity {
  return identityOf(name, agents ? agentIndex(agents).get(name) : undefined, Boolean(agents));
}
