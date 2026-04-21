import { http, HttpResponse, type HttpHandler } from "msw";

import type {
  ArchiveResult,
  SyncResult,
  TaskBoardPayload,
  TaskDetailPayload,
  WorkflowSummary,
} from "../types";
import {
  taskBoardFixture,
  taskDetailFixture,
  workflowArchiveResultFixture,
  workflowSyncResultFixture,
  workflowsFixture,
} from "./fixtures";

export interface WorkflowHandlerOptions {
  workflows?: WorkflowSummary[];
  board?: TaskBoardPayload;
  task?: TaskDetailPayload;
  syncResult?: SyncResult;
  archiveResult?: ArchiveResult;
}

export function createWorkflowHandlers(options: WorkflowHandlerOptions = {}): HttpHandler[] {
  const workflows = options.workflows ?? workflowsFixture;
  const board = options.board ?? taskBoardFixture;
  const task = options.task ?? taskDetailFixture;
  const syncResult = options.syncResult ?? workflowSyncResultFixture;
  const archiveResult = options.archiveResult ?? workflowArchiveResultFixture;

  return [
    http.get("/api/tasks", () => HttpResponse.json({ workflows })),
    http.post("/api/sync", () => HttpResponse.json(syncResult)),
    http.post("/api/tasks/:slug/archive", () => HttpResponse.json(archiveResult)),
    http.get("/api/tasks/:slug/board", () => HttpResponse.json({ board })),
    http.get("/api/tasks/:slug/items/:task_id", () => HttpResponse.json({ task })),
  ];
}

export const handlers = createWorkflowHandlers();
