import { CheckCircle2, Info, Play } from "lucide-react";

import { Button, Eyebrow, Section } from "@compozy/ui";
import { NetworkParticipationFields } from "@/systems/network";

import { useLoopRunForm } from "../../hooks/use-loop-run-form";
import { loopSourceLabel } from "../../lib/loop-catalog";
import { LoopPageLede } from "../loop-page-lede";
import type { LoopDetail, LoopEffectiveConfig, LoopRun } from "../../types";
import { LoopRunActiveNotice } from "./loop-run-active-notice";
import { LoopRunAfterStart } from "./loop-run-after-start";
import { LoopRunInputField } from "./loop-run-input-field";
import { LoopRunOverrides } from "./loop-run-overrides";
import { LoopRunPreview } from "./loop-run-preview";
import { LoopRunWaysToStart } from "./loop-run-ways-to-start";

interface LoopRunFormProps {
  workspaceId: string;
  loop: LoopDetail;
  effectiveConfig: LoopEffectiveConfig;
  /** The run of this loop that is already live, when the route resolved one. */
  activeRun?: LoopRun | null;
  onRunStarted?: (runId: string) => void;
}

/**
 * The hero run form (§4.3): an auto-generated typed input form from the Loop's declared
 * inputs, a folded per-run limits grid (clamped, no cost cap), a live contract preview,
 * and the Dry run / Start run actions. Leaving the form without starting is a route
 * action (`Close` in the window chrome), not a third button beside the two that submit.
 * State + the run/dry calls live in `useLoopRunForm`; this component is the presentation.
 */
export function LoopRunForm({
  workspaceId,
  loop,
  effectiveConfig,
  activeRun,
  onRunStarted,
}: LoopRunFormProps) {
  const form = useLoopRunForm({ workspaceId, loop, effectiveConfig, onRunStarted });
  const inputNames = form.schema ? Object.keys(form.schema) : [];

  return (
    <div
      className="grid min-h-0 flex-1 grid-cols-1 max-lg:block max-lg:overflow-y-auto lg:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]"
      data-testid="loop-run-form"
    >
      <form
        className="flex min-w-0 flex-col gap-6 overflow-y-auto px-6 py-5 max-lg:overflow-visible"
        onSubmit={event => {
          event.preventDefault();
          form.handleRun();
        }}
      >
        <LoopPageLede
          lede="Fill the inputs, start the run. Everything else — retries, budgets, gates — is already declared on the loop."
          meta={[
            loop.definition.apiVersion,
            `${inputNames.length} declared input${inputNames.length === 1 ? "" : "s"}`,
          ]}
          name={loop.name}
          prefix="Run"
          tags={[loopSourceLabel(loop), `v${loop.version}`]}
          testId="loop-run-form-lede"
        />

        <div className="max-w-prose">
          <Eyebrow className="text-muted">Contract goal</Eyebrow>
          <p className="mt-1.5 text-sm text-fg">{form.contract.goal}</p>
        </div>

        {activeRun ? (
          <LoopRunActiveNotice concurrency={loop.definition.concurrency} run={activeRun} />
        ) : null}

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

        <div className="flex flex-wrap items-center gap-2 border-t border-line-soft pt-4">
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
            Start run
          </Button>
        </div>

        <p className="flex items-start gap-2 text-form-hint leading-relaxed text-subtle">
          <Info className="mt-0.5 size-3 shrink-0 text-faint" aria-hidden="true" />
          <span>
            Dry run validates inputs and renders the first-generation plan without starting a run.
          </span>
        </p>
      </form>

      <div className="flex min-w-0 flex-col gap-3 overflow-y-auto border-t border-line-soft bg-canvas-tint px-5 py-5 max-lg:overflow-visible lg:border-t-0 lg:border-l">
        <LoopRunPreview
          loopName={loop.name}
          contract={form.contract}
          effectiveConfig={effectiveConfig}
          configOverrides={form.configOverrides}
          inputs={form.schema}
          networkParticipation={form.networkParticipation}
          plan={form.plan}
        />
        <LoopRunWaysToStart start={loop.definition.start} />
        <LoopRunAfterStart />
      </div>
    </div>
  );
}
