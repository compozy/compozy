import type { OsWindowRoute } from "../../lib/os-types";
import {
  validateLoopRunDiffSearch,
  validateLoopRunsSearch,
  validateLoopsSearch,
  type LoopRunDiffRouteSearch,
  type LoopRunsRouteSearch,
  type LoopsRouteSearch,
} from "@/systems/loops";

export interface LoopRequestFocus {
  generation?: number;
  nodeId: string;
  itemIndex: number;
}

/** The surface a Loops window route names; the window and its loading fallback read it alike. */
export type LoopWindowLocation =
  | { kind: "run-diff"; runId: string; search: LoopRunDiffRouteSearch }
  | { kind: "run"; runId: string; requestFocus: LoopRequestFocus | undefined }
  | { kind: "runs"; search: LoopRunsRouteSearch }
  | { kind: "configure" | "editor" | "run-form" | "detail"; name: string }
  | { kind: "catalog"; search: LoopsRouteSearch };

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function loopRequestFocus(search: OsWindowRoute["search"]): LoopRequestFocus | undefined {
  const nodeId = typeof search.request_node === "string" ? search.request_node.trim() : "";
  const itemIndex = Number(search.request_item);
  return nodeId !== "" && Number.isInteger(itemIndex) && itemIndex >= 0
    ? { nodeId, itemIndex }
    : undefined;
}

export function parseLoopWindowLocation(location: OsWindowRoute): LoopWindowLocation {
  const runDiff = /^\/loop-runs\/([^/]+)\/diff$/.exec(location.pathname);
  if (runDiff) {
    return {
      kind: "run-diff",
      runId: decodePathSegment(runDiff[1]),
      search: validateLoopRunDiffSearch(location.search),
    };
  }

  const runDetail = /^\/loop-runs\/([^/]+)$/.exec(location.pathname);
  if (runDetail) {
    return {
      kind: "run",
      runId: decodePathSegment(runDetail[1]),
      requestFocus: loopRequestFocus(location.search),
    };
  }
  if (location.pathname === "/loop-runs") {
    return { kind: "runs", search: validateLoopRunsSearch(location.search) };
  }

  const nested = /^\/loops\/([^/]+)\/(configure|editor|run)$/.exec(location.pathname);
  if (nested) {
    const name = decodePathSegment(nested[1]);
    if (nested[2] === "configure") return { kind: "configure", name };
    if (nested[2] === "editor") return { kind: "editor", name };
    return { kind: "run-form", name };
  }

  const detail = /^\/loops\/([^/]+)$/.exec(location.pathname);
  if (detail) return { kind: "detail", name: decodePathSegment(detail[1]) };
  return { kind: "catalog", search: validateLoopsSearch(location.search) };
}
