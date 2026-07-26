import { Network as NetworkIcon } from "lucide-react";

import { Button, Empty } from "@agh/ui";

export interface NetworkEmptyProps {
  /** Settings deep-link handler from the parent route. */
  onOpenSettings?: () => void;
  /** When true, Network is administratively disabled; name who can change it. */
  disabledByAdmin?: boolean;
  className?: string;
}

/**
 * Oriented empty for Network discovery (UT-058):
 * where am I, what can I do, who changes this, what is the next action.
 */
export function NetworkEmpty({
  onOpenSettings,
  disabledByAdmin = false,
  className,
}: NetworkEmptyProps) {
  const title = disabledByAdmin ? "Network is disabled." : "Network is ready when you are.";
  const description = disabledByAdmin
    ? "An operator with admin access can enable Network availability. Local work remains available, and existing Network channels and history stay intact."
    : "Executions stay Local by default. Choose Live explicitly for a future run to coordinate in channels, or open Settings to review Live defaults and ceilings. Network availability never enrolls an execution.";

  return (
    <Empty
      action={
        onOpenSettings ? (
          <Button
            data-testid="network-empty-open-settings"
            onClick={onOpenSettings}
            size="default"
            type="button"
            variant="neutral"
          >
            Open Network settings
          </Button>
        ) : null
      }
      className={className}
      data-testid="network-empty"
      description={description}
      icon={NetworkIcon}
      title={title}
    />
  );
}
