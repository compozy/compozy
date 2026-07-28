import {
  validateAgentDetailSearch,
  validateAgentSettingsSearch,
  validateAgentsFleetSearch,
} from "@/systems/agent";
import type { OsWindowRoute } from "../../lib/os-types";

export type AgentWindowLocation =
  | { kind: "catalog"; search: ReturnType<typeof validateAgentsFleetSearch> }
  | {
      kind: "detail";
      name: string;
      search: ReturnType<typeof validateAgentDetailSearch>;
    }
  | {
      kind: "settings";
      name: string;
      search: ReturnType<typeof validateAgentSettingsSearch>;
    };

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export function parseAgentWindowLocation(location: OsWindowRoute): AgentWindowLocation {
  const settingsMatch = /^\/agents\/([^/]+)\/settings$/.exec(location.pathname);
  if (settingsMatch) {
    return {
      kind: "settings",
      name: decodePathSegment(settingsMatch[1]),
      search: validateAgentSettingsSearch(location.search),
    };
  }

  const detailMatch = /^\/agents\/([^/]+)$/.exec(location.pathname);
  if (detailMatch) {
    return {
      kind: "detail",
      name: decodePathSegment(detailMatch[1]),
      search: validateAgentDetailSearch(location.search),
    };
  }

  return { kind: "catalog", search: validateAgentsFleetSearch(location.search) };
}
