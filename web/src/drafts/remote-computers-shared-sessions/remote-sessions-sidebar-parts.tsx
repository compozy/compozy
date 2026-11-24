import { RefreshCw, TriangleAlert } from "@compozy/ui/icons";

import {
  Button,
  cn,
  Skeleton,
  SkeletonRegion,
  Spinner,
  StateGlyph,
  type StateGlyphState,
} from "@compozy/ui";

import { ComputerMetaLabel } from "@/systems/computers/components/computer-chip";
import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import type {
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import type { RemoteSessionRowEntry } from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import {
  SESSION_ROW_END_SLOT_FRAME,
  SESSION_ROW_FRAME,
  SESSION_ROW_META_FRAME,
  SESSION_ROW_TARGET_FRAME,
  SESSION_ROW_WHO_FRAME,
} from "@/systems/session/components/session-list/session-list-row-frame";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

export interface PerComputerLoadingBlock {
  computerId: string;
  computerName: string;
  iconKind: ComputerIconKind;
  platform: ComputerPlatform;
  rows: number;
}

export interface PerComputerErrorBlock {
  computerId: string;
  computerName: string;
  message?: string;
}

export interface PerComputerPaginationBlock {
  computerId: string;
  computerName: string;
  remainingCount: number;
}

function glyphStateFor(entry: RemoteSessionRowEntry): StateGlyphState {
  if (entry.computerOffline) return "idle";
  if (entry.state === "running") return "running";
  if (entry.state === "attention") return "attention";
  if (entry.state === "done") return "done";
  return "idle";
}

export function RemoteSessionRow({
  entry,
  current,
  hideProject = false,
  hideComputer = false,
  onSelect,
}: {
  entry: RemoteSessionRowEntry;
  current: boolean;
  hideProject?: boolean;
  hideComputer?: boolean;
  onSelect: () => void;
}) {
  const isOffline = Boolean(entry.computerOffline);
  const isAttention = entry.state === "attention" && !isOffline;

  return (
    <div
      className={cn(
        SESSION_ROW_FRAME,
        "group/session-row transition-colors hover:bg-surface-2 has-[[data-slot=session-row-target]:focus-visible]:shadow-focus-ring",
        current && "bg-selected",
        isOffline && "opacity-60 hover:opacity-80"
      )}
      data-computer-id={entry.computerId}
      data-offline={isOffline ? "true" : undefined}
      data-slot="remote-session-row"
    >
      <button
        aria-current={current ? "true" : undefined}
        className={cn(
          SESSION_ROW_TARGET_FRAME,
          "cursor-pointer outline-none after:absolute after:inset-0 after:rounded-md",
          (current || isAttention) && "font-medium"
        )}
        data-session-id={entry.id}
        data-slot="session-row-target"
        onClick={onSelect}
        title={
          entry.isLocal
            ? `${entry.title} · ${entry.project}`
            : `${entry.title} · ${entry.project} on ${entry.computerName}${isOffline ? " (Offline)" : ""}`
        }
        type="button"
      >
        <span className="grid size-3 shrink-0 place-items-center">
          <StateGlyph size="sm" state={glyphStateFor(entry)} />
        </span>
        <span className="min-w-0 truncate">{entry.title}</span>
      </button>

      <span className={SESSION_ROW_END_SLOT_FRAME}>
        <span className="font-mono text-micro text-subtle tabular-nums">{entry.updatedLabel}</span>
      </span>

      <RemoteSessionRowMeta entry={entry} hideComputer={hideComputer} hideProject={hideProject} />
    </div>
  );
}

function RemoteSessionRowMeta({
  entry,
  hideComputer,
  hideProject,
}: {
  entry: RemoteSessionRowEntry;
  hideComputer: boolean;
  hideProject: boolean;
}) {
  const isOffline = Boolean(entry.computerOffline);
  const attention = entry.state === "attention" && !isOffline ? entry.needsYouPhrase : undefined;
  const showComputer = !entry.isLocal && !hideComputer;
  const showModel = !attention && (entry.isLocal || hideComputer);
  const separator = (
    <span aria-hidden="true" className="text-faint">
      ·
    </span>
  );

  return (
    <span className={SESSION_ROW_META_FRAME}>
      <span className={SESSION_ROW_WHO_FRAME}>
        {!hideProject && (
          <WorkspaceBadge
            className="max-w-24 shrink-[1000]"
            identity={{ kind: "project", name: entry.project }}
          />
        )}
        {showComputer && (
          <>
            {!hideProject && separator}
            <ComputerMetaLabel
              className="shrink-0"
              iconKind={entry.iconKind}
              name={entry.computerName}
              offline={isOffline}
              platform={entry.platform}
            />
          </>
        )}
        {isOffline && hideComputer && (
          <span className="shrink-0 text-micro text-faint">Offline</span>
        )}
        {attention && (
          <>
            {separator}
            <span className="min-w-0 truncate text-attn">{attention}</span>
          </>
        )}
        {showModel && (
          <>
            {(!hideProject || isOffline) && separator}
            <span className="min-w-0 truncate text-fg-2">{entry.model}</span>
          </>
        )}
      </span>
    </span>
  );
}

const SKELETON_TITLE_WIDTHS = ["w-40", "w-32", "w-36"] as const;
const SKELETON_META_WIDTHS = ["w-24", "w-20", "w-28"] as const;

export function ScopedComputerSkeleton({ block }: { block: PerComputerLoadingBlock }) {
  const rowCount = Math.max(2, block.rows);
  return (
    <section
      className="mt-1.5 flex flex-col gap-0.5"
      data-testid={`computer-loading-${block.computerId}`}
    >
      <div className="flex items-center gap-1.5 px-2 py-1 text-eyebrow text-muted-foreground">
        <ComputerPlatformIcon
          className="size-3 shrink-0"
          iconKind={block.iconKind}
          platform={block.platform}
          size="sm"
          tone="muted"
        />
        <span className="font-medium text-fg-2">{block.computerName}</span>
        <Spinner aria-hidden="true" className="ml-auto size-3 text-subtle" />
      </div>
      <SkeletonRegion
        className="flex flex-col gap-0.5 pl-1"
        label={`Loading sessions from ${block.computerName}`}
      >
        {Array.from({ length: rowCount }, (_, idx) => (
          <div aria-hidden="true" className={SESSION_ROW_FRAME} key={idx}>
            <span className={SESSION_ROW_TARGET_FRAME}>
              <span className="grid size-3 shrink-0 place-items-center">
                <Skeleton className="size-2.5 animate-none rounded-full" />
              </span>
              <Skeleton
                className={cn(
                  "h-3 animate-none rounded-sm",
                  SKELETON_TITLE_WIDTHS[idx % SKELETON_TITLE_WIDTHS.length]
                )}
              />
            </span>
            <span className={SESSION_ROW_END_SLOT_FRAME}>
              <Skeleton className="h-2.5 w-6 animate-none rounded-sm" />
            </span>
            <span className={SESSION_ROW_META_FRAME}>
              <span className={SESSION_ROW_WHO_FRAME}>
                <Skeleton
                  className={cn(
                    "h-2.5 animate-none rounded-sm",
                    SKELETON_META_WIDTHS[idx % SKELETON_META_WIDTHS.length]
                  )}
                />
              </span>
            </span>
          </div>
        ))}
      </SkeletonRegion>
    </section>
  );
}

export function ScopedComputerError({
  block,
  onRetry,
  retrying,
}: {
  block: PerComputerErrorBlock;
  onRetry: () => void;
  retrying: boolean;
}) {
  return (
    <div
      className="mt-1.5 flex items-center gap-2 rounded-md bg-warning-tint px-2.5 py-1.5 text-eyebrow text-fg"
      data-testid={`computer-error-${block.computerId}`}
      role="status"
    >
      <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
      <span className="min-w-0 flex-1 truncate text-fg-2">
        {block.message ?? `Couldn't load sessions from ${block.computerName}`}
      </span>
      <Button
        className="h-6 shrink-0 px-2 text-badge"
        disabled={retrying}
        onClick={onRetry}
        size="xs"
        type="button"
        variant="secondary"
      >
        {retrying ? (
          <Spinner aria-hidden="true" className="size-3" />
        ) : (
          <RefreshCw aria-hidden="true" className="size-3" />
        )}
        {retrying ? "Retrying" : "Retry"}
      </Button>
    </div>
  );
}
