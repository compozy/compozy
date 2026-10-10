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
    "size-3.5 shrink-0 text-faint transition-colors duration-base group-hover/tool-row:text-subtle";
  if (icon === undefined) {
    return <WrenchIcon aria-hidden="true" data-slot="tool-call-row-icon" className={iconClass} />;
  }
  if (isIconComponent(icon)) {
    const IconComp = icon;
    return <IconComp aria-hidden="true" data-slot="tool-call-row-icon" className={iconClass} />;
  }
  return icon;
}

const LINE_CLASS = "min-h-6 gap-1.5 rounded-md px-1 text-small-body";

/** Trailing affordances stay out of the resting read: they surface on hover, focus, or open. */
const REVEAL_CLASS =
  "opacity-0 transition-opacity duration-base ease-out group-hover/tool-row:opacity-100 group-focus-within/tool-row:opacity-100 motion-reduce:transition-none";

interface ToolCallRowLineProps {
  toolName: React.ReactNode;
  toolNameId: string;
  preview?: React.ReactNode;
  previewVariant: NonNullable<ToolCallRowProps["previewVariant"]>;
  icon: ToolCallRowProps["icon"];
  stat?: React.ReactNode;
  actions?: React.ReactNode;
  status: ToolCallStatus;
  expandable: boolean;
  isExpanded: boolean;
}

function ToolCallRowPreview({
  preview,
  variant,
}: {
  preview: React.ReactNode;
  variant: ToolCallRowLineProps["previewVariant"];
}) {
  return (
    <span
      data-slot="tool-call-row-preview"
      data-variant={variant}
      className={cn(
        "min-w-0 truncate",
        variant === "chip"
          ? "rounded-xs bg-hover px-1.5 py-px font-mono text-transcript-body text-muted transition-colors duration-base group-hover/tool-row:text-fg"
          : variant === "code"
            ? "font-mono text-transcript-body text-subtle"
            : "text-subtle"
      )}
      title={nativeTitle(preview)}
    >
      {preview}
    </span>
  );
}

/**
 * The single line: `[icon well] [verb] [object] [diff stat] [actions] [chevron] [status glyph]`.
 * The line hugs its content so the trailing affordances sit beside the words,
 * never at the far edge of a wide transcript column.
 */
export function ToolCallRowLine({
  toolName,
  toolNameId,
  preview,
  previewVariant,
  icon,
  stat,
  actions,
  status,
  expandable,
  isExpanded,
}: ToolCallRowLineProps) {
  return (
    <>
      {icon === null ? null : (
        <span
          data-slot="tool-call-row-icon-well"
          className="flex size-5 shrink-0 items-center justify-center rounded-xs"
        >
          {renderToolCallIcon(icon)}
        </span>
      )}
      <span className="flex min-w-0 items-center gap-1.5">
        <span
          id={toolNameId}
          data-slot="tool-call-row-tool"
          className="shrink-0 whitespace-nowrap text-subtle transition-colors duration-base group-hover/tool-row:text-muted"
          title={nativeTitle(toolName)}
        >
          {toolName}
        </span>
        {preview ? <ToolCallRowPreview preview={preview} variant={previewVariant} /> : null}
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
            className={cn("relative z-10 flex shrink-0 items-center", REVEAL_CLASS)}
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
              "size-3 shrink-0 text-subtle transition-[transform,opacity] duration-base ease-out motion-reduce:transition-none",
              isExpanded ? "rotate-90 text-muted opacity-100" : REVEAL_CLASS
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
        "relative flex w-fit max-w-full min-w-0 cursor-pointer items-center text-left",
        LINE_CLASS
      )}
    >
      <button
        type="button"
        data-slot="tool-call-row-trigger"
        aria-expanded={isExpanded}
        aria-labelledby={`${toolNameId}${statLabelledBy} ${triggerDescriptionId}`}
        className="absolute inset-0 rounded-md outline-none transition-colors duration-base ease-out hover:bg-chat-fill-user focus-visible:shadow-focus-inset"
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
      className={cn("flex w-fit max-w-full min-w-0 items-center", LINE_CLASS)}
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
      className="mt-1 mb-1.5 ml-7 flex max-h-80 min-w-0 cursor-default flex-col gap-2.5 overflow-auto rounded-lg border border-line-soft bg-chat-fill-code p-2.5 text-small-body text-muted select-text"
      onClick={event => event.stopPropagation()}
      onPointerDown={event => event.stopPropagation()}
    >
      {errorMessage ? (
        <p
          data-slot="tool-call-row-error"
          className="font-mono text-transcript-caption leading-prose break-words whitespace-pre-wrap text-muted"
        >
          {errorMessage}
        </p>
      ) : null}
      {children}
    </div>
  );
}
