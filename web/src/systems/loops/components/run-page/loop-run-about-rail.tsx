import type { ComponentProps } from "react";
import { Link } from "@tanstack/react-router";

import { cn, Eyebrow, MonoId, PropertyRow } from "@agh/ui";

import type { LoopRunInputRow } from "../../lib/loop-run-about";
import type { LoopRunRecord } from "../../types";

interface LoopRunAboutRailProps extends ComponentProps<"div"> {
  run: LoopRunRecord;
  /** `v3 · pinned` (executed definition) or `v3` (loop fallback); omitted while loading. */
  versionLabel?: string;
  inputRows: LoopRunInputRow[];
  /** Humanized `started_origin_kind/ref`. */
  startedBy: string;
  /** Workspace display name, falling back to the raw id. */
  workspaceLabel: string;
}

/** Two-letter avatar seed for an agent-bound input value. */
function avatarSeed(value: string): string {
  const parts = value.split(/[^a-z0-9]+/i).filter(Boolean);
  const seed = parts.length >= 2 ? `${parts[0][0]}${parts[1][0]}` : (parts[0] ?? "ag").slice(0, 2);
  return seed.toUpperCase();
}

/**
 * The About rail section (§2/§3): loop link, pinned version, the watched
 * subject's input rows, the humanized start origin, workspace, and run id —
 * durable descriptors only, all from the run projection.
 */
export function LoopRunAboutRail({
  run,
  versionLabel,
  inputRows,
  startedBy,
  workspaceLabel,
  className,
  ...props
}: LoopRunAboutRailProps) {
  return (
    <div
      className={cn("border-t border-line-soft px-4.5 py-4", className)}
      data-testid="loop-run-about"
      {...props}
    >
      <Eyebrow className="mb-2 block text-subtle">About this run</Eyebrow>
      <PropertyRow label="Loop">
        <Link
          className="inline-flex min-h-6 items-center truncate rounded-xs text-small-body font-medium text-fg hover:text-fg-strong focus-visible:outline-none focus-visible:shadow-focus-ring"
          data-testid="loop-run-about-loop"
          params={{ name: run.loop_name }}
          to="/loops/$name"
        >
          {run.loop_name}
        </Link>
      </PropertyRow>
      {versionLabel !== undefined ? (
        <PropertyRow label="Version" mono data-testid="loop-run-about-version">
          {versionLabel}
        </PropertyRow>
      ) : null}
      {inputRows.map(row =>
        row.isAgent ? (
          <PropertyRow key={row.key} label={row.label} data-testid={`loop-run-input-${row.key}`}>
            <span
              aria-hidden="true"
              className="grid size-4 shrink-0 place-items-center rounded-xs bg-badge-fill font-mono text-pill-group-badge font-semibold text-muted"
            >
              {avatarSeed(row.value)}
            </span>
            <span className="truncate">{row.value}</span>
          </PropertyRow>
        ) : (
          <PropertyRow
            key={row.key}
            label={row.label}
            mono
            data-testid={`loop-run-input-${row.key}`}
          >
            {row.value}
          </PropertyRow>
        )
      )}
      <PropertyRow label="Started by" data-testid="loop-run-about-started-by">
        {startedBy}
      </PropertyRow>
      <PropertyRow label="Workspace" data-testid="loop-run-about-workspace">
        {workspaceLabel}
      </PropertyRow>
      <PropertyRow label="Run id" mono>
        <MonoId
          className="text-muted"
          copy
          copyLabel="Copy run id"
          data-testid="loop-run-about-id"
          value={run.id}
        />
      </PropertyRow>
    </div>
  );
}
