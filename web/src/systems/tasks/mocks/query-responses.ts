import type {
  TaskInboxItem,
  TaskInboxLane,
  TaskInboxView,
  TaskListItem,
  TaskListPage,
} from "../types";

const DEFAULT_PAGE_LIMIT = 50;
const MAX_PAGE_LIMIT = 200;
const TASK_CURSOR_PREFIX = "mock-task:";
const INBOX_CURSOR_PREFIX = "mock-inbox:";

const INBOX_LANES: TaskInboxLane[] = ["my_work", "approvals", "failed_runs", "blocked", "archived"];

function queryText(url: URL, name: string): string {
  return url.searchParams.get(name)?.trim() ?? "";
}

function queryLimit(url: URL): number {
  const raw = queryText(url, "limit");
  if (raw === "") {
    return DEFAULT_PAGE_LIMIT;
  }

  const limit = Number(raw);
  if (!Number.isInteger(limit) || limit < 1 || limit > MAX_PAGE_LIMIT) {
    throw new Error(`Tasks mock limit must be between 1 and ${MAX_PAGE_LIMIT}`);
  }
  return limit;
}

function matchesScope(
  item: { scope: "global" | "workspace"; workspace_id?: string },
  url: URL
): boolean {
  const scope = queryText(url, "scope") || "all";
  const workspace = queryText(url, "workspace");

  if (scope === "global") {
    return item.scope === "global";
  }
  if (scope === "workspace") {
    return item.scope === "workspace" && (workspace === "" || item.workspace_id === workspace);
  }
  if (workspace !== "" && item.scope === "workspace") {
    return item.workspace_id === workspace;
  }
  return true;
}

function priorityRank(priority: TaskListItem["priority"]): number {
  switch (priority) {
    case "urgent":
      return 4;
    case "high":
      return 3;
    case "medium":
      return 2;
    case "low":
      return 1;
    default:
      return 0;
  }
}

function compareTaskActivity(left: TaskListItem, right: TaskListItem): number {
  const leftActivity = left.last_activity_at ?? left.updated_at;
  const rightActivity = right.last_activity_at ?? right.updated_at;
  const activityOrder = rightActivity.localeCompare(leftActivity);
  return activityOrder === 0 ? right.id.localeCompare(left.id) : activityOrder;
}

function compareCatalogTasks(left: TaskListItem, right: TaskListItem, sort: string): number {
  if (sort === "priority") {
    const rankOrder = priorityRank(right.priority) - priorityRank(left.priority);
    if (rankOrder !== 0) {
      return rankOrder;
    }
  }
  return compareTaskActivity(left, right);
}

function matchesCatalogTask(task: TaskListItem, url: URL): boolean {
  if (!matchesScope(task, url)) {
    return false;
  }

  const status = queryText(url, "status");
  const includeDrafts = queryText(url, "include_drafts") === "true";
  if (status === "" && !includeDrafts && task.status === "draft") {
    return false;
  }
  if (status !== "" && task.status !== status) {
    return false;
  }

  const priority = queryText(url, "priority");
  const approvalState = queryText(url, "approval_state");
  const ownerKind = queryText(url, "owner_kind");
  const ownerRef = queryText(url, "owner_ref");
  const parentTaskId = queryText(url, "parent_task_id");
  const participationChannel = queryText(url, "participation_channel");
  const search = queryText(url, "query").toLowerCase();

  if (priority !== "" && task.priority !== priority) {
    return false;
  }
  if (approvalState !== "" && task.approval_state !== approvalState) {
    return false;
  }
  if (ownerKind !== "" && task.owner?.kind !== ownerKind) {
    return false;
  }
  if (ownerRef !== "" && task.owner?.ref !== ownerRef) {
    return false;
  }
  if (parentTaskId !== "" && task.parent_task_id !== parentTaskId) {
    return false;
  }
  if (participationChannel !== "") {
    const participation = task.resolved_network_participation;
    if (participation?.mode !== "live" || participation.channel_id !== participationChannel) {
      return false;
    }
  }
  if (
    search !== "" &&
    !task.title.toLowerCase().includes(search) &&
    !task.identifier?.toLowerCase().includes(search)
  ) {
    return false;
  }
  return true;
}

function catalogFacets(tasks: TaskListItem[]): TaskListPage["facets"] {
  const statusCounts = new Map<TaskListItem["status"], number>();
  const ownerCounts = new Map<
    string,
    { count: number; owner: NonNullable<TaskListItem["owner"]> }
  >();

  for (const task of tasks) {
    statusCounts.set(task.status, (statusCounts.get(task.status) ?? 0) + 1);
    if (task.owner) {
      const key = JSON.stringify([task.owner.kind, task.owner.ref]);
      const current = ownerCounts.get(key);
      ownerCounts.set(key, {
        count: (current?.count ?? 0) + 1,
        owner: task.owner,
      });
    }
  }

  return {
    owners: [...ownerCounts.values()].sort((left, right) => {
      const kindOrder = left.owner.kind.localeCompare(right.owner.kind);
      return kindOrder === 0 ? left.owner.ref.localeCompare(right.owner.ref) : kindOrder;
    }),
    statuses: [...statusCounts.entries()]
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([status, count]) => ({ count, status })),
  };
}

function cursorOffset<T>(
  items: T[],
  cursor: string,
  prefix: string,
  identity: (item: T) => string
) {
  if (cursor === "") {
    return 0;
  }
  if (!cursor.startsWith(prefix)) {
    throw new Error("Tasks mock cursor is invalid");
  }

  const cursorIdentity = cursor.slice(prefix.length);
  const index = items.findIndex(item => identity(item) === cursorIdentity);
  if (index < 0) {
    throw new Error("Tasks mock cursor does not belong to this query");
  }
  return index + 1;
}

export function buildTaskCatalogResponse(tasks: TaskListItem[], url: URL): TaskListPage {
  const filtered = tasks
    .filter(task => matchesCatalogTask(task, url))
    .sort((left, right) => compareCatalogTasks(left, right, queryText(url, "sort") || "recent"));
  const limit = queryLimit(url);
  const offset = cursorOffset(
    filtered,
    queryText(url, "cursor"),
    TASK_CURSOR_PREFIX,
    task => task.id
  );
  const pageTasks = filtered.slice(offset, offset + limit);
  const hasMore = offset + pageTasks.length < filtered.length;
  const lastTask = pageTasks.at(-1);

  return {
    facets: catalogFacets(filtered),
    page: {
      has_more: hasMore,
      limit,
      ...(hasMore && lastTask ? { next_cursor: `${TASK_CURSOR_PREFIX}${lastTask.id}` } : {}),
      total: filtered.length,
    },
    tasks: pageTasks,
  };
}

function matchesInboxItem(item: TaskInboxItem, url: URL): boolean {
  if (!matchesScope(item.task, url)) {
    return false;
  }

  const ownerKind = queryText(url, "owner_kind");
  const ownerRef = queryText(url, "owner_ref");
  const lane = queryText(url, "lane");
  const status = queryText(url, "status");
  const priority = queryText(url, "priority");
  const unread = queryText(url, "unread");
  const search = queryText(url, "query").toLowerCase();

  if (ownerKind !== "" && item.task.owner?.kind !== ownerKind) {
    return false;
  }
  if (ownerRef !== "" && item.task.owner?.ref !== ownerRef) {
    return false;
  }
  if (lane !== "" && item.lane !== lane) {
    return false;
  }
  if (status !== "" && item.task.status !== status) {
    return false;
  }
  if (priority !== "" && item.task.priority !== priority) {
    return false;
  }
  if (unread !== "" && !item.triage.read !== (unread === "true")) {
    return false;
  }
  if (
    search !== "" &&
    !item.task.title.toLowerCase().includes(search) &&
    !item.task.identifier?.toLowerCase().includes(search)
  ) {
    return false;
  }
  return true;
}

function compareInboxItems(left: TaskInboxItem, right: TaskInboxItem): number {
  const unreadOrder = Number(!right.triage.read) - Number(!left.triage.read);
  if (unreadOrder !== 0) {
    return unreadOrder;
  }
  const activityOrder = right.latest_activity_at.localeCompare(left.latest_activity_at);
  if (activityOrder !== 0) {
    return activityOrder;
  }
  const priorityOrder = priorityRank(right.task.priority) - priorityRank(left.task.priority);
  return priorityOrder === 0 ? left.task.id.localeCompare(right.task.id) : priorityOrder;
}

function inboxLanes(url: URL): TaskInboxLane[] {
  const lane = queryText(url, "lane");
  return INBOX_LANES.filter(candidate => lane === "" || candidate === lane);
}

function inboxFacets(items: TaskInboxItem[]): TaskInboxView["facets"] {
  const priorityCounts = new Map<NonNullable<TaskInboxItem["task"]["priority"]>, number>();
  const statusCounts = new Map<TaskInboxItem["task"]["status"], number>();

  for (const item of items) {
    statusCounts.set(item.task.status, (statusCounts.get(item.task.status) ?? 0) + 1);
    if (item.task.priority) {
      priorityCounts.set(item.task.priority, (priorityCounts.get(item.task.priority) ?? 0) + 1);
    }
  }

  return {
    priorities: [...priorityCounts.entries()]
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([priority, count]) => ({ count, priority })),
    statuses: [...statusCounts.entries()]
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([status, count]) => ({ count, status })),
  };
}

export function buildTaskInboxResponse(source: TaskInboxView, url: URL): TaskInboxView {
  const filtered = source.groups
    .flatMap(group => group.items ?? [])
    .filter(item => matchesInboxItem(item, url))
    .sort(compareInboxItems);
  const limit = queryLimit(url);
  const offset = cursorOffset(
    filtered,
    queryText(url, "cursor"),
    INBOX_CURSOR_PREFIX,
    item => item.task.id
  );
  const pageItems = filtered.slice(offset, offset + limit);
  const hasMore = offset + pageItems.length < filtered.length;
  const lastItem = pageItems.at(-1);

  return {
    archived_total: filtered.filter(item => item.lane === "archived").length,
    facets: inboxFacets(filtered),
    groups: inboxLanes(url).map(lane => {
      const allLaneItems = filtered.filter(item => item.lane === lane);
      const laneItems = pageItems.filter(item => item.lane === lane);
      return {
        count: allLaneItems.length,
        ...(laneItems.length > 0 ? { items: laneItems } : {}),
        lane,
        unread_count: allLaneItems.filter(item => !item.triage.read).length,
      };
    }),
    page: {
      has_more: hasMore,
      limit,
      ...(hasMore && lastItem ? { next_cursor: `${INBOX_CURSOR_PREFIX}${lastItem.task.id}` } : {}),
      total: filtered.length,
    },
    unread_total: filtered.filter(item => !item.triage.read).length,
  };
}
