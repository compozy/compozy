import { lazy } from "react";

import { useDesktop } from "../../hooks/use-desktop";
import { sameOsWindowRoute } from "../../lib/window-manager-route";
import type { OsWindowRoute } from "../../lib/os-types";
import { LoopConfigureLocation } from "./loop-configure-location";
import { LoopDetailLocation } from "./loop-detail-location";
import { LoopRunDetailLocation } from "./loop-run-detail-location";
import { LoopRunDiffLocation } from "./loop-run-diff-location";
import { LoopRunFormLocation } from "./loop-run-form-location";
import { LoopRunsLocation } from "./loop-runs-location";
import { LoopsCatalogLocation } from "./loops-catalog-location";
import {
  validateLoopRunDiffSearch,
  validateLoopRunsSearch,
  validateLoopsSearch,
} from "@/systems/loops";

const DEFAULT_LOOPS_ROUTE = { pathname: "/loops", search: {} } as const;
const LoopEditorLocation = lazy(() =>
  import("./loop-editor-location").then(module => ({ default: module.LoopEditorLocation }))
);

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/**
 * Loops and Loop runs controller driven exclusively by the logical WM location.
 * Every location reads its data scope from the window (`useWindowDataWorkspace`):
 * a serialized `workspace` binding wins, so opening a bound Loop never changes
 * the shell's destination.
 */
export function LoopsWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(
    (state): OsWindowRoute => state.windows[windowId]?.route ?? DEFAULT_LOOPS_ROUTE,
    (left, right) => left !== undefined && sameOsWindowRoute(left, right)
  );
  return <LoopLocation location={location} />;
}

function LoopLocation({ location }: { location: OsWindowRoute }) {
  const runDiff = /^\/loop-runs\/([^/]+)\/diff$/.exec(location.pathname);
  if (runDiff) {
    return (
      <LoopRunDiffLocation
        runId={decodePathSegment(runDiff[1])}
        search={validateLoopRunDiffSearch(location.search)}
      />
    );
  }

  const runDetail = /^\/loop-runs\/([^/]+)$/.exec(location.pathname);
  if (runDetail) {
    return (
      <LoopRunDetailLocation
        runId={decodePathSegment(runDetail[1])}
        requestFocus={loopRequestFocus(location.search)}
      />
    );
  }
  if (location.pathname === "/loop-runs") {
    return <LoopRunsLocation search={validateLoopRunsSearch(location.search)} />;
  }

  const nested = /^\/loops\/([^/]+)\/(configure|editor|run)$/.exec(location.pathname);
  if (nested) {
    const name = decodePathSegment(nested[1]);
    if (nested[2] === "configure") return <LoopConfigureLocation name={name} />;
    if (nested[2] === "editor") return <LoopEditorLocation name={name} />;
    return <LoopRunFormLocation name={name} />;
  }

  const detail = /^\/loops\/([^/]+)$/.exec(location.pathname);
  if (detail) {
    return <LoopDetailLocation name={decodePathSegment(detail[1])} />;
  }
  return <LoopsCatalogLocation search={validateLoopsSearch(location.search)} />;
}

function loopRequestFocus(search: OsWindowRoute["search"]) {
  const nodeId = typeof search.request_node === "string" ? search.request_node.trim() : "";
  const itemIndex = Number(search.request_item);
  return nodeId !== "" && Number.isInteger(itemIndex) && itemIndex >= 0
    ? { nodeId, itemIndex }
    : undefined;
}
