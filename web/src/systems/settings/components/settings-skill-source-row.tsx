import { ChevronDown, Folder, X } from "lucide-react";

import {
  Button,
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Pill,
  Switch,
} from "@compozy/ui";

import type { SkillSourceRootView, SkillSourceView } from "../lib/skill-sources-view";
import { SettingsSkillSourceDiagnostics } from "./settings-skill-source-diagnostics";

interface SettingsSkillSourceRowProps {
  source: SkillSourceView;
  disabled: boolean;
  onToggle?: (enabled: boolean) => void;
  onRemove?: () => void;
}

/**
 * Tone of a root's state sentence. An absent folder is a normal state and takes
 * no signal colour; a folder we could not read is a failure; a measured folder
 * only carries a sentence when the scan was cut short.
 */
const STATE_LABEL_TONE = {
  absent: "text-subtle",
  unreadable: "text-danger",
  measured: "text-warning",
  unavailable: "text-subtle",
} as const;

/**
 * One source, one line. Folder paths and their measured state live behind the
 * disclosure so a machine with a dozen sources still reads as a list rather than
 * a wall. Degraded state stays visible on the closed line.
 */
export function SettingsSkillSourceRow({
  source,
  disabled,
  onToggle,
  onRemove,
}: SettingsSkillSourceRowProps) {
  const testId = `settings-page-skills-source-${source.slug}`;
  return (
    <Collapsible
      className="flex min-w-0 flex-col border-b border-line last:border-b-0"
      data-testid={testId}
    >
      <div className="flex min-h-11 min-w-0 items-center gap-2 px-3">
        <CollapsibleTrigger
          className={cn(
            "group/skill-source flex min-w-0 flex-1 items-center gap-2 py-2 text-left",
            "rounded-sm focus-visible:shadow-focus-ring focus-visible:outline-none"
          )}
          data-testid={`${testId}-disclosure`}
          type="button"
        >
          <ChevronDown
            aria-hidden="true"
            className={cn(
              "size-3.5 shrink-0 -rotate-90 text-faint",
              "transition-transform duration-base group-data-panel-open/skill-source:rotate-0"
            )}
          />
          <span className="truncate text-sm text-fg">{source.label}</span>
          {source.isCustom ? <Pill size="xs">custom</Pill> : null}
          {source.hasUnreadableRoot ? (
            <Pill size="xs" tone="danger" data-testid={`${testId}-unreadable`}>
              can&rsquo;t read
            </Pill>
          ) : null}
          {source.hasTruncatedRoot ? (
            <Pill size="xs" tone="warning" data-testid={`${testId}-truncated`}>
              partial scan
            </Pill>
          ) : null}
        </CollapsibleTrigger>
        <div className="flex shrink-0 items-center gap-2">
          {source.totalLabel !== null ? (
            <Pill size="xs" data-testid={`${testId}-count`}>
              {source.totalLabel}
            </Pill>
          ) : null}
          {source.alwaysOn ? (
            <Pill size="xs" data-testid={`${testId}-always-on`}>
              always on
            </Pill>
          ) : null}
          {onToggle ? (
            <Switch
              aria-label={`Enable ${source.label}`}
              checked={source.enabled}
              data-testid={`${testId}-toggle`}
              disabled={disabled}
              onCheckedChange={onToggle}
            />
          ) : null}
          {onRemove ? (
            <Button
              aria-label={`Remove ${source.label}`}
              data-testid={`${testId}-remove`}
              disabled={disabled}
              onClick={onRemove}
              size="sm"
              type="button"
              variant="ghost"
            >
              <X aria-hidden="true" className="size-3" />
            </Button>
          ) : null}
        </div>
      </div>
      <CollapsibleContent>
        <div className="flex flex-col gap-0.5 px-3 pb-2">
          {source.roots.map(root => (
            <SkillSourceRootLine key={root.rootId} root={root} testId={testId} />
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function SkillSourceRootLine({ root, testId }: { root: SkillSourceRootView; testId: string }) {
  const rootTestId = `${testId}-root-${root.rootId}`;
  return (
    <div className="flex min-w-0 flex-col">
      <div className="flex min-w-0 items-baseline gap-2 py-0.5" data-testid={rootTestId}>
        <Folder aria-hidden="true" className="size-3 shrink-0 self-center text-faint" />
        <span className="truncate font-mono text-xs text-muted">{root.path}</span>
        <span aria-hidden="true" className="flex-1" />
        {root.stateLabel !== null ? (
          <span
            className={cn("shrink-0 text-form-hint", STATE_LABEL_TONE[root.state])}
            data-testid={`${rootTestId}-state`}
          >
            {root.stateLabel}
          </span>
        ) : null}
        {root.countLabel !== null ? (
          <span
            className="shrink-0 font-mono text-badge text-faint"
            data-testid={`${rootTestId}-count`}
          >
            {root.countLabel}
          </span>
        ) : null}
      </div>
      {root.diagnostics !== null ? (
        <SettingsSkillSourceDiagnostics
          diagnostics={root.diagnostics}
          testId={`${rootTestId}-diagnostics`}
        />
      ) : null}
    </div>
  );
}
