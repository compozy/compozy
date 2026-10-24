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
import { parseLoopWindowLocation } from "./loop-window-location";

const DEFAULT_LOOPS_ROUTE = { pathname: "/loops", search: {} } as const;
const LoopEditorLocation = lazy(() =>
  import("./loop-editor-location").then(module => ({ default: module.LoopEditorLocation }))
);

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
  const parsed = parseLoopWindowLocation(location);
  switch (parsed.kind) {
    case "run-diff":
      return <LoopRunDiffLocation runId={parsed.runId} search={parsed.search} />;
    case "run":
      return <LoopRunDetailLocation requestFocus={parsed.requestFocus} runId={parsed.runId} />;
    case "runs":
      return <LoopRunsLocation search={parsed.search} />;
    case "configure":
      return <LoopConfigureLocation name={parsed.name} />;
    case "editor":
      return <LoopEditorLocation name={parsed.name} />;
    case "run-form":
      return <LoopRunFormLocation name={parsed.name} />;
    case "detail":
      return <LoopDetailLocation name={parsed.name} />;
    case "catalog":
      return <LoopsCatalogLocation search={parsed.search} />;
  }
}
