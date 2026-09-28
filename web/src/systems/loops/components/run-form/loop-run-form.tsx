import { CheckCircle2, FlaskConical, Info, Play, TextCursorInput } from "lucide-react";

import { Button, PageContent, Spinner } from "@compozy/ui";

import { useLoopRunForm } from "../../hooks/use-loop-run-form";
import { loopSourceLabel } from "../../lib/loop-catalog";
import { declaredInputCountsGist, loopInputLabel } from "../../lib/loop-run-form";
import { LoopPageLede } from "../loop-page-lede";
import { LoopRailSection } from "../loop-rail-section";
import type { LoopDetail, LoopEffectiveConfig, LoopRun } from "../../types";
import { LoopRunActiveNotice } from "./loop-run-active-notice";
import { LoopRunInputField } from "./loop-run-input-field";
import { LoopRunOverrides } from "./loop-run-overrides";
import { LoopRunPlan } from "./loop-run-plan";
import { ProfileDestinationChip } from "@/systems/profiles";
import { useWorktrees } from "@/systems/workspace";
import { LoopRunEnvironment } from "./loop-run-environment";
import { loopInputCatalogNeeds } from "../../lib/loop-input-catalogs";
import { LoopInputCatalogBoundary } from "../input/loop-input-catalogs";

const DRY_RUN_SENTENCE = "Dry run checks your inputs without starting anything.";

interface LoopRunFormProps {
  workspaceId: string;
  loop: LoopDetail;
  effectiveConfig: LoopEffectiveConfig;
  /** The run of this loop that is already live, when the route resolved one. */
  activeRun?: LoopRun | null;
  onRunStarted?: (runId: string) => void;
}

function actionBarNote(form: {
  missing: Set<string>;
  schema: LoopDetail["definition"]["inputs"] | null | undefined;
  plan: unknown;
  submitAttempted: boolean;
  valid: boolean;
}): { kind: "plan" | "missing" | "idle"; text: string } {
  if (form.plan) return { kind: "plan", text: "Plan rendered" };
  if (form.submitAttempted && !form.valid) {
    if (form.missing.size === 1) {
      const [name] = form.missing;
      return { kind: "missing", text: requiredMessage(name, form.schema?.[name]) };
    }
    return { kind: "missing", text: "Fill the required inputs, then run." };
  }
  return { kind: "idle", text: DRY_RUN_SENTENCE };
}

function requiredMessage(
  name: string,
  field: NonNullable<LoopDetail["definition"]["inputs"]>[string] | undefined
): string {
  return `${loopInputLabel(name, field)} is required to run this Loop.`;
}

function LoopRunFormActions({
  busy,
  note,
  onDryRun,
  onRun,
  pendingKind,
  profileDestination,
  valid,
}: {
  busy: boolean;
  note: { kind: "plan" | "missing" | "idle"; text: string };
  onDryRun: () => void;
  onRun: () => void;
  pendingKind: "dry-run" | "run" | null;
  /** Where the run will be filed, stated only while the aggregate is on. */
  profileDestination: string | null;
  valid: boolean;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-line-soft pt-4">
      <span
        className={
          note.kind === "plan"
            ? "flex flex-1 items-center gap-1.5 text-form-hint text-success"
            : "flex flex-1 items-start gap-1.5 text-form-hint leading-relaxed text-subtle"
        }
      >
        {note.kind === "plan" ? (
          <CheckCircle2 className="size-3.5 shrink-0" aria-hidden="true" />
        ) : null}
        {note.kind === "idle" ? (
          <Info className="mt-0.5 size-3.5 shrink-0 text-faint" aria-hidden="true" />
        ) : null}
        {note.text}
      </span>
      {profileDestination ? <ProfileDestinationChip profile={profileDestination} /> : null}
      <Button
        type="button"
        variant="secondary"
        size="sm"
        data-testid="loop-run-dry-button"
        disabled={busy}
        onClick={onDryRun}
      >
        {pendingKind === "dry-run" ? (
          <Spinner className="size-3.5" />
        ) : (
          <FlaskConical className="size-3.5" aria-hidden="true" />
        )}
        Dry run
      </Button>
      <Button
        type="button"
        variant="primary"
        size="sm"
        data-testid="loop-run-submit-button"
        disabled={busy || !valid}
        onClick={onRun}
      >
        {pendingKind === "run" ? (
          <Spinner className="size-3.5" />
        ) : (
          <Play className="size-3.5" aria-hidden="true" />
        )}
        Start run
      </Button>
    </div>
  );
}

export function LoopRunForm({
  workspaceId,
  loop,
  effectiveConfig,
  activeRun,
  onRunStarted,
}: LoopRunFormProps) {
  const worktrees = useWorktrees(workspaceId);
  const gitBacked = Boolean(worktrees.data?.repo.git_backed);
  const form = useLoopRunForm({
    workspaceId,
    loop,
    effectiveConfig,
    gitBacked,
    onRunStarted,
  });
  const inputNames = form.schema ? Object.keys(form.schema) : [];
  const note = actionBarNote(form);
  const inputCatalogNeeds = loopInputCatalogNeeds(form.schema);

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto" data-testid="loop-run-form">
      <PageContent className="flex max-w-2xl min-w-0 flex-col gap-6" density="route">
        <form
          className="flex min-w-0 flex-col gap-6"
          onSubmit={event => {
            event.preventDefault();
            form.handleRun();
          }}
        >
          <LoopPageLede
            lede={form.contract.goal}
            meta={[]}
            name={loop.name}
            prefix="Run"
            tags={[loopSourceLabel(loop), `v${loop.version}`]}
            testId="loop-run-form-lede"
          />

          {activeRun ? (
            <LoopRunActiveNotice concurrency={loop.definition.concurrency} run={activeRun} />
          ) : null}

          <LoopRailSection
            defaultOpen
            gist={declaredInputCountsGist(form.schema)}
            icon={<TextCursorInput aria-hidden="true" className="size-3.5" />}
            title="Inputs"
          >
            <LoopInputCatalogBoundary workspaceId={workspaceId} needs={inputCatalogNeeds}>
              {inputNames.length === 0 ? (
                <p className="px-4 py-3 text-form-hint text-subtle">No inputs needed.</p>
              ) : (
                <div className="flex flex-col gap-4 px-4 py-3">
                  {inputNames.map(name => (
                    <LoopRunInputField
                      key={name}
                      name={name}
                      field={form.schema![name]}
                      value={form.inputs[name]}
                      disabled={form.busy}
                      error={
                        form.fieldErrors[name] ??
                        (form.submitAttempted && form.missing.has(name)
                          ? requiredMessage(name, form.schema?.[name])
                          : undefined)
                      }
                      onChange={value => form.setInput(name, value)}
                    />
                  ))}
                </div>
              )}
            </LoopInputCatalogBoundary>
          </LoopRailSection>

          <LoopRunEnvironment
            disabled={form.busy}
            gitBacked={gitBacked}
            onChange={environment => form.setOverridesDraft({ ...form.overrides, environment })}
            value={form.overrides.environment}
            worktrees={worktrees.data?.worktrees ?? []}
          />

          <LoopRunOverrides
            effectiveConfig={effectiveConfig}
            draft={form.overrides}
            disabled={form.busy}
            onChange={form.setOverridesDraft}
          />

          <LoopRunFormActions
            busy={form.busy}
            note={note}
            onDryRun={form.handleDryRun}
            onRun={form.handleRun}
            pendingKind={form.pendingKind}
            profileDestination={form.profileDestination}
            valid={form.valid}
          />

          {form.plan ? <LoopRunPlan plan={form.plan} /> : null}
        </form>
      </PageContent>
    </div>
  );
}
