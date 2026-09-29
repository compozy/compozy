import { Check, Minus, TriangleAlert } from "lucide-react";

import { Icon, Pill, type PillTone } from "@compozy/ui";

import type { SystemNotificationState } from "@/systems/os";

const SYSTEM_STATE: Record<
  SystemNotificationState,
  { label: string; icon: typeof Check; tone: PillTone }
> = {
  granted: { label: "Allowed", icon: Check, tone: "success" },
  denied: { label: "Blocked", icon: TriangleAlert, tone: "warning" },
  unsupported: { label: "Unavailable", icon: Minus, tone: "neutral" },
  default: { label: "Not allowed yet", icon: Minus, tone: "neutral" },
};

export function AttentionSystemStateChip({ state }: { state: SystemNotificationState }) {
  const chip = SYSTEM_STATE[state];
  return (
    <Pill data-testid={`settings-attention-system-${state}`} size="sm" tone={chip.tone}>
      <Icon as={chip.icon} size="xs" />
      {chip.label}
    </Pill>
  );
}
