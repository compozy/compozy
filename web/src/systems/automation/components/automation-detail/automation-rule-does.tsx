import { useState } from "react";
import type { ComponentProps, ReactNode } from "react";
import { Repeat2, SquareCheck } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { cn } from "@compozy/ui";

import { isAutomationTrigger, type AutomationEntity } from "../../lib/automation-detail";
import { formatAutomationInputValue, projectAutomationTarget } from "../../lib/automation-target";
import type { LoopTargetProjection } from "../../lib/automation-target";
import { tokenizeTemplate } from "../../lib/trigger-template";
import type { AutomationView } from "../../lib/automation-view";
import type { AutomationJob } from "../../types";
import { AutomationValueBadge } from "./automation-value-badge";

type TargetChipProps = Omit<ComponentProps<"span">, "children"> & { children: ReactNode };

function TargetChip({ children, className, ...props }: TargetChipProps) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex size-5.5 shrink-0 items-center justify-center rounded-md bg-surface-2 font-mono text-badge font-semibold text-muted",
        className
      )}
      {...props}
    >
      {children}
    </span>
  );
}

function RuleSub({ children }: { children: ReactNode }) {
  return <span className="mt-1.5 text-eyebrow leading-normal text-subtle">{children}</span>;
}

/** Message clamped to three lines; event templates mark their variables. */
function PromptPreview({ prompt, templated }: { prompt: string; templated: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <div className="mt-2 rounded-lg border border-line-soft bg-sunken px-3 py-2.5 font-mono text-form-hint leading-relaxed whitespace-pre-wrap text-fg-2">
        <div className={cn(!open && "line-clamp-3")} data-testid="automation-prompt-preview">
          {templated
            ? tokenizeTemplate(prompt).map(token =>
                token.type === "var" ? (
                  <AutomationValueBadge key={token.id} tone="variable" value={token.value} />
                ) : (
                  <span key={token.id}>{token.value}</span>
                )
              )
            : prompt}
        </div>
      </div>
      <button
        aria-expanded={open}
        className="mt-1.5 self-start rounded-xs text-form-label font-medium text-muted transition-colors duration-base ease-out hover:text-fg-strong focus-visible:shadow-focus-ring focus-visible:outline-none"
        data-testid="automation-prompt-toggle"
        onClick={() => setOpen(previous => !previous)}
        type="button"
      >
        {open ? "Hide prompt" : "Show full prompt"}
      </button>
    </>
  );
}

const MAPPING_ROW =
  "grid grid-cols-[minmax(88px,auto)_18px_minmax(0,1fr)_auto] items-center gap-2 border-t border-line-soft px-3 py-1.75 first:border-t-0";

/** `←` reads a field off the event; `=` is a value fixed when the automation was made. */
function LoopInputs({ target }: { target: LoopTargetProjection }) {
  const mapped = Object.entries(target.inputMapping);
  const statics = Object.entries(target.inputs);
  if (mapped.length === 0 && statics.length === 0) return null;
  return (
    <div className="mt-2 overflow-hidden rounded-lg bg-sunken" data-testid="automation-loop-inputs">
      {mapped.map(([key, path]) => (
        <div className={MAPPING_ROW} key={`map-${key}`}>
          <span className="font-mono text-form-hint text-fg">{key}</span>
          <span aria-hidden="true" className="text-center text-small-body text-faint">
            ←
          </span>
          <span className="min-w-0">
            <AutomationValueBadge fill value={path} />
          </span>
          <span className="text-badge text-faint">from the event</span>
        </div>
      ))}
      {statics.map(([key, value]) => (
        <div className={MAPPING_ROW} key={`static-${key}`}>
          <span className="font-mono text-form-hint text-fg">{key}</span>
          <span aria-hidden="true" className="text-center text-small-body text-faint">
            =
          </span>
          <span className="min-w-0">
            <AutomationValueBadge fill value={formatAutomationInputValue(value)} />
          </span>
          <span className="text-badge text-faint">always</span>
        </div>
      ))}
    </div>
  );
}

const OWNER_KIND_WORDS = {
  human: "Person",
  agent_session: "Agent session",
  automation: "Automation",
  extension: "Extension",
  pool: "Agent pool",
} as const;

function TaskDoes({ job }: { job: AutomationJob }) {
  const owner = job.task?.owner;
  return (
    <>
      <span className="flex min-w-0 items-center gap-2">
        <TargetChip>
          <SquareCheck className="size-3" />
        </TargetChip>
        <b className="text-form-input font-medium text-fg-strong">Create a task</b>
      </span>
      <dl
        className="mt-2 grid grid-cols-[56px_minmax(0,1fr)] gap-x-3 gap-y-1 text-small-body"
        data-testid="automation-task-details"
      >
        <dt className="text-subtle">Title</dt>
        <dd className="min-w-0 text-fg">{job.task?.title?.trim() || job.name}</dd>
        <dt className="text-subtle">For</dt>
        <dd className="min-w-0 text-fg">
          {owner ? (
            <>
              {OWNER_KIND_WORDS[owner.kind]}{" "}
              <span className="font-mono text-mono-id text-muted">{owner.ref}</span>
            </>
          ) : (
            "Unassigned"
          )}
        </dd>
      </dl>
      <RuleSub>Each run creates a new task. The task handles its own retries.</RuleSub>
    </>
  );
}

interface AutomationRuleDoesProps {
  entity: AutomationEntity;
  view: AutomationView;
  loopWorkspaceName: string | null;
  /** The Loop is gone from its project's catalog. */
  loopMissing: boolean;
}

/**
 * Does splits on the immutable target kind: an agent receives a message, a
 * Loop receives labeled inputs and owns the run from there, a task owns its
 * own work.
 */
export function AutomationRuleDoes({
  entity,
  view,
  loopWorkspaceName,
  loopMissing,
}: AutomationRuleDoesProps) {
  if (view.does === "task" && !isAutomationTrigger(entity)) return <TaskDoes job={entity} />;
  const target = projectAutomationTarget(entity);

  if (target.kind === "loop") {
    const workspaceLabel = loopWorkspaceName ?? target.workspaceId;
    return (
      <>
        <span className="flex min-w-0 items-center gap-2">
          <TargetChip>
            <Repeat2 className="size-3" />
          </TargetChip>
          <b className="min-w-0 truncate text-form-input font-medium text-fg-strong">
            Start the Loop{" "}
            {loopMissing ? (
              <span data-testid="automation-loop-name">{target.loopName}</span>
            ) : (
              <Link
                className="rounded-xs underline underline-offset-2 transition-colors duration-base ease-out hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
                data-testid="automation-loop-link"
                params={{ name: target.loopName }}
                search={target.workspaceId ? { workspace: target.workspaceId } : {}}
                to="/loops/$name"
              >
                {target.loopName}
              </Link>
            )}
          </b>
        </span>
        <RuleSub>
          {loopMissing
            ? "This Loop no longer exists."
            : `Starts a Loop run in ${workspaceLabel} with these inputs:`}
        </RuleSub>
        <LoopInputs target={target} />
      </>
    );
  }

  const templated = view.start !== "schedule";
  const noun = view.start === "webhook" ? "request" : "event";
  return (
    <>
      <span className="flex min-w-0 items-center gap-2">
        <TargetChip>{target.agentName.charAt(0)}</TargetChip>
        <span className="min-w-0 truncate text-form-input text-fg">
          Ask <b className="font-medium text-fg-strong">{target.agentName}</b>
        </span>
      </span>
      <RuleSub>
        {templated
          ? `The message is filled in from each ${noun}.`
          : "The agent gets this message, word for word, in a new session."}
      </RuleSub>
      {target.prompt.trim() === "" ? null : (
        <PromptPreview prompt={target.prompt} templated={templated} />
      )}
    </>
  );
}
