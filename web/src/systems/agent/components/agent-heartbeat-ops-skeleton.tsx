import { MetadataList, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

const POLICY_ROWS = [
  { label: "Enabled", width: "w-8" },
  { label: "Schedule", width: "w-48" },
  { label: "Recent wake", width: "w-40" },
] as const;

const HEARTBEAT_CARD_CLASS = "flex flex-col gap-4 rounded-lg bg-card p-4 shadow-card";

function HeartbeatOpsSkeletonContent() {
  return (
    <>
      <MetadataList>
        {POLICY_ROWS.map(row => (
          <MetadataList.Row key={row.label}>
            <MetadataList.Term>{row.label}</MetadataList.Term>
            <MetadataList.Value>
              <SkeletonText className={row.width} />
            </MetadataList.Value>
          </MetadataList.Row>
        ))}
      </MetadataList>
      <div className="flex flex-col gap-2">
        <p className="eyebrow text-muted-foreground">Wake target</p>
        <Skeleton className="h-input" />
      </div>
      <Skeleton className="h-button-sm w-24" />
    </>
  );
}

/** The heartbeat status card for a page that owns one status region around everything it loads. */
export function AgentHeartbeatOpsPlaceholder() {
  return (
    <div aria-hidden="true" className={HEARTBEAT_CARD_CLASS}>
      <HeartbeatOpsSkeletonContent />
    </div>
  );
}

/** The heartbeat status card while its status loads: the real card, policy terms and wake controls. */
export function AgentHeartbeatOpsSkeleton() {
  return (
    <SkeletonRegion
      className={HEARTBEAT_CARD_CLASS}
      data-testid="agent-heartbeat-ops-skeleton"
      label="Loading heartbeat status"
    >
      <HeartbeatOpsSkeletonContent />
    </SkeletonRegion>
  );
}
