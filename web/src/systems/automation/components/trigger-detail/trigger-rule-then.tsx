import { useState } from "react";
import type { ComponentProps, ReactNode } from "react";
import { Workflow } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { cn } from "@compozy/ui";

import { formatAutomationInputValue, projectAutomationTarget } from "../../lib/automation-target";
import { tokenizeTemplate } from "../../lib/trigger-template";
import type { LoopTargetProjection } from "../../lib/automation-target";
import type { AutomationTrigger } from "../../types";
import { TriggerValueBadge } from "./trigger-value-badge";

type TargetChipProps = Omit<ComponentProps<"span">, "children"> & { children: ReactNode };

function TargetChip({ children, className, ...props }: TargetChipProps) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex size-5.5 shrink-0 items-center justify-center rounded-md border border-line-strong bg-elevated font-mono text-badge font-semibold text-muted",
        className
      )}
      {...props}
    >
      {children}
    </span>
  );
}

function TriggerPromptPreview({ prompt }: { prompt: string }) {
  const [open, setOpen] = useState(false);
  const tokens = tokenizeTemplate(prompt);
  return (
    <>
      <div
        className={cn(
          "mt-2 rounded-md border border-line-soft bg-input-fill px-3 py-2.5 font-mono text-form-label leading-relaxed whitespace-pre-wrap text-muted",
          !open && "line-clamp-3"
        )}
        data-testid="trigger-prompt-preview"
      >
        {tokens.map(token =>
          token.type === "var" ? (
            <TriggerValueBadge key={token.id} value={token.value} />
          ) : (
            <span key={token.id}>{token.value}</span>
          )
        )}
      </div>
      <button
        aria-expanded={open}
        className="mt-1.5 self-start rounded-xs text-form-label font-medium text-muted transition-colors duration-base ease-out hover:text-fg-strong focus-visible:shadow-focus-ring focus-visible:outline-none"
        data-testid="trigger-prompt-toggle"
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

/**
 * How the loop's declared inputs get filled: `←` reads a field off the event,
 * `=` is a value fixed at create time.
 */
type TriggerLoopMappingProps = Omit<ComponentProps<"div">, "children"> & {
  target: LoopTargetProjection;
};

function TriggerLoopMapping({ target, className, ...props }: TriggerLoopMappingProps) {
  const mapped = Object.entries(target.inputMapping);
  const statics = Object.entries(target.inputs);
  if (mapped.length === 0 && statics.length === 0) return null;
  return (
    <div
      className={cn(
        "mt-2 overflow-hidden rounded-md border border-line-soft bg-input-fill",
        className
      )}
      data-testid="trigger-loop-mapping"
      {...props}
    >
      {mapped.map(([key, path]) => (
        <div className={MAPPING_ROW} key={`map-${key}`}>
          <span className="font-mono text-form-hint text-fg">{key}</span>
          <span aria-hidden="true" className="text-center text-small-body text-faint">
            ←
          </span>
          <span className="min-w-0">
            <TriggerValueBadge fill value={path} />
          </span>
          <span className="text-badge text-faint">from event</span>
        </div>
      ))}
      {statics.map(([key, value]) => (
        <div className={MAPPING_ROW} key={`static-${key}`}>
          <span className="font-mono text-form-hint text-fg">{key}</span>
          <span aria-hidden="true" className="text-center text-small-body text-faint">
            =
          </span>
          <span className="min-w-0">
            <TriggerValueBadge fill value={formatAutomationInputValue(value)} />
          </span>
          <span className="text-badge text-faint">static</span>
        </div>
      ))}
    </div>
  );
}

interface TriggerRuleThenProps {
  trigger: AutomationTrigger;
  loopWorkspaceName: string | null;
}

/**
 * Then splits on the immutable target kind: an agent receives a rendered
 * prompt; a loop receives labeled input rows and owns the run from there.
 */
export function TriggerRuleThen({ trigger, loopWorkspaceName }: TriggerRuleThenProps) {
  const target = projectAutomationTarget(trigger);

  if (target.kind === "loop") {
    const workspaceLabel = loopWorkspaceName ?? target.workspaceId;
    return (
      <>
        <span className="flex min-w-0 items-center gap-2">
          <TargetChip>
            <Workflow className="size-3" strokeWidth={1.75} />
          </TargetChip>
          <b className="min-w-0 truncate text-modal-title font-medium text-fg-strong">
            <Link
              className="rounded-xs transition-colors duration-base ease-out hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
              data-testid="trigger-loop-link"
              params={{ name: target.loopName }}
              search={target.workspaceId ? { workspace: target.workspaceId } : {}}
              to="/loops/$name"
            >
              {target.loopName}
            </Link>
          </b>
        </span>
        <span className="mt-1.5 text-small-body leading-relaxed text-muted">
          {`Starts a loop run in workspace ${workspaceLabel}. Event fields fill declared inputs; the rest are static.`}
        </span>
        <TriggerLoopMapping target={target} />
      </>
    );
  }

  return (
    <>
      <span className="flex min-w-0 items-center gap-2">
        <TargetChip>{target.agentName.charAt(0)}</TargetChip>
        <b className="min-w-0 truncate text-modal-title font-medium text-fg-strong">
          {target.agentName}
        </b>
      </span>
      <span className="mt-1.5 text-small-body leading-relaxed text-muted">
        {trigger.event === "webhook"
          ? "The agent receives a prompt built from the delivery."
          : "The agent receives a prompt built from the event."}
      </span>
      {target.prompt.trim() === "" ? null : <TriggerPromptPreview prompt={target.prompt} />}
    </>
  );
}
