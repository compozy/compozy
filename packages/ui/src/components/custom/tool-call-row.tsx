"use client";

import { ChevronRight, TerminalIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { ToolCallStatusIcon } from "./tool-call-status-icon";
import { ToolCallRowSection, type ToolCallRowSectionProps } from "./tool-call-row-section";

export type ToolCallStatus = "pending" | "running" | "failed" | "success" | "empty";

type ToolCallIconComponent = React.ComponentType<{
  className?: string;
  strokeWidth?: number;
}>;

export interface ToolCallRowProps extends Omit<React.ComponentProps<"div">, "title"> {
  toolName: React.ReactNode;
  /** Mono, truncated summary shown after the heading (command, path, pattern…). */
  preview?: React.ReactNode;
  status: ToolCallStatus;
  icon?: ToolCallIconComponent | React.ReactNode;
  errorMessage?: React.ReactNode;
  /** Per-file diff stat (+a −d) rendered between the text and the trailing glyphs. */
  stat?: React.ReactNode;
  /** Accessible description for `stat`, for example "28 additions, 104 deletions". */
  statLabel?: string;
  /** Trailing affordances rendered beside chevron/status (e.g. copy). */
  actions?: React.ReactNode;
  expanded?: boolean;
  defaultExpanded?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  children?: React.ReactNode;
}

function isIconComponent(value: unknown): value is ToolCallIconComponent {
  if (typeof value === "function") return true;
  if (typeof value === "object" && value !== null && "render" in value) return true;
  return false;
}

function nativeTitle(value: React.ReactNode): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function renderToolCallIcon(icon: ToolCallRowProps["icon"]): React.ReactNode {
  const iconClass =
    "size-3.5 shrink-0 text-subtle transition-colors group-hover/tool-row:text-muted";
  if (icon === undefined) {
    return (
      <TerminalIcon
        aria-hidden="true"
        data-slot="tool-call-row-icon"
        className={iconClass}
        strokeWidth={1.75}
      />
    );
  }
  if (isIconComponent(icon)) {
    const IconComp = icon;
    return (
      <IconComp
        aria-hidden="true"
        data-slot="tool-call-row-icon"
        className={iconClass}
        strokeWidth={1.75}
      />
    );
  }
  return icon;
}

function ToolCallRowInput(props: ToolCallRowSectionProps) {
  return <ToolCallRowSection slot="input" label="Input" {...props} />;
}

function ToolCallRowOutput(props: ToolCallRowSectionProps) {
  return <ToolCallRowSection slot="output" label="Output" {...props} />;
}

/**
 * `ToolCallRow` renders one tool call as a single ~24px line —
 * `[icon well] [verb] [mono preview] [diff stat] [chevron] [status glyph]` —
 * that expands an inline indented body (params/outputs) on click or
 * Enter/Space. Calm-transcript grammar: no tinted wells, row text never
 * changes color on failure — status lives in the trailing glyph alone (grey
 * check, red ×, grey spinner).
 */
function ToolCallRowInner({
  toolName,
  preview,
  status,
  icon,
  errorMessage,
  stat,
  statLabel,
  actions,
  expanded,
  defaultExpanded = false,
  onExpandedChange,
  children,
  className,
  ...props
}: ToolCallRowProps) {
  const [localExpanded, setLocalExpanded] = React.useState(defaultExpanded);
  const toolNameId = React.useId();
  const statDescriptionId = React.useId();
  const triggerDescriptionId = React.useId();
  const isExpanded = expanded ?? localExpanded;
  const expandable = Boolean(errorMessage) || React.Children.toArray(children).length > 0;
  const accessibleStatLabel = stat ? statLabel : undefined;
  const iconContent = renderToolCallIcon(icon);

  const setExpanded = (next: boolean) => {
    if (expanded === undefined) {
      setLocalExpanded(next);
    }
    onExpandedChange?.(next);
  };

  const toggle = () => {
    if (!expandable) return;
    setExpanded(!isExpanded);
  };

  const rowContent = (
    <>
      <span
        data-slot="tool-call-row-icon-well"
        className="flex size-5 shrink-0 items-center justify-center rounded-xs"
      >
        {iconContent}
      </span>
      <span className="flex min-w-0 flex-1 items-baseline gap-1.5">
        <span
          id={toolNameId}
          data-slot="tool-call-row-tool"
          className="min-w-0 shrink truncate font-medium text-muted transition-colors group-hover/tool-row:text-fg"
          title={nativeTitle(toolName)}
        >
          {toolName}
        </span>
        {preview ? (
          <span
            data-slot="tool-call-row-preview"
            className="min-w-0 flex-1 truncate font-mono text-subtle"
            title={nativeTitle(preview)}
          >
            {preview}
          </span>
        ) : (
          <span className="min-w-0 flex-1" />
        )}
      </span>
      {stat ? (
        <span
          data-slot="tool-call-row-stat"
          className="flex shrink-0 items-center gap-1 font-mono text-transcript-caption tabular-nums"
        >
          {stat}
        </span>
      ) : null}
      <span className="flex shrink-0 items-center gap-1 text-subtle">
        {actions ? (
          <span
            data-slot="tool-call-row-actions"
            className="relative z-10 flex shrink-0 items-center"
            onClick={event => event.stopPropagation()}
            onKeyDown={event => event.stopPropagation()}
            onPointerDown={event => event.stopPropagation()}
          >
            {actions}
          </span>
        ) : null}
        {expandable ? (
          <ChevronRight
            aria-hidden="true"
            data-slot="tool-call-row-chevron"
            className={cn(
              "size-3 shrink-0 text-subtle transition-transform duration-base ease-out motion-reduce:transition-none",
              isExpanded ? "rotate-90 text-muted" : null
            )}
            strokeWidth={1.75}
          />
        ) : null}
        <ToolCallStatusIcon status={status} />
      </span>
    </>
  );

  return (
    <div
      data-slot="tool-call-row"
      data-status={status}
      data-expanded={expandable ? String(isExpanded) : undefined}
      className={cn("group/tool-row min-w-0", className)}
      {...props}
    >
      {expandable ? (
        <div
          data-slot="tool-call-row-header"
          className={cn(
            "relative flex min-h-6 w-full min-w-0 cursor-pointer items-center gap-1.5 rounded-sm px-1 text-left text-small-body"
          )}
        >
          <button
            type="button"
            data-slot="tool-call-row-trigger"
            aria-expanded={isExpanded}
            aria-labelledby={`${toolNameId}${accessibleStatLabel ? ` ${statDescriptionId}` : ""} ${triggerDescriptionId}`}
            className="absolute inset-0 rounded-sm outline-none transition-colors duration-base ease-out hover:bg-hover focus-visible:shadow-focus-inset"
            onClick={toggle}
          />
          <span id={triggerDescriptionId} className="sr-only">
            Toggle tool call ({status})
          </span>
          {accessibleStatLabel ? (
            <span id={statDescriptionId} className="sr-only">
              {accessibleStatLabel}
            </span>
          ) : null}
          {rowContent}
        </div>
      ) : (
        <div
          data-slot="tool-call-row-static"
          className="flex min-h-6 w-full min-w-0 items-center gap-1.5 rounded-sm px-1 text-small-body"
        >
          {rowContent}
        </div>
      )}
      {expandable && isExpanded ? (
        <div
          data-slot="tool-call-row-body"
          className="mt-1 ml-7 flex max-h-64 min-w-0 cursor-default flex-col gap-2 overflow-auto border-l border-line pl-3 text-small-body text-muted select-text"
          onClick={event => event.stopPropagation()}
          onPointerDown={event => event.stopPropagation()}
        >
          {errorMessage ? (
            <p data-slot="tool-call-row-error" className="text-small-body text-muted">
              {errorMessage}
            </p>
          ) : null}
          {children}
        </div>
      ) : null}
    </div>
  );
}

const ToolCallRow = Object.assign(ToolCallRowInner, {
  Input: ToolCallRowInput,
  Output: ToolCallRowOutput,
});

export { ToolCallRow };
export type { ToolCallRowSectionProps } from "./tool-call-row-section";
