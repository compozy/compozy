import { Button } from "@compozy/ui";

import {
  ScopedComputerError,
  ScopedComputerSkeleton,
  type ScopedComputerFailureItem,
} from "@/systems/computers";

import type { RemoteSessionsFanoutResult } from "../../hooks/use-remote-sessions-fanout";
import { SessionListComputerContext } from "../../lib/session-list-computer";
import { buildSessionListThreads } from "../../lib/session-list-threads";
import type { GroupRowProps } from "./session-list-group-body";
import { SessionListGroupToggle } from "./session-list-group-toggle";
import { SessionListThreadList } from "./session-list-thread-list";

export function SessionListRemoteGroups({
  remote,
  view,
  onSelectSession,
  ...rows
}: GroupRowProps & { remote: RemoteSessionsFanoutResult }) {
  return (
    <SessionListComputerContext value={remote.computerBySessionId}>
      {remote.projectComputerGroups.map(group => {
        const collapsed = view.collapsedGroupKeys.has(group.key);
        return (
          <section key={group.key} data-group-key={group.key}>
            <SessionListGroupToggle
              collapsed={collapsed}
              count={null}
              identity={<span className="truncate text-small-body font-medium">{group.label}</span>}
              onToggle={() => view.toggleGroup(group.key)}
            />
            {collapsed ? null : (
              <SessionListThreadList
                {...rows}
                emptyMessage={null}
                onSelectSession={session =>
                  onSelectSession({ ...session, computerId: group.computerId })
                }
                readOnly
                threads={buildSessionListThreads(
                  group.rows.map(row => row.session),
                  ""
                )}
              />
            )}
          </section>
        );
      })}
      <SessionListRemoteFooter
        {...remote}
        onLoadMore={remote.loadMore}
        testIdPrefix={rows.testIdPrefix}
      />
    </SessionListComputerContext>
  );
}

/** Per-computer loading skeletons, inline retryable failures, and oldest-cursor Load more. */
export function SessionListRemoteFooter({
  loadingComputers,
  failedComputers,
  hasMore,
  loadingMore,
  onLoadMore,
  testIdPrefix,
}: {
  loadingComputers: readonly { id: string; name: string }[];
  failedComputers: readonly ScopedComputerFailureItem[];
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  testIdPrefix: string;
}) {
  return (
    <>
      {loadingComputers.map(computer => (
        <ScopedComputerSkeleton
          computerId={computer.id}
          computerName={computer.name}
          key={computer.id}
        />
      ))}
      {failedComputers.map(computer => (
        <ScopedComputerError
          computerId={computer.id}
          computerName={computer.name}
          key={computer.id}
          onRetry={computer.retry}
        />
      ))}
      {hasMore ? (
        <div className="px-1 py-1.5">
          <Button
            className="w-full"
            data-testid={`${testIdPrefix}-remote-load-more`}
            disabled={loadingMore}
            onClick={onLoadMore}
            size="xs"
            type="button"
            variant="secondary"
          >
            {loadingMore ? "Loading…" : "Load more"}
          </Button>
        </div>
      ) : null}
    </>
  );
}
