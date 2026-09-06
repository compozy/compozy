import type { ReactNode } from "react";

import { SessionThreadLiveDataContext } from "./session-thread-live-data-context";

export function SessionThreadLiveDataProvider({
  children,
  liveDataEnabled,
}: {
  children: ReactNode;
  liveDataEnabled: boolean;
}) {
  return (
    <SessionThreadLiveDataContext value={liveDataEnabled}>{children}</SessionThreadLiveDataContext>
  );
}
