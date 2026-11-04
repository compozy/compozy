/** Heartbeat › Sources with a script that waits for approval: two ways to ask for it. */
import { FileCode } from "@compozy/ui/icons";

import { Alert, AlertDescription, AlertTitle, Button, Eyebrow, Time } from "@compozy/ui";

import { CheckInSourceRow } from "@/systems/agent/components/agent-editors/check-in-source-row";
import { EditorHead } from "@/systems/agent/components/agent-editors/editor-head";
import { EditorCard } from "@/systems/agent/components/agent-editors/rich-controls";
import { shortHash, type SourceStatusView } from "@/systems/agent/lib/agent-check-in-sources";
import { primaryAgentFixture } from "@/systems/agent/mocks/fixtures";
import {
  approvedScriptFixture,
  changedScriptFixture,
  pendingScriptFixture,
} from "@/systems/agent/mocks/script-approval-fixtures";

/** `inline`: the row carries the hash and Approve. `callout`: a card above the list asks first. */
export type SourcesVariant = "inline" | "callout";
/** Before the first check, on a paused place, after an edit, and once approved. */
export type SourcesCase = "first" | "paused" | "changed" | "approved";

const AGENT = { ...primaryAgentFixture, name: "merge-train", label: "Merge Train" };

function scriptOf(which: SourcesCase) {
  if (which === "changed") return changedScriptFixture;
  if (which === "approved") return approvedScriptFixture;
  return pendingScriptFixture;
}

function scriptStatus(which: SourcesCase, variant: SourcesVariant): SourceStatusView {
  const script = scriptOf(which);
  if (which === "approved") return { label: "Approved · runs at the next check-in", tone: "muted" };
  const hash = variant === "inline" ? ` · ${shortHash(script.sha256)}` : "";
  if (which === "changed") return { label: `Changed since you approved${hash}`, tone: "warning" };
  return { label: `Waiting for your approval${hash}`, tone: "warning" };
}

function ApproveCallout({ which }: { which: SourcesCase }) {
  const script = scriptOf(which);
  return (
    <Alert variant="warning">
      <AlertTitle>
        {which === "changed"
          ? `${script.name} changed since you approved it`
          : `${script.name} needs your approval to run`}
      </AlertTitle>
      <AlertDescription>
        <p>
          Heartbeat runs it on this computer.{" "}
          {which === "paused" ? "You can approve it while the agent is paused. " : null}
          Check what it does, then approve this exact version.
        </p>
        <p className="mt-1 font-mono text-eyebrow text-subtle">
          {shortHash(script.sha256)} · {script.line_count} lines · changed{" "}
          <Time iso={script.last_modified ?? ""} />
        </p>
        <div className="mt-2 flex gap-2">
          <Button size="sm" type="button" variant="secondary">
            Approve
          </Button>
          <Button size="sm" type="button" variant="quiet">
            <FileCode aria-hidden="true" data-icon="inline-start" />
            Open script
          </Button>
        </div>
      </AlertDescription>
    </Alert>
  );
}

export function ScriptSourcesCard({
  variant,
  which,
}: {
  variant: SourcesVariant;
  which: SourcesCase;
}) {
  const script = scriptOf(which);
  const waiting = which !== "approved";
  const paused = which === "paused";
  return (
    <div className="flex max-w-3xl flex-col gap-6 p-6">
      <EditorHead
        agent={AGENT}
        state={paused ? "paused" : "idle"}
        subtitle={paused ? "Waiting for you" : "Every 30 min, 9:00–18:00"}
        title="Heartbeat"
      />
      <EditorCard>
        {variant === "callout" && waiting ? <ApproveCallout which={which} /> : null}
        <div className="flex flex-col">
          <Eyebrow className="text-subtle">Sources</Eyebrow>
          <div className="flex flex-col divide-y divide-line-soft">
            <CheckInSourceRow
              params="compozy/compozy"
              script={false}
              status={{ label: "Checked 12 min ago · 2 new", tone: "neutral" }}
              title="New GitHub issues"
            />
            <CheckInSourceRow
              actions={
                variant === "inline" && waiting ? (
                  <Button size="sm" type="button" variant="neutral">
                    Approve
                  </Button>
                ) : null
              }
              script
              status={scriptStatus(which, variant)}
              title={script.name}
            />
          </div>
        </div>
      </EditorCard>
    </div>
  );
}
