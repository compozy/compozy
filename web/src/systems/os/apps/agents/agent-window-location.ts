import type { OsWindowRoute } from "../../lib/os-types";
import {
  validateAgentDetailSearch,
  validateAgentSettingsSearch,
  validateAgentsFleetSearch,
} from "@/systems/agent";

export type AgentWindowLocation =
  | { kind: "catalog"; search: ReturnType<typeof validateAgentsFleetSearch> }
  /** Delegation trees across the workspace, live. */
  | { kind: "activity" }
  /** One call's record. */
  | { kind: "call"; callId: string }
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
  // Order is load-bearing. `/agents/activity` and `/agents/calls/<id>` are static
  // routes that the `/agents/<name>` detail pattern would otherwise swallow —
  // opening Activity would render a detail page for an agent named "activity".
  // Both must match before the dynamic segment gets a look.
  if (location.pathname === "/agents/activity") {
    return { kind: "activity" };
  }

  const callMatch = /^\/agents\/calls\/([^/]+)$/.exec(location.pathname);
  if (callMatch) {
    return { kind: "call", callId: decodePathSegment(callMatch[1]) };
  }

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
