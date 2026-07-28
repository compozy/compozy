import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockEmptyResponse, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  createNotificationPreset,
  deleteNotificationPreset,
  listNotificationPresets,
  NotificationsApiError,
  updateNotificationPreset,
} from "../notifications-api";

const presetFixture = {
  name: "task_terminal",
  events: ["task.run_*"],
  targets: [{ bridge_id: "bridge_slack_ops", canonical_route: "channel:ops" }],
  filter: "",
  enabled: false,
  built_in: true,
  default_version: "1",
  default_hash: "sha256:default",
  user_modified: false,
  default_update_available: false,
  created_at: "2026-05-21T10:00:00Z",
  updated_at: "2026-05-21T10:00:00Z",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("notificationsApi", () => {
  it("lists notification presets with normalized filters", async () => {
    mockJsonResponse({
      presets: [presetFixture],
      total: 1,
      generated_at: "2026-05-21T10:00:00Z",
    });

    const result = await listNotificationPresets({
      enabled: true,
      built_in: false,
      name: " task_terminal ",
      limit: 10,
    });

    expect(result.presets).toHaveLength(1);
    await expectFetchRequest({
      path: "/api/notifications/presets?enabled=true&built_in=false&name=task_terminal&limit=10",
    });
  });

  it("creates, updates, and deletes presets through daemon-owned routes", async () => {
    mockJsonResponse({ preset: { ...presetFixture, name: "custom_task" } });

    await createNotificationPreset({
      name: "custom_task",
      events: ["task.run_*"],
      targets: [{ bridge_id: "bridge_slack_ops", canonical_route: "channel:ops" }],
      enabled: true,
    });

    await expectFetchRequest({
      method: "POST",
      path: "/api/notifications/presets",
      body: {
        name: "custom_task",
        events: ["task.run_*"],
        targets: [{ bridge_id: "bridge_slack_ops", canonical_route: "channel:ops" }],
        enabled: true,
      },
    });

    mockJsonResponse({ preset: { ...presetFixture, enabled: true } });
    await updateNotificationPreset("task_terminal", { enabled: true });
    await expectFetchRequest({
      callIndex: 1,
      method: "PUT",
      path: "/api/notifications/presets/task_terminal",
      body: { enabled: true },
    });

    mockEmptyResponse({ status: 204 });
    await deleteNotificationPreset("custom_task");
    await expectFetchRequest({
      callIndex: 2,
      method: "DELETE",
      path: "/api/notifications/presets/custom_task",
    });
  });

  it("throws typed errors on failed preset reads", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listNotificationPresets()).rejects.toBeInstanceOf(NotificationsApiError);
    await expect(listNotificationPresets()).rejects.toThrow("Failed to load notification presets");
  });
});
