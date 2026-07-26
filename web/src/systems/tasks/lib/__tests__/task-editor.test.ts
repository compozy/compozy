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
  taskEditorDraftFromTask,
} from "../task-editor";
import type { TaskOwnerKind } from "../../types";
import { buildTaskExecutionProfileFixture, taskDetailFixture } from "../../mocks/fixtures";
import { buildLiveNetworkParticipationFixture } from "@/test/network-participation-fixtures";

describe("taskEditorDraftFromTask", () => {
  it("Should hydrate future-run intent only from the authoritative execution profile", () => {
    const task = {
      ...taskDetailFixture.task,
      resolved_network_participation: buildLiveNetworkParticipationFixture({
        channelId: "resolved-run-room",
        workspaceId: taskDetailFixture.task.workspace_id ?? "ws_test",
      }),
    };
    const profile = buildTaskExecutionProfileFixture({
      task_id: task.id,
      network_participation: {
        mode: "live",
        channel_id: "authored-future-room",
        channel_strategy: "named",
      },
    });

    const draft = taskEditorDraftFromTask(task, profile);

    expect(draft.networkParticipationMode).toBe("live");
    expect(draft.networkChannelId).toBe("authored-future-room");
    expect(draft.networkChannelStrategy).toBe("named");
  });
});

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
    expect(payload.network_participation).toEqual({ mode: "local" });
    expect("network_channel" in payload).toBe(false);
    expect("channel" in payload).toBe(false);
    expect("coordination_channel_id" in payload).toBe(false);
    expect(payload.identifier).toBeUndefined();
    expect("parent_task_id" in payload).toBe(false);
  });
});

describe("buildCreateChildTaskRequest", () => {
  it("builds the child-task payload without serializing network_channel", () => {
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
    expect(payload.network_participation).toEqual({ mode: "local" });
    expect("network_channel" in payload).toBe(false);
    expect("channel" in payload).toBe(false);
    expect("coordination_channel_id" in payload).toBe(false);
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

  it.each<TaskOwnerKind>([
    "pool",
    "agent_session",
    "human",
    "automation",
    "extension",
    "network_peer",
  ])("preserves the %s owner union when ownership is not cleared", ownerKind => {
    const payload = buildUpdateTaskRequest({
      ...EMPTY_TASK_EDITOR_DRAFT,
      title: "Keep explicit ownership",
      ownerKind,
      ownerRef: "owner-ref",
    });

    expect(payload.owner).toEqual({ kind: ownerKind, ref: "owner-ref" });
    expect(payload).not.toHaveProperty("clear_owner");
  });
});
