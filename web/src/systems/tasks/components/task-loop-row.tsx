import { Link } from "@tanstack/react-router";
import { CircleSlash, GitBranch } from "lucide-react";

import { ListingRow, MonoId, Pill } from "@compozy/ui";

import {
  RUN_GONE_LABEL,
  taskLoopIdentityLabel,
  taskLoopRoleLabel,
  taskLoopRunLink,
  type TaskLoopProvenance,
} from "../lib/task-loop-identity";
import { taskStatusLabel, taskStatusTone } from "../lib/task-formatters";
import type { TaskListItem } from "../types";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface TaskLoopRowProps {
  task: TaskListItem;
  loop: TaskLoopProvenance;
  onOpenRun?: () => void;
  profileOwner?: ProfileOwner;
}

/** The runtime models a record's state, not a per-step narrative — that lives on the run page. */
const LOOP_RECORD_DESCRIPTION = "Loop execution record — open the run to act on it.";

/**
 * A revealed Loop execution record in the Tasks listing (US-002.AC-1).
 *
 * Exclusion is default-filtering, never erasure (ADR-001), so a revealed record
 * keeps the work item's row geometry and earns its distinction from two separate
 * channels: structure — the `git-branch` glyph and the neutral role tag — and
 * status — the signal pill in the trail. Identity is plain words end to end; the
 * machine id only ever appears in secondary text.
 */
export function TaskLoopRow({ task, loop, onOpenRun, profileOwner }: TaskLoopRowProps) {
  const runLink = taskLoopRunLink(loop);
  const roleLabel = taskLoopRoleLabel(loop);
  const identity = taskLoopIdentityLabel(loop);

  const name = (
    <ListingRow.Name>
      <ListingRow.Title data-slot="task-loop-row-identity">{identity}</ListingRow.Title>
      <Pill data-slot="task-loop-row-role" size="sm" tone="neutral">
        {roleLabel}
      </Pill>
      {profileOwner ? <ProfileOwnerTag owner={profileOwner} /> : null}
    </ListingRow.Name>
  );

  // Retention removed the run: the record stays and says so, with the id
  // carrying identity. A dead link would be worse than no link (US-002.EC-2).
  if (!runLink) {
    return (
      <ListingRow
        data-slot="task-loop-row"
        data-loop-role={loop.role}
        data-testid={`task-loop-row-${task.id}`}
        interactive={false}
      >
        <ListingRow.Icon>
          <GitBranch aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          {name}
          <ListingRow.Meta data-slot="task-loop-row-meta">
            <MonoId data-slot="task-loop-row-run-id" size="sm" value={loop.run_id} />
          </ListingRow.Meta>
        </ListingRow.Main>
        <ListingRow.Trail data-slot="task-loop-row-trailing">
          <Pill data-testid={`task-loop-row-run-gone-${task.id}`} size="sm" tone="neutral">
            <CircleSlash aria-hidden="true" />
            {RUN_GONE_LABEL}
          </Pill>
        </ListingRow.Trail>
      </ListingRow>
    );
  }

  return (
    <ListingRow
      data-slot="task-loop-row"
      data-loop-role={loop.role}
      data-status={task.status}
      data-testid={`task-loop-row-${task.id}`}
    >
      <ListingRow.Link
        render={
          <Link
            aria-label={`Open run for ${identity}`}
            onClick={onOpenRun}
            params={runLink.params}
            to={runLink.to}
          />
        }
      >
        <ListingRow.Icon>
          <GitBranch aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          {name}
          <ListingRow.Description data-slot="task-loop-row-description">
            {LOOP_RECORD_DESCRIPTION}
          </ListingRow.Description>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail data-slot="task-loop-row-trailing">
        <Pill size="sm" tone={taskStatusTone(task.status)}>
          {taskStatusLabel(task.status)}
        </Pill>
      </ListingRow.Trail>
    </ListingRow>
  );
}
