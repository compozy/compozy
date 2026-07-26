import type { AgentCatalogItem, AgentPayload } from "../types";
import { AGENT_CATEGORY_LABEL_SEPARATOR, formatCategoryLabel } from "./agent-category";
import type { AgentFleetStatus } from "./fleet-signals";

const META_SEPARATOR = " · ";
const CATEGORY_ELLIPSIS_LIMIT = 40;

export interface AgentFleetRowModel {
  agent: AgentPayload;
  signals: AgentFleetSessionSignals | null;
  meta: string;
  cardCategory: string | null;
  cardOrigin: string;
  ariaLabel: string;
  hasDiagnostics: boolean;
  sessionsAvailable: boolean;
}

export interface AgentFleetSessionSignals {
  status: AgentFleetStatus;
  active: number;
  total: number;
}

export function formatAgentOriginLabel(origin: AgentPayload["origin"]): string {
  return origin === "workspace" ? "Workspace" : "Global";
}

/** Middle-truncate long category joins while preserving first and last segments. */
export function formatCategoryMetaSegment(path: string[] | null | undefined): string {
  const label = formatCategoryLabel(path);
  if (label.length <= CATEGORY_ELLIPSIS_LIMIT) return label;
  if (!Array.isArray(path) || path.length <= 2) {
    const head = Math.max(8, Math.floor((CATEGORY_ELLIPSIS_LIMIT - 1) / 2));
    const tail = CATEGORY_ELLIPSIS_LIMIT - 1 - head;
    return `${label.slice(0, head)}…${label.slice(-tail)}`;
  }
  const sep = `${AGENT_CATEGORY_LABEL_SEPARATOR}…${AGENT_CATEGORY_LABEL_SEPARATOR}`;
  const budget = CATEGORY_ELLIPSIS_LIMIT - sep.length;
  let first = path[0] ?? "";
  let last = path[path.length - 1] ?? "";
  if (first.length + last.length > budget) {
    const headLen = Math.max(1, Math.floor(budget / 2));
    const tailLen = Math.max(1, budget - headLen);
    first = first.slice(0, headLen);
    last = last.slice(-tailLen);
  }
  return `${first}${sep}${last}`;
}

/** Row meta facts: category · provider · model (origin is a Name pill, not meta). */
export function formatAgentFleetMeta(agent: AgentPayload): string {
  const segments: string[] = [];
  const category = formatCategoryMetaSegment(agent.category_path);
  if (category) segments.push(category);
  if (agent.provider) segments.push(agent.provider);
  if (agent.model) segments.push(agent.model);
  return segments.join(META_SEPARATOR);
}

/** Card structural category (or provider fallback when category is empty). */
export function formatAgentFleetCardCategory(agent: AgentPayload): string | null {
  const category = formatCategoryMetaSegment(agent.category_path);
  if (category) return category;
  if (agent.provider) return agent.provider;
  return null;
}

export function formatAgentFleetAriaLabel(
  agent: AgentPayload,
  signals: AgentFleetSessionSignals | null,
  sessionsAvailable: boolean
): string {
  if (!sessionsAvailable || signals === null) {
    return `${agent.name}, session status unavailable`;
  }
  const statusLabel = signals.status === "active" ? "Active" : "Idle";
  return `${agent.name}, ${statusLabel}, ${signals.active} of ${signals.total} sessions active`;
}

export function projectAgentFleetRows(input: {
  items: readonly AgentCatalogItem[];
  sessionsAvailable: boolean;
}): AgentFleetRowModel[] {
  return input.items.map(item => {
    const agent = item.agent;
    const signals =
      input.sessionsAvailable && item.sessions
        ? {
            status: item.sessions.active > 0 ? ("active" as const) : ("idle" as const),
            active: item.sessions.active,
            total: item.sessions.total,
          }
        : null;
    return {
      agent,
      signals,
      meta: formatAgentFleetMeta(agent),
      cardCategory: formatAgentFleetCardCategory(agent),
      cardOrigin: formatAgentOriginLabel(agent.origin),
      ariaLabel: formatAgentFleetAriaLabel(agent, signals, input.sessionsAvailable),
      hasDiagnostics: Array.isArray(agent.diagnostics) && agent.diagnostics.length > 0,
      sessionsAvailable: input.sessionsAvailable,
    };
  });
}
