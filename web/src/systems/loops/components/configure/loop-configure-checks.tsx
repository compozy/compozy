import { Input } from "@compozy/ui";

import { humanizeLoopNodeId } from "../../lib/loop-node-labels";
import type { LoopConfigCheckDescriptor, LoopConfigCheckState } from "../../lib/loop-config-checks";
import { LoopConfigureSwitchRow } from "./loop-configure-switch-row";

interface LoopConfigureChecksProps {
  descriptors: LoopConfigCheckDescriptor[];
  states: Record<string, LoopConfigCheckState>;
  disabled?: boolean;
  onToggle: (id: string, enabled: boolean) => void;
  onCommandChange: (id: string, command: string) => void;
}

export function LoopConfigureChecks({
  descriptors,
  states,
  disabled = false,
  onToggle,
  onCommandChange,
}: LoopConfigureChecksProps) {
  if (descriptors.length === 0) {
    return (
      <p
        className="rounded-lg bg-sunken px-4 py-3 text-form-hint text-subtle"
        data-testid="loop-configure-checks-empty"
      >
        This Loop has no checks to turn on or off.
      </p>
    );
  }
  return (
    <div
      className="flex flex-col overflow-hidden rounded-lg bg-sunken"
      data-testid="loop-configure-checks"
    >
      {descriptors.map(descriptor => {
        const state = states[descriptor.id] ?? { enabled: true, command: "" };
        const commandDisabled = disabled || !state.enabled;
        return (
          <LoopConfigureSwitchRow
            key={descriptor.id}
            testId={`loop-configure-check-${descriptor.id}`}
            title={humanizeLoopNodeId(descriptor.id)}
            description={descriptor.method || undefined}
            checked={state.enabled}
            disabled={descriptor.locked || disabled}
            lockedHint={
              descriptor.locked ? "Built into this Loop. Edit its steps to remove it." : undefined
            }
            onCheckedChange={next => onToggle(descriptor.id, next)}
          >
            {descriptor.isCommand ? (
              <Input
                type="text"
                data-testid={`loop-configure-command-${descriptor.id}`}
                className="font-mono text-form-input"
                placeholder={descriptor.declaredCommand || "command"}
                value={state.command}
                disabled={commandDisabled}
                onChange={event => onCommandChange(descriptor.id, event.target.value)}
                aria-label={`${descriptor.id} command`}
              />
            ) : null}
          </LoopConfigureSwitchRow>
        );
      })}
    </div>
  );
}
