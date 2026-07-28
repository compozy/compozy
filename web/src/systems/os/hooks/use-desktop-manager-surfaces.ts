import { useShallow } from "zustand/shallow";

import { getOsApp } from "../lib/app-registry";
import { useDesktop } from "./use-desktop";
import {
  useDesktopOverviewSegmentRequest,
  usePendingWindowManagerCommand,
  useWindowManagerConflict,
  useWindowManagerDiagnostic,
  useWindowManagerOverlay,
} from "./use-window-manager-store";

/** Runtime inputs for the desktop overview and connection diagnostic surfaces. */
export function useDesktopManagerSurfaces() {
  const overlay = useWindowManagerOverlay();
  const overviewSegmentRequest = useDesktopOverviewSegmentRequest();
  const conflict = useWindowManagerConflict();
  const diagnostic = useWindowManagerDiagnostic();
  const pending = usePendingWindowManagerCommand();
  const hydration = useDesktop(state => state.hydration);
  const connectionStatus = useDesktop(state => state.connectionStatus);
  const activeDesktopId = useDesktop(state => state.activeDesktopId);
  const canMutate = useDesktop(
    state =>
      state.client !== null && state.hydration === "live" && state.connectionStatus === "connected"
  );
  const projection = useDesktop(
    useShallow(state => ({
      desktops: state.desktops,
      projections: state.projections,
      windows: state.windows,
    }))
  );
  const windowsByDesktop = new Map<string, Array<(typeof projection.windows)[string]>>();
  for (const windowRecord of Object.values(projection.windows)) {
    const records = windowsByDesktop.get(windowRecord.desktopId);
    if (records) records.push(windowRecord);
    else windowsByDesktop.set(windowRecord.desktopId, [windowRecord]);
  }
  const desktops = projection.desktops.map(desktop => {
    const windowRecords = windowsByDesktop.get(desktop.id) ?? [];
    return {
      id: desktop.id,
      name: desktop.name,
      purpose: desktop.purpose,
      projection: projection.projections[desktop.id],
      windowRecords,
      windows: windowRecords.map(window => ({
        id: window.id,
        title: getOsApp(window.app).title,
        detail: window.instanceKey ?? undefined,
      })),
    };
  });

  return {
    activeDesktopId,
    canMutate,
    connectionStatus,
    conflict,
    desktops,
    diagnostic,
    hydration,
    overlay,
    overviewSegmentRequest,
    pending,
  };
}

export type DesktopManagerSurfacesModel = ReturnType<typeof useDesktopManagerSurfaces>;
