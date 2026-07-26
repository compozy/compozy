import { Input } from "@agh/ui";

import type { LoopConfigCheckDescriptor, LoopConfigCheckState } from "../../lib/loop-config-checks";
import { LoopConfigureSwitchRow } from "./loop-configure-switch-row";

interface LoopConfigureChecksProps {
  descriptors: LoopConfigCheckDescriptor[];
  states: Record<string, LoopConfigCheckState>;
  disabled?: boolean;
  onToggle: (id: string, enabled: boolean) => void;
  onCommandChange: (id: string, command: string) => void;
}

/**
 * The "Review gate" group (§4.7.1): each declared verification criterion as a switch row.
 * A `command` check is toggleable and exposes a project-command field (disabled with the
 * check); every other typed criterion is LOCKED on — removing it needs a fork (ADR-009).
 */
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
        className="rounded-lg border border-line-soft bg-canvas-tint px-3.5 py-3 text-form-hint text-subtle"
        data-testid="loop-configure-checks-empty"
      >
        This loop declares no verification checks.
      </p>
    );
  }
  return (
    <div
      className="flex flex-col overflow-hidden rounded-lg border border-line-soft bg-canvas-tint"
      data-testid="loop-configure-checks"
    >
      {descriptors.map(descriptor => {
        const state = states[descriptor.id] ?? { enabled: true, command: "" };
        const commandDisabled = disabled || !state.enabled;
        return (
          <LoopConfigureSwitchRow
            key={descriptor.id}
            testId={`loop-configure-check-${descriptor.id}`}
            typeLabel={descriptor.type}
            title={descriptor.id}
            description={descriptor.method || undefined}
            checked={state.enabled}
            disabled={descriptor.locked || disabled}
            lockedHint={descriptor.locked ? "Cannot be removed without a fork." : undefined}
            onCheckedChange={next => onToggle(descriptor.id, next)}
          >
            {descriptor.isCommand ? (
              <Input
                type="text"
                data-testid={`loop-configure-command-${descriptor.id}`}
                className="ml-[96px] h-8 font-mono text-form-input"
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
