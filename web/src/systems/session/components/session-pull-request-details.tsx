import { CheckIcon, ClockIcon } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { MonoId, Pill, Spinner, Time, cn } from "@compozy/ui";

import type { PullRequestChipModel, PullRequestDetailLine } from "../lib/pull-request-signal";

function DetailMark({ line }: { line: PullRequestDetailLine }) {
  return (
    <span aria-hidden="true" className="grid size-3.5 shrink-0 place-items-center text-subtle">
      {line.mark === "check" ? (
        <CheckIcon className="size-3.5" />
      ) : line.mark === "spinner" ? (
        <Spinner className="size-3" role={undefined} />
      ) : (
        <Pill.Dot size="sm" tone={line.tone ?? "neutral"} />
      )}
    </span>
  );
}

function FailedChecks({ names, more }: { names: readonly string[]; more: number }) {
  return (
    <ul className="flex flex-col gap-0.5 pl-5.5" data-slot="pull-request-failed-checks">
      {names.map(name => (
        <li className="truncate font-mono text-mono-id text-fg-2" key={name}>
          {name}
        </li>
      ))}
      {more > 0 ? <li className="text-eyebrow text-subtle">{more} more</li> : null}
    </ul>
  );
}

type PullRequestModel = Extract<PullRequestChipModel, { kind: "pull-request" }>;

function DetailLines({ model }: { model: PullRequestModel }) {
  return (
    <ul className="flex flex-col gap-1.5" data-slot="pull-request-detail-lines">
      {model.lines.map(line => (
        <li className="flex flex-col gap-1" data-line={line.id} key={line.id}>
          <span
            className={cn(
              "flex items-center gap-2 text-small-body",
              line.stale ? "text-muted-foreground" : "text-fg",
              line.tone === "destructive" && "text-destructive"
            )}
          >
            <DetailMark line={line} />
            {line.text}
          </span>
          {line.id === "checks" && model.failedNames.length > 0 ? (
            <FailedChecks more={model.failedMore} names={model.failedNames} />
          ) : null}
        </li>
      ))}
    </ul>
  );
}

function Observed({ model }: { model: PullRequestModel }) {
  if (!model.observedAt) return <span>Not checked yet</span>;
  return (
    <span className="inline-flex shrink-0 items-center gap-1">
      {model.stale ? <ClockIcon aria-hidden="true" className="size-3" /> : null}
      {model.stale ? "Last checked " : "Checked "}
      <Time iso={model.observedAt} />
    </span>
  );
}

function BranchFoot({ branch, children }: { branch: string; children?: ReactNode }) {
  return (
    <div
      className="flex min-w-0 items-center justify-between gap-3 text-eyebrow text-subtle"
      data-slot="pull-request-details-foot"
    >
      <MonoId className="min-w-0 truncate" preserveCase value={branch} />
      {children}
    </div>
  );
}

const RULED = "border-t border-line-soft pt-2.5";

/**
 * What stands between a pull request and merging, one plain sentence per line, with the branch
 * and the last check underneath. Shared by the chip's tooltip and every list that names a PR.
 */
export function SessionPullRequestDetails({ model }: { model: PullRequestChipModel }) {
  if (model.kind === "branch") {
    return (
      <>
        <p className="text-small-body text-fg">{model.detail}</p>
        <div className={RULED}>
          <BranchFoot branch={model.branch} />
        </div>
      </>
    );
  }
  return (
    <>
      {model.lines.length > 0 ? <DetailLines model={model} /> : null}
      <div className={cn("flex flex-col gap-1", model.lines.length > 0 && RULED)}>
        <BranchFoot branch={model.branch}>
          <Observed model={model} />
        </BranchFoot>
        {model.causeText ? (
          <span className="text-eyebrow text-subtle" data-slot="pull-request-details-cause">
            {model.causeText}
          </span>
        ) : null}
      </div>
    </>
  );
}
