import { createContext } from "react";

import type { AgentIdentitySource } from "./agent-identity";

export interface AgentIdentityScope {
  /** The agent's workspace; `null` reads the user layer, absent reads the selected project. */
  workspace?: string | null;
  /** The owning profile when the surface knows it, e.g. a session row's. */
  profile?: string | null;
}

type AgentIdentityCatalogHook = (
  scope: AgentIdentityScope
) => readonly AgentIdentitySource[] | undefined;

const noAgentCatalog: AgentIdentityCatalogHook = () => undefined;

/**
 * The hook that reads the agent catalog. Views stay network-free: alone they read humanized slugs
 * and slug-derived faces, and `AgentIdentityProvider` at the app root gives them the cached catalog.
 */
export const AgentIdentityCatalogContext = createContext<AgentIdentityCatalogHook>(noAgentCatalog);
