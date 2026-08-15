import { Repeat2 } from "lucide-react";

import { CodeBlock, Eyebrow, Pill, Section } from "@compozy/ui";

import { describeFireLimit, describeRetry } from "../lib/automation-formatters";
import type { LoopTargetProjection } from "../lib/automation-target";
import type { AutomationJob } from "../types";
import { AutomationTargetDetails } from "./automation-target-details";

export function AutomationTargetSection({ target }: { target: LoopTargetProjection }) {
  return (
    <Section
      label="Target"
      right={
        <Pill mono tone="accent">
          <Repeat2 aria-hidden="true" className="size-3" />
          LOOP
        </Pill>
      }
    >
      <div className="rounded-md border border-line bg-canvas-soft px-4 py-3">
        <AutomationTargetDetails showInputMapping={false} target={target} />
      </div>
    </Section>
  );
}

export function PromptSection({ prompt }: { prompt: string }) {
  return (
    <Section label="Prompt">
      <CodeBlock code={prompt} copyable={false} showPrompt={false} />
    </Section>
  );
}

export function GovernanceSection({ item }: { item: AutomationJob }) {
  return (
    <Section label="Governance">
      <div className="grid gap-2 rounded-md border border-line bg-canvas-soft px-4 py-3 md:grid-cols-2">
        <div>
          <Eyebrow className="text-muted">Retry</Eyebrow>
          <p className="mt-1 text-small-body text-muted">{describeRetry(item.retry)}</p>
        </div>
        <div>
          <Eyebrow className="text-muted">Fire limit</Eyebrow>
          <p className="mt-1 text-small-body text-muted">{describeFireLimit(item.fire_limit)}</p>
        </div>
      </div>
    </Section>
  );
}
