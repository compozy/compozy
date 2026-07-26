import { CheckCircle2, Info, Play } from "lucide-react";

import { Button, Eyebrow, Section } from "@agh/ui";
import { NetworkParticipationFields } from "@/systems/network";

import { useLoopRunForm } from "../../hooks/use-loop-run-form";
import type { LoopDetail, LoopEffectiveConfig } from "../../types";
import { LoopRunInputField } from "./loop-run-input-field";
import { LoopRunOverrides } from "./loop-run-overrides";
import { LoopRunPreview } from "./loop-run-preview";

interface LoopRunFormProps {
  workspaceId: string;
  loop: LoopDetail;
  effectiveConfig: LoopEffectiveConfig;
  onRunStarted?: (runId: string) => void;
  onCancel?: () => void;
}

/**
 * The hero run form (§4.3): an auto-generated typed input form from the Loop's declared
 * inputs, an Advanced per-run override grid (clamped, no cost cap), a live contract
 * preview, and the Dry run / Run actions. State + the run/dry calls live in
 * `useLoopRunForm`; this component is the presentation.
 */
export function LoopRunForm({
  workspaceId,
  loop,
  effectiveConfig,
  onRunStarted,
  onCancel,
}: LoopRunFormProps) {
  const form = useLoopRunForm({ workspaceId, loop, effectiveConfig, onRunStarted });
  const inputNames = form.schema ? Object.keys(form.schema) : [];

  return (
    <div
      className="grid min-h-0 flex-1 grid-cols-1 max-lg:block max-lg:overflow-y-auto lg:grid-cols-[minmax(0,1fr)_400px]"
      data-testid="loop-run-form"
    >
      <form
        className="flex min-w-0 flex-col gap-6 overflow-y-auto px-6 py-5 max-lg:overflow-visible"
        onSubmit={event => {
          event.preventDefault();
          form.handleRun();
        }}
      >
        <div className="max-w-prose">
          <Eyebrow className="text-muted">Contract goal</Eyebrow>
          <p className="mt-1.5 text-sm text-fg">{form.contract.goal}</p>
        </div>

        <Section
          label="Inputs"
          note={inputNames.length > 0 ? "Generated from the loop's declared inputs" : undefined}
        >
          {inputNames.length === 0 ? (
            <p className="text-sm text-subtle">This Loop declares no inputs — run it directly.</p>
          ) : (
            <div className="flex flex-col gap-4">
              {inputNames.map(name => (
                <LoopRunInputField
                  key={name}
                  name={name}
                  field={form.schema![name]}
                  value={form.inputs[name]}
                  disabled={form.busy}
                  error={
                    form.submitAttempted && form.missing.has(name)
                      ? `${name} is required to run this loop.`
                      : undefined
                  }
                  onChange={value => form.setInput(name, value)}
                />
              ))}
            </div>
          )}
        </Section>

        <Section label="Participation" note="Resolved once when this run starts.">
          <NetworkParticipationFields
            allowedStrategies={["named", "loop_run"]}
            disabled={form.busy}
            onChange={form.setNetworkParticipationDraft}
            testIdPrefix="loop-run-participation"
            value={form.networkParticipation}
          />
        </Section>

        <LoopRunOverrides
          effectiveConfig={effectiveConfig}
          draft={form.overrides}
          disabled={form.busy}
          onChange={form.setOverridesDraft}
        />

        <p className="flex items-start gap-2 text-form-hint leading-relaxed text-subtle">
          <Info className="mt-0.5 size-3 shrink-0 text-faint" aria-hidden="true" />
          <span>
            Dry run validates inputs and renders the first-generation plan without starting a run.
            Manage automated starts in <b className="font-medium text-muted">Start bindings</b> on
            the loop page.
          </span>
        </p>
      </form>

      <div className="flex min-w-0 flex-col border-t border-line-soft bg-canvas-tint lg:border-t-0 lg:border-l">
        <div className="flex-1 overflow-y-auto px-5 py-5 max-lg:overflow-visible">
          <LoopRunPreview
            loopName={loop.name}
            contract={form.contract}
            effectiveConfig={effectiveConfig}
            configOverrides={form.configOverrides}
            inputs={form.schema}
            networkParticipation={form.networkParticipation}
            plan={form.plan}
          />
        </div>
        <div className="flex items-center gap-2 border-t border-line-soft px-5 py-3.5">
          {form.plan ? (
            <span className="flex flex-1 items-center gap-1.5 text-form-hint text-success">
              <CheckCircle2 className="size-3.5" aria-hidden="true" />
              Plan rendered
            </span>
          ) : (
            <span className="flex-1 text-form-hint text-subtle">
              {form.valid ? "Ready to run." : "Fill the required inputs, then run."}
            </span>
          )}
          {onCancel ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={form.busy}
              onClick={onCancel}
            >
              Cancel
            </Button>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            data-testid="loop-run-dry-button"
            disabled={form.busy || !form.valid}
            onClick={form.handleDryRun}
          >
            Dry run
          </Button>
          <Button
            type="button"
            variant="primary"
            size="sm"
            data-testid="loop-run-submit-button"
            disabled={form.busy || !form.valid}
            onClick={form.handleRun}
          >
            <Play className="size-3.5" aria-hidden="true" />
            Run loop
          </Button>
        </div>
      </div>
    </div>
  );
}
