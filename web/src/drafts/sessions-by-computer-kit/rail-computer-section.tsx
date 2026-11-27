import {
  Ban,
  RefreshCw,
  Sparkles,
  TriangleAlert,
  WifiOff,
  type LucideIcon,
} from "@compozy/ui/icons";
import { use, type ReactNode } from "react";

import { Button, Icon, Spinner, cn } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import {
  ScopedComputerError,
  ScopedComputerSkeleton,
} from "@/systems/computers/components/scoped-computer-status";
import { SessionListCreateMenu } from "@/systems/session/components/session-list/session-list-create-menu";
import { GroupNote } from "@/systems/session/components/session-list/session-list-group-note";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import type { SessionListGroupBy } from "@/systems/session/lib/session-list-preferences";

import { useFold } from "./hooks/use-fold";
import { leadingOf, type RailComputer } from "./model";
import { RailLookContext } from "./rail-look";
import { FlatRows, ProjectTree } from "./rail-tree";

const noop = () => undefined;
const smallAction = "h-6 shrink-0 px-2 text-badge";

function Note({
  icon,
  tone = "subtle",
  action,
  children,
}: {
  icon?: LucideIcon;
  tone?: "subtle" | "warning";
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <GroupNote>
      {icon ? (
        <Icon
          as={icon}
          className={tone === "warning" ? "shrink-0 text-warning" : "shrink-0 text-faint"}
          size="sm"
        />
      ) : null}
      <span className="min-w-0 flex-1">{children}</span>
      {action}
    </GroupNote>
  );
}

/** A computer's list, or why it has none: loading, failed, offline, plan, version or empty. */
export function ComputerBody({
  computer,
  groupBy,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
}) {
  const { name } = computer;
  switch (computer.read) {
    case "connecting":
      return (
        <ScopedComputerSkeleton className="mx-1" computerId={computer.key} computerName={name} />
      );
    case "failed":
      return (
        <ScopedComputerError
          className="mx-1"
          computerId={computer.key}
          computerName={name}
          onRetry={noop}
        />
      );
    case "offline_empty":
      return <Note icon={WifiOff}>{name} is offline. Its sessions show when it reconnects.</Note>;
    case "needs_pro":
      return (
        <Note
          action={
            <Button className={smallAction} size="xs" type="button" variant="secondary">
              Upgrade
            </Button>
          }
          icon={Sparkles}
        >
          Pro lists sessions from more than one remote computer.
        </Note>
      );
    case "incompatible":
      return (
        <Note
          action={
            <Button className={smallAction} size="xs" type="button" variant="secondary">
              Details
            </Button>
          }
          icon={TriangleAlert}
          tone="warning"
        >
          Update CompozyOS on {name} to list its sessions.
        </Note>
      );
    default:
      break;
  }
  if (computer.sessions.length === 0) return <Note>No sessions on {name} yet.</Note>;
  const rows =
    groupBy === "project" ? (
      <ProjectTree computer={computer} />
    ) : (
      <FlatRows sessions={computer.sessions} />
    );
  return (
    <div className={cn("flex flex-col gap-0.5", computer.read === "offline" && "opacity-70")}>
      {rows}
      {computer.more ? (
        <GroupNote>
          <Button size="sm" type="button" variant="ghost">
            Next sessions
          </Button>
        </GroupNote>
      ) : null}
    </div>
  );
}

function ComputerIdentity({ computer }: { computer: RailComputer }) {
  const quiet =
    computer.read === "offline" ||
    computer.read === "offline_empty" ||
    computer.read === "needs_pro";
  return (
    <span className="flex min-w-0 items-center gap-2">
      <ComputerPlatformIcon
        className="size-3.5 shrink-0"
        iconKind={computer.computer.iconKind}
        platform={computer.computer.platform}
        size="sm"
        tone="muted"
      />
      <span className="truncate text-small-body font-medium text-fg">{computer.name}</span>
      <span
        className={cn(
          "truncate text-eyebrow",
          computer.read === "incompatible" ? "text-warning" : quiet ? "text-faint" : "text-subtle"
        )}
      >
        {computer.detail}
      </span>
    </span>
  );
}

/** The header's end: create where the computer runs agents, Reconnect where it is away. */
function ComputerAction({
  computer,
  groupBy,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
}) {
  if (computer.read === "connecting") return <Spinner className="mr-1.5 size-3 text-faint" />;
  if (computer.read === "offline" || computer.read === "offline_empty") {
    return (
      <Button className={smallAction} size="xs" type="button" variant="secondary">
        <RefreshCw aria-hidden="true" className="size-3" />
        Reconnect
      </Button>
    );
  }
  if (computer.read !== "ok") return null;
  return (
    <SessionListCreateMenu
      className="text-subtle"
      folderLabel="New project folder"
      label={{
        session: `New session on ${computer.name}`,
        menu: `New session or project folder on ${computer.name}`,
      }}
      newFolderUnavailable={groupBy === "project" ? null : "Group by Project to add folders."}
      onNewFolder={groupBy === "project" ? noop : undefined}
      onNewSession={noop}
      size="icon-xs"
      testId={`draft-computer-${computer.key}`}
      variant="quiet"
    />
  );
}

/** One computer's section: its header, then its own list with the window's grouping inside. */
export function ComputerSection({
  computer,
  groupBy,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
}) {
  const look = use(RailLookContext);
  const [open, toggle] = useFold(`computer:${computer.key}`);
  const refused = look.refusedOn === computer.key;
  const counted = computer.read === "ok" || computer.read === "offline";
  return (
    <section
      className={cn(
        "flex flex-col gap-0.5 rounded-md",
        refused && "cursor-not-allowed bg-surface-2/60"
      )}
      data-testid={`draft-computer-${computer.key}`}
    >
      <div className="sticky top-0 z-10 flex min-h-8 min-w-0 items-center gap-1 rounded-sm bg-sunken">
        <SessionListGroupToggle
          collapsed={!open}
          count={counted ? computer.sessions.length : null}
          identity={<ComputerIdentity computer={computer} />}
          leading={leadingOf(computer.sessions)}
          onToggle={toggle}
        />
        <ComputerAction computer={computer} groupBy={groupBy} />
      </div>
      {refused ? <Note icon={Ban}>Sessions stay on the computer that runs them.</Note> : null}
      {look.query !== "" && computer.read === "offline" ? (
        <Note icon={WifiOff}>Matches from the rows seen before it went offline.</Note>
      ) : null}
      {open ? (
        <div className="pl-2.5">
          <ComputerBody computer={computer} groupBy={groupBy} />
        </div>
      ) : null}
    </section>
  );
}
