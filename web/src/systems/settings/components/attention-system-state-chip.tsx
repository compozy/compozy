import { Check, Minus, TriangleAlert } from "lucide-react";

import { Icon } from "@compozy/ui";

import type { SystemNotificationState } from "@/systems/os";

const SYSTEM_STATE: Record<
  SystemNotificationState,
  { label: string; icon: typeof Check; className: string }
> = {
  granted: {
    label: "Armed",
    icon: Check,
    className: "bg-success-tint text-success",
  },
  denied: {
    label: "Denied",
    icon: TriangleAlert,
    className: "bg-warning-tint text-warning",
  },
  unsupported: {
    label: "Unavailable",
    icon: Minus,
    className: "bg-neutral-tint text-muted",
  },
  default: {
    label: "Not armed",
    icon: Minus,
    className: "bg-neutral-tint text-muted",
  },
};

export function AttentionSystemStateChip({ state }: { state: SystemNotificationState }) {
  const chip = SYSTEM_STATE[state];
  return (
    <span
      data-testid={`settings-attention-system-${state}`}
      className={`inline-flex h-5 items-center gap-1.5 rounded-full px-2 text-micro font-semibold ${chip.className}`}
    >
      <Icon as={chip.icon} size="xs" />
      {chip.label}
    </span>
  );
}
