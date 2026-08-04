import { describeStartKinds } from "../../lib/loop-start-kinds";
import { LoopRailSection } from "../loop-rail-section";
import type { LoopStartBinding } from "../../types";

interface LoopRunWaysToStartProps {
  start: readonly LoopStartBinding[] | undefined;
}

/**
 * Which start surfaces this loop declares, and which one the operator is using now.
 *
 * Read-only by contract: authoring the `start[]` allowlist is Spec 3 (ADR-018 §3), so
 * the other declared kinds render as plain text with no control attached to them.
 */
export function LoopRunWaysToStart({ start }: LoopRunWaysToStartProps) {
  const kinds = describeStartKinds(start);
  return (
    <LoopRailSection
      data-testid="loop-run-ways-to-start"
      gist={`this form starts over ${kinds.thisForm}${
        kinds.alsoDeclared.length > 0 ? ` · ${kinds.alsoDeclared.length} more` : ""
      }`}
      title="Ways to start"
    >
      <dl className="flex flex-col gap-2 px-4 py-3.5">
        <div className="flex items-baseline justify-between gap-3">
          <dt className="text-form-hint text-subtle">This form</dt>
          <dd className="font-mono text-mono-id text-fg" data-testid="loop-run-this-form-kind">
            {kinds.thisForm}
          </dd>
        </div>
        {kinds.alsoDeclared.length > 0 ? (
          <div className="flex flex-col gap-1">
            <dt className="text-form-hint text-subtle">Also declared in start[]</dt>
            <dd
              className="font-mono text-mono-id break-words text-subtle"
              data-testid="loop-run-other-start-kinds"
            >
              {kinds.alsoDeclared.join(" · ")}
            </dd>
          </div>
        ) : null}
      </dl>
      <p className="px-4 pb-3.5 text-form-hint leading-relaxed text-faint">
        Automated starts are managed as Start bindings on the loop page.
      </p>
    </LoopRailSection>
  );
}
