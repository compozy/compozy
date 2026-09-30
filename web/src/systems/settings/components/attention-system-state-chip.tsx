import { Pill, type PillTone } from "@compozy/ui";

import type { SystemNotificationState } from "@/systems/os";

const SYSTEM_STATE: Record<SystemNotificationState, { label: string; tone: PillTone }> = {
  granted: { label: "Allowed", tone: "success" },
  denied: { label: "Blocked", tone: "warning" },
  unsupported: { label: "Unavailable", tone: "neutral" },
  default: { label: "Not allowed yet", tone: "neutral" },
};

export function AttentionSystemStateChip({ state }: { state: SystemNotificationState }) {
  const chip = SYSTEM_STATE[state];
  return (
    <Pill data-testid={`settings-attention-system-${state}`} form="plain" tone={chip.tone}>
      <Pill.Dot />
      {chip.label}
    </Pill>
  );
}
