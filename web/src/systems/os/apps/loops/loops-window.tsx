import { useDesktop } from "../../hooks/use-desktop";
import { LoopConfigureLocation } from "./loop-configure-location";
import { LoopDetailLocation } from "./loop-detail-location";
import { LoopEditorLocation } from "./loop-editor-location";
import { LoopRunDetailLocation } from "./loop-run-detail-location";
import { LoopRunFormLocation } from "./loop-run-form-location";
import { LoopRunsLocation } from "./loop-runs-location";
import { LoopsCatalogLocation } from "./loops-catalog-location";
import { validateLoopRunsSearch } from "./use-loop-runs-route";
import { validateLoopsSearch } from "./use-loops-catalog";

const DEFAULT_LOOPS_ROUTE = { pathname: "/loops", search: {} } as const;

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** Loops and Loop runs controller driven exclusively by the logical WM location. */
export function LoopsWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_LOOPS_ROUTE);

  const runDetail = /^\/loop-runs\/([^/]+)$/.exec(location.pathname);
  if (runDetail) return <LoopRunDetailLocation runId={decodePathSegment(runDetail[1])} />;
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
  if (detail) return <LoopDetailLocation name={decodePathSegment(detail[1])} />;
  return <LoopsCatalogLocation search={validateLoopsSearch(location.search)} />;
}
