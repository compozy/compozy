import type { ReactNode } from "react";

import { Panel, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import type { AuthoredFileKind } from "../hooks/use-agent-authored-file-editor";

function AuthoredFilePanelContent() {
  return (
    <Panel bodyClassName="p-0">
      <div className="flex flex-wrap items-center gap-2 border-b border-line-soft px-4 py-2.5">
        <span className="text-eyebrow">
          <SkeletonText className="w-16" />
        </span>
        <span className="font-mono text-badge">
          <SkeletonText className="w-40" />
        </span>
      </div>
      <Skeleton className="min-h-64 rounded-none border-b border-line-soft" />
      <div className="flex flex-wrap items-center gap-2 px-4 py-3">
        <Skeleton className="h-button-sm w-20" />
        <span className="min-w-0 flex-1" />
        <Skeleton className="h-button-sm w-16" />
        <Skeleton className="h-button-sm w-14" />
      </div>
    </Panel>
  );
}

/** The editor panel for a page that owns one status region around everything it loads. */
export function AuthoredFilePanelPlaceholder() {
  return (
    <div aria-hidden="true">
      <AuthoredFilePanelContent />
    </div>
  );
}

/**
 * The editor panel while the authored file loads: the real panel with its status row, the editor
 * at its minimum height, and the action row. `headerSlot` (the heartbeat status card) stays real
 * above it, outside the busy region.
 */
export function AuthoredFileLoading({
  kind,
  headerSlot,
}: {
  kind: AuthoredFileKind;
  headerSlot?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4">
      {headerSlot}
      <SkeletonRegion
        data-testid={`agent-${kind}-loading`}
        label={kind === "soul" ? "Loading SOUL.md" : "Loading HEARTBEAT.md"}
      >
        <AuthoredFilePanelContent />
      </SkeletonRegion>
    </div>
  );
}
