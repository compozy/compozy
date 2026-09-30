"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import {
  ToolCallRowBody,
  ToolCallRowHeader,
  ToolCallRowLine,
  ToolCallRowStatic,
} from "./tool-call-row-parts";
import { ToolCallRowSection, type ToolCallRowSectionProps } from "./tool-call-row-section";

/**
 * `absorbed` is a failure the turn kept going past (subtle ×, the word carries
 * it); `stopped` is the call that was running when the operator stopped the turn
 * (no glyph — the word carries it). Both stay in the settled ink; only `failed`
 * earns the danger hue.
 */
export type ToolCallStatus =
  | "pending"
  | "running"
  | "failed"
  | "absorbed"
  | "stopped"
  | "success"
  | "empty";

export type ToolCallIconComponent = React.ComponentType<{
  className?: string;
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
  const isExpanded = expanded ?? localExpanded;
  const expandable = Boolean(errorMessage) || React.Children.toArray(children).length > 0;

  const toggle = () => {
    if (!expandable) return;
    if (expanded === undefined) {
      setLocalExpanded(!isExpanded);
    }
    onExpandedChange?.(!isExpanded);
  };

  const line = (
    <ToolCallRowLine
      actions={actions}
      expandable={expandable}
      icon={icon}
      isExpanded={isExpanded}
      preview={preview}
      stat={stat}
      status={status}
      toolName={toolName}
      toolNameId={toolNameId}
    />
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
        <ToolCallRowHeader
          isExpanded={isExpanded}
          onToggle={toggle}
          statLabel={stat ? statLabel : undefined}
          status={status}
          toolNameId={toolNameId}
        >
          {line}
        </ToolCallRowHeader>
      ) : (
        <ToolCallRowStatic>{line}</ToolCallRowStatic>
      )}
      {expandable && isExpanded ? (
        <ToolCallRowBody errorMessage={errorMessage}>{children}</ToolCallRowBody>
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
