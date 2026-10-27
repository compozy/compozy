import { lazy, Suspense, type ComponentProps } from "react";

import { SessionThreadSkeleton } from "@/components/assistant-ui/session-thread-skeleton";

import { loadSessionThread } from "./session-window-module-loader";

const LazySessionThread = lazy(() =>
  loadSessionThread().then(module => ({ default: module.SessionThread }))
);

/**
 * The session thread, loaded on demand. Until its chunk lands the pane holds the transcript and
 * composer it will fill (a read-only history has no composer), so the swap moves nothing.
 */
export function SessionThread(props: ComponentProps<typeof LazySessionThread>) {
  return (
    <Suspense fallback={<SessionThreadSkeleton composer={!props.readOnly} />}>
      <LazySessionThread {...props} />
    </Suspense>
  );
}
