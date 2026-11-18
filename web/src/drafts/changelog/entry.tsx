/** Version, date and flags for one release. */
import { formatDate, hasBreaking, type Release } from "./releases";

export function ReleaseMeta({ release }: { release: Release }) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 lg:flex-col lg:items-start">
      <span className="font-mono text-[14px] text-fg">{release.version}</span>
      <span className="text-[14px] text-subtle">{formatDate(release.date)}</span>
      {release.pre ? <span className="text-[13px] text-subtle">Beta</span> : null}
      {hasBreaking(release) ? (
        <span className="text-[13px] text-warning">Breaking changes</span>
      ) : null}
    </div>
  );
}
