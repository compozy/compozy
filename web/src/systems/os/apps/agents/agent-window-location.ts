import type { OsWindowRoute } from "../../lib/os-types";
import {
  AGENT_EDITOR_PAGES,
  agentSettingsDestination,
  agentTabDestination,
  resolveAgentSettingsSearch,
  validateAgentDetailSearch,
  validateAgentSettingsSearch,
  validateAgentsFleetSearch,
  type AgentEditorPage,
  type AgentLegacyDestination,
} from "@/systems/agent";

export type AgentWindowLocation =
  | { kind: "catalog"; search: ReturnType<typeof validateAgentsFleetSearch> }
  | {
      kind: "detail";
      name: string;
      search: ReturnType<typeof validateAgentDetailSearch>;
    }
  | {
      kind: "page";
      name: string;
      page: AgentEditorPage;
      search: ReturnType<typeof validateAgentDetailSearch>;
    };

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function fromLegacy(
  name: string,
  destination: AgentLegacyDestination,
  search: { workspace?: string }
): AgentWindowLocation {
  const binding = search.workspace ? { workspace: search.workspace } : {};
  if (destination.kind === "page") {
    return { kind: "page", name, page: destination.page, search: binding };
  }
  return {
    kind: "detail",
    name,
    search: { ...binding, ...(destination.panel ? { panel: destination.panel } : {}) },
  };
}

export function parseAgentWindowLocation(location: OsWindowRoute): AgentWindowLocation {
  const settingsMatch = /^\/agents\/([^/]+)\/settings$/.exec(location.pathname);
  if (settingsMatch) {
    const search = validateAgentSettingsSearch(location.search);
    return fromLegacy(
      decodePathSegment(settingsMatch[1]),
      agentSettingsDestination(resolveAgentSettingsSearch(search).section),
      search
    );
  }

  const pageMatch = /^\/agents\/([^/]+)\/([^/]+)$/.exec(location.pathname);
  const page = AGENT_EDITOR_PAGES.find(item => item === pageMatch?.[2]);
  if (pageMatch && page) {
    return {
      kind: "page",
      name: decodePathSegment(pageMatch[1]),
      page,
      search: validateAgentDetailSearch(location.search),
    };
  }

  const detailMatch = /^\/agents\/([^/]+)$/.exec(location.pathname);
  if (detailMatch) {
    const name = decodePathSegment(detailMatch[1]);
    const search = validateAgentDetailSearch(location.search);
    const legacy = agentTabDestination((location.search as { tab?: unknown }).tab);
    return legacy.kind === "page" || legacy.panel
      ? fromLegacy(name, legacy, search)
      : { kind: "detail", name, search };
  }

  return { kind: "catalog", search: validateAgentsFleetSearch(location.search) };
}
