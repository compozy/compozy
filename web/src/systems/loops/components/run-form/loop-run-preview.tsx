import { MonoId, Pill } from "@compozy/ui";
import type { NetworkParticipationDraft } from "@/lib/network-participation";

import {
  resolveLoopEffectiveConfig,
  type LoopRunConfigOverrides,
} from "../../lib/loop-effective-config";
import type {
  LoopContract,
  LoopDryRunPreview,
  LoopEffectiveConfig,
  LoopInputSchema,
} from "../../types";
import { LoopContractRows } from "../detail/loop-contract-panel";
import { LoopRailSection } from "../loop-rail-section";
import { LoopRunPlan } from "./loop-run-plan";

interface LoopRunPreviewProps {
  loopName: string;
  contract: LoopContract;
  effectiveConfig: LoopEffectiveConfig;
  configOverrides: LoopRunConfigOverrides | null;
  inputs?: LoopInputSchema;
  networkParticipation: NetworkParticipationDraft;
  /** Present only after a successful Dry run. */
  plan?: LoopDryRunPreview | null;
}

/**
 * The run-form right rail (§4.3): a live "what will run" summary, the read-only
 * contract (goal, DoD, verification, terminal chips — reused from the loop detail
 * surface), the lifecycle line, and — after Dry run — the resolved gen-1 plan.
 */
export function LoopRunPreview({
  loopName,
  contract,
  effectiveConfig,
  configOverrides,
  inputs,
  networkParticipation,
  plan,
}: LoopRunPreviewProps) {
  const inputCount = inputs ? Object.keys(inputs).length : 0;
  const effective =
    plan?.effective_config ?? resolveLoopEffectiveConfig(effectiveConfig, configOverrides);
  const reattemptLabel = effective.reattempt_strategy === "full_body" ? "full-body" : "failed-only";
  const terminalCount = contract.terminal_states?.length ?? 0;
  return (
    <aside className="flex flex-col gap-3" data-testid="loop-run-preview">
      <LoopRailSection
        defaultOpen
        gist={`${inputCount} declared input${inputCount === 1 ? "" : "s"}`}
        title="What will run"
      >
        <div className="px-4 py-3.5">
          <p className="text-small-body leading-relaxed text-muted">
            Run <b className="font-medium text-fg-strong">{loopName}</b> against its contract:{" "}
            {contract.goal} Iterates up to{" "}
            <b className="font-medium text-fg-strong">
              {effective.iteration_cap === 0 ? "∞" : effective.iteration_cap}
            </b>{" "}
            generations with {reattemptLabel} re-attempts from {inputCount} declared input
            {inputCount === 1 ? "" : "s"}, ending honestly at one of its terminal outcomes.
          </p>
          <div
            className="mt-3 flex flex-wrap items-center gap-2 border-t border-line-soft pt-3"
            data-testid="loop-run-participation-preview"
          >
            <span className="text-form-hint text-subtle">Participation</span>
            <Pill tone="neutral">{networkParticipation.mode === "live" ? "Live" : "Local"}</Pill>
            {networkParticipation.mode === "live" ? (
              networkParticipation.channelId ? (
                <MonoId value={networkParticipation.channelId} />
              ) : (
                <span className="text-form-hint text-faint">channel resolved at start</span>
              )
            ) : null}
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-line-soft pt-3 font-mono text-mono-id text-subtle">
            <span className="text-muted">running</span>
            <span className="text-faint">→</span>
            <span className="text-muted">verify</span>
            {/* needs-approval only appears when the loop actually declares a human gate —
                a loop with no human criterion can never enter it (truthful lifecycle, §4.3). */}
            {effective.human_gate_enabled ? (
              <>
                <span className="text-faint">→</span>
                <span className="text-warning">needs-approval</span>
              </>
            ) : null}
            <span className="text-faint">→</span>
            <span className="text-success">done</span>
          </div>
        </div>
      </LoopRailSection>
      <LoopRailSection
        gist={`${contract.verification?.length ?? 0} checks · ${terminalCount} possible endings`}
        title="Contract"
      >
        <div className="flex flex-col">
          <LoopContractRows contract={contract} />
        </div>
      </LoopRailSection>
      {plan ? <LoopRunPlan plan={plan} /> : null}
    </aside>
  );
}
