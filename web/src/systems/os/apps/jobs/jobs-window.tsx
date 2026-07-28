import { useDesktop } from "../../hooks/use-desktop";
import { validateJobsSearch } from "../automation/use-automation-page";
import { JobDetailLocation } from "./job-detail-location";
import { JobsCatalogLocation } from "./jobs-catalog-location";

const DEFAULT_JOBS_ROUTE = { pathname: "/jobs", search: {} } as const;

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** Jobs app controller driven exclusively by the logical window's WM location. */
export function JobsWindow({ windowId }: { windowId: string }) {
  const location = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_JOBS_ROUTE);
  const detail = /^\/jobs\/([^/]+)$/.exec(location.pathname);
  if (detail) return <JobDetailLocation jobId={decodePathSegment(detail[1])} />;
  return <JobsCatalogLocation search={validateJobsSearch(location.search)} />;
}
