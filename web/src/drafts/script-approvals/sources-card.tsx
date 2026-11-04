/** Heartbeat › Sources with a script that waits for approval: two ways to ask for it. */
import { FileCode, SquareTerminal } from "@compozy/ui/icons";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertMeta,
  AlertTitle,
  Button,
  Time,
} from "@compozy/ui";

import { CheckInSourceRow } from "@/systems/agent/components/agent-editors/check-in-source-row";
import { EditorHead } from "@/systems/agent/components/agent-editors/editor-head";
import { EditorCard, SettingRow } from "@/systems/agent/components/agent-editors/rich-controls";
import type { SourceStatusView } from "@/systems/agent/lib/agent-check-in-sources";
import {
  approvedScriptFixture,
  changedScriptFixture,
  pendingScriptFixture,
} from "@/systems/agent/mocks/script-approval-fixtures";

import { MERGE_TRAIN_PAYLOAD } from "./fleet-fixtures";

/** `inline`: the waiting row asks for itself. `callout`: a notice above the list asks first. */
export type SourcesVariant = "inline" | "callout";
/** Before the first check, on a paused place, after an edit, and once approved. */
export type SourcesCase = "first" | "paused" | "changed" | "approved";

function scriptOf(which: SourcesCase) {
  if (which === "changed") return changedScriptFixture;
  if (which === "approved") return approvedScriptFixture;
  return pendingScriptFixture;
}

/** What the source row says on its own; the hash lives in the script, not the default read. */
function scriptStatus(which: SourcesCase): SourceStatusView {
  if (which === "approved") return { label: "Approved · runs at the next check-in", tone: "muted" };
  if (which === "changed") return { label: "Changed since you approved it", tone: "warning" };
  return { label: "Needs your approval", tone: "warning" };
}

/** One plain sentence on why it waits and what approving does. */
function waitingCopy(which: SourcesCase) {
  if (which === "changed") {
    return "The file changed since you approved it, so Heartbeat stopped running it. Approve the new version to bring it back.";
  }
  if (which === "paused") {
    return "Heartbeat runs it on this computer at each check-in. Approve it now and it runs as soon as you resume Merge Train.";
  }
  return "Heartbeat runs it on this computer at each check-in. It stays off until you approve this version.";
}

function OpenScript() {
  return (
    <Button size="sm" type="button" variant="ghost">
      <FileCode aria-hidden="true" data-icon="inline-start" />
      Open script
    </Button>
  );
}

/** B: a notice above the list, built from the Alert's own title, meta and actions slots. */
function ApproveCallout({ which }: { which: SourcesCase }) {
  const script = scriptOf(which);
  return (
    <Alert aria-label={`${script.name} needs your approval`} role="region">
      <SquareTerminal aria-hidden="true" className="text-warning" />
      <AlertTitle>
        {which === "changed"
          ? `Approve the new version of ${script.name}`
          : `Approve ${script.name} to let it run`}
      </AlertTitle>
      <AlertDescription>{waitingCopy(which)}</AlertDescription>
      <AlertMeta>
        <span>{script.line_count} lines</span>
        <span>
          Changed <Time iso={script.last_modified ?? ""} />
        </span>
      </AlertMeta>
      <AlertActions>
        <OpenScript />
        <Button size="sm" type="button">
          Approve
        </Button>
      </AlertActions>
    </Alert>
  );
}

/** A: the waiting row lifts out of the list's rhythm and asks in its own words. */
function WaitingScriptRow({ which }: { which: SourcesCase }) {
  const script = scriptOf(which);
  return (
    <div className="-mx-3 my-1 flex items-start gap-3 rounded-lg bg-surface-2 p-3">
      <SquareTerminal aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
      <div className="flex min-w-0 flex-1 flex-col gap-3">
        <div className="flex flex-col gap-1">
          <span className="text-small-body font-medium text-fg">
            {which === "changed" ? `${script.name} changed` : `${script.name} needs your approval`}
          </span>
          <span className="text-small-body text-muted-foreground text-pretty">
            {waitingCopy(which)}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <Button size="sm" type="button">
            Approve
          </Button>
          <OpenScript />
        </div>
      </div>
    </div>
  );
}

/** Heartbeat's Sources section with the waiting script, in either variation. */
export function ScriptSourcesBlock({
  variant,
  which,
}: {
  variant: SourcesVariant;
  which: SourcesCase;
}) {
  const script = scriptOf(which);
  const waiting = which !== "approved";
  return (
    <SettingRow title="Sources" value={waiting ? "1 needs your approval" : undefined}>
      {variant === "callout" && waiting ? <ApproveCallout which={which} /> : null}
      <div className="flex flex-col divide-y divide-line-soft">
        <CheckInSourceRow
          params="compozy/compozy"
          script={false}
          status={{ label: "Checked 12 min ago · 2 new", tone: "neutral" }}
          title="New GitHub issues"
        />
        {variant === "inline" && waiting ? (
          <WaitingScriptRow which={which} />
        ) : (
          <CheckInSourceRow script status={scriptStatus(which)} title={script.name} />
        )}
      </div>
    </SettingRow>
  );
}

export function ScriptSourcesCard({
  variant,
  which,
}: {
  variant: SourcesVariant;
  which: SourcesCase;
}) {
  const paused = which === "paused";
  return (
    <div className="flex max-w-3xl flex-col gap-6 p-6">
      <EditorHead
        agent={MERGE_TRAIN_PAYLOAD}
        state={paused ? "paused" : "idle"}
        subtitle={paused ? "Waiting for you" : "Every 30 min, 9:00–18:00"}
        title="Heartbeat"
      />
      <EditorCard>
        <ScriptSourcesBlock variant={variant} which={which} />
      </EditorCard>
    </div>
  );
}
