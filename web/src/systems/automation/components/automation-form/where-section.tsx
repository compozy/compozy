import { Field, FieldDescription, FieldLabel } from "@compozy/ui";

import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { findComputerById, RunOnPicker, useComputers } from "@/systems/computers";

import { AutomationFormSection } from "./automation-form-section";

interface WhereSectionProps {
  computerId: ComputerId | undefined;
  mode: "create" | "edit";
  number: number;
  onComputerChange: (next: ComputerId) => void;
  timeZone?: string;
}

/**
 * Scheduled-job ownership selector (`_uiux.md` U4, UT-094).
 *
 * Jobs belong to one concrete computer and run there even when the controller
 * is closed — `Auto` is intentionally omitted (`allowAuto={false}`). Hidden when
 * only this computer exists.
 */
export function WhereSection({
  computerId,
  mode,
  number,
  onComputerChange,
  timeZone,
}: WhereSectionProps) {
  const computers = useComputers();
  const scopedComputerId = normalizeComputerId(computerId);
  const hasRemote = computers.computerViews.some(computer => computer.enabled);
  const show = hasRemote || !computers.localView.localRunAgents;

  if (!show) return null;

  const selectedRemote = findComputerById(computers.computers, scopedComputerId);
  const ownerName = isRemoteComputerId(scopedComputerId)
    ? (selectedRemote?.name ?? scopedComputerId)
    : "this computer";

  return (
    <AutomationFormSection
      data-testid="automation-form-where"
      description="Choose which computer stores and schedules this job"
      number={number}
      title="Where it runs"
    >
      <Field>
        <FieldLabel htmlFor="automation-run-on">Run on</FieldLabel>
        <RunOnPicker
          allowAuto={false}
          computers={computers.computerViews}
          hideWhenSingleComputer
          loadBalancingEnabled={false}
          localComputer={computers.localView}
          locked={mode === "edit"}
          onChange={next => {
            if (next !== "auto") onComputerChange(next);
          }}
          testId="automation-run-on-picker"
          triggerId="automation-run-on"
          value={scopedComputerId}
        />
        <FieldDescription data-testid="automation-where-hint">
          This job is saved on {ownerName} and runs there, even when this app is closed.
          {timeZone ? ` Schedule runs in ${timeZone}.` : ""}
        </FieldDescription>
      </Field>
    </AutomationFormSection>
  );
}
