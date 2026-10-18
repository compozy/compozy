// Suite: task editor request projection
// Invariant: editor drafts project explicit owner intent without stale union fields.
// Boundary IN: pure task editor draft-to-request builders.
// Boundary OUT: React controls, HTTP transport, and daemon persistence.

import { describe, expect, it } from "vitest";

import {
  buildCreateChildTaskRequest,
  buildCreateTaskRequest,
  buildUpdateTaskRequest,
  EMPTY_TASK_EDITOR_DRAFT,
} from "../task-editor";
import type { TaskOwnerKind } from "../../types";

describe("buildCreateTaskRequest", () => {
  it("builds the root-task payload without a parent task id", () => {
    const payload = buildCreateTaskRequest(
      {
        ...EMPTY_TASK_EDITOR_DRAFT,
        title: "Standalone task",
        workspaceId: "ws_signalforge",
      },
      {
        asDraft: false,
        templateId: "one_shot",
      }
    );

    expect(payload.workspace).toBe("ws_signalforge");
    expect(payload.identifier).toBeUndefined();
    expect("parent_task_id" in payload).toBe(false);
  });
});

describe("buildCreateChildTaskRequest", () => {
  it("builds the child-task payload with its identifier", () => {
    const payload = buildCreateChildTaskRequest(
      {
        ...EMPTY_TASK_EDITOR_DRAFT,
        title: "Web child creation probe 0425",
        description: "Probe task",
        scope: "workspace",
        workspaceId: "ws_signalforge",
        parentTaskId: " task-44a84096bb3e51ea ",
        identifier: " WEB-CHILD-0425 ",
      },
      {
        asDraft: false,
        templateId: "one_shot",
      }
    );

    expect(payload.workspace).toBe("ws_signalforge");
    expect(payload.identifier).toBe("WEB-CHILD-0425");
    expect("parent_task_id" in payload).toBe(false);
  });
});

describe("buildUpdateTaskRequest", () => {
  it("projects Unassigned as the authoritative clear operation without a stale owner", () => {
    const payload = buildUpdateTaskRequest({
      ...EMPTY_TASK_EDITOR_DRAFT,
      title: "Release queued work",
      ownerKind: "",
      ownerRef: "sess-stale-owner",
    });

    expect(payload.clear_owner).toBe(true);
    expect(payload).not.toHaveProperty("owner");
  });

  it.each<TaskOwnerKind>(["pool", "agent_session", "human", "automation", "extension"])(
    "preserves the %s owner union when ownership is not cleared",
    ownerKind => {
      const payload = buildUpdateTaskRequest({
        ...EMPTY_TASK_EDITOR_DRAFT,
        title: "Keep explicit ownership",
        ownerKind,
        ownerRef: "owner-ref",
      });

      expect(payload.owner).toEqual({ kind: ownerKind, ref: "owner-ref" });
      expect(payload).not.toHaveProperty("clear_owner");
    }
  );
});
