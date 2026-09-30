"use client";

import { ChevronRight, WrenchIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import type { ToolCallIconComponent, ToolCallRowProps, ToolCallStatus } from "./tool-call-row";
import { ToolCallStatusIcon } from "./tool-call-status-icon";

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
    return <WrenchIcon aria-hidden="true" data-slot="tool-call-row-icon" className={iconClass} />;
  }
  if (isIconComponent(icon)) {
    const IconComp = icon;
    return <IconComp aria-hidden="true" data-slot="tool-call-row-icon" className={iconClass} />;
  }
  return icon;
}

const LINE_CLASS = "min-h-6 gap-1.5 rounded-sm px-1 text-small-body";

interface ToolCallRowLineProps {
  toolName: React.ReactNode;
  toolNameId: string;
  preview?: React.ReactNode;
  icon: ToolCallRowProps["icon"];
  stat?: React.ReactNode;
  actions?: React.ReactNode;
  status: ToolCallStatus;
  expandable: boolean;
  isExpanded: boolean;
}

/** The single line: `[icon well] [verb] [mono preview] [diff stat] [actions] [chevron] [status glyph]`. */
export function ToolCallRowLine({
  toolName,
  toolNameId,
  preview,
  icon,
  stat,
  actions,
  status,
  expandable,
  isExpanded,
}: ToolCallRowLineProps) {
  return (
    <>
      <span
        data-slot="tool-call-row-icon-well"
        className="flex size-5 shrink-0 items-center justify-center rounded-xs"
      >
        {renderToolCallIcon(icon)}
      </span>
      <span className="flex min-w-0 max-w-sm flex-1 items-baseline gap-1.5">
        <span
          id={toolNameId}
          data-slot="tool-call-row-tool"
          className="min-w-0 max-w-xs shrink truncate font-medium text-muted transition-colors group-hover/tool-row:text-fg"
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
          />
        ) : null}
        <ToolCallStatusIcon status={status} />
      </span>
    </>
  );
}

/** Expandable header: a full-bleed trigger button under the line, labelled by its parts. */
export function ToolCallRowHeader({
  isExpanded,
  status,
  statLabel,
  toolNameId,
  onToggle,
  children,
}: {
  isExpanded: boolean;
  status: ToolCallStatus;
  statLabel?: string;
  toolNameId: string;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  const statDescriptionId = React.useId();
  const triggerDescriptionId = React.useId();
  const statLabelledBy = statLabel ? ` ${statDescriptionId}` : "";
  return (
    <div
      data-slot="tool-call-row-header"
      className={cn(
        "relative flex w-full min-w-0 cursor-pointer items-center text-left",
        LINE_CLASS
      )}
    >
      <button
        type="button"
        data-slot="tool-call-row-trigger"
        aria-expanded={isExpanded}
        aria-labelledby={`${toolNameId}${statLabelledBy} ${triggerDescriptionId}`}
        className="absolute inset-0 rounded-sm outline-none transition-colors duration-base ease-out hover:bg-hover focus-visible:shadow-focus-inset"
        onClick={onToggle}
      />
      <span id={triggerDescriptionId} className="sr-only">
        Toggle tool call ({status})
      </span>
      {statLabel ? (
        <span id={statDescriptionId} className="sr-only">
          {statLabel}
        </span>
      ) : null}
      {children}
    </div>
  );
}

export function ToolCallRowStatic({ children }: { children: React.ReactNode }) {
  return (
    <div
      data-slot="tool-call-row-static"
      className={cn("flex w-full min-w-0 items-center", LINE_CLASS)}
    >
      {children}
    </div>
  );
}

export function ToolCallRowBody({
  errorMessage,
  children,
}: {
  errorMessage?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
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
  );
}
