import { RestartBanner, cn } from "@compozy/ui";

import { useDaemonRecovery } from "../hooks/use-daemon-recovery";
import type { DaemonRecovery } from "../lib/daemon-readiness";

export interface DaemonRecoveryNoticeProps {
  recovery: DaemonRecovery;
  className?: string;
}

/**
 * Says the runtime is back and still resuming sessions after a restart: reading works, sending
 * opens when recovery ends. Each resuming session keeps its own notice in its window.
 */
export function DaemonRecoveryNotice({ recovery, className }: DaemonRecoveryNoticeProps) {
  return (
    <RestartBanner
      busy
      className={cn("rounded-lg border-0 px-3.5 py-3 md:px-3.5", className)}
      data-testid="daemon-recovery-notice"
      description="You can read your sessions now. Sending opens again when this finishes."
      message={recovery.label}
      tone="info"
    />
  );
}

/** Mounts the notice only while the runtime reports session recovery. */
export function DaemonRecoveryNoticeHost({ className }: { className?: string }) {
  const recovery = useDaemonRecovery();
  if (recovery === null) return null;
  return <DaemonRecoveryNotice className={className} recovery={recovery} />;
}
