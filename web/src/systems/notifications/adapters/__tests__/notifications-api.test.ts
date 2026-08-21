import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockEmptyResponse, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  createNotificationPreset,
  deleteNotificationPreset,
  listNotificationPresets,
  NotificationsApiError,
  setNotificationPresetEnablement,
  updateNotificationPreset,
} from "../notifications-api";

const presetFixture = {
  name: "task_terminal",
  profile: "marketing",
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
      profile: " marketing ",
      name: " task_terminal ",
      limit: 10,
    });

    expect(result.presets).toHaveLength(1);
    await expectFetchRequest({
      path: "/api/notifications/presets?enabled=true&built_in=false&profile=marketing&name=task_terminal&limit=10",
    });
  });

  it("creates and updates presets through profile-scoped daemon routes", async () => {
    mockJsonResponse({ preset: { ...presetFixture, name: "custom_task" } });

    await createNotificationPreset(
      {
        name: "custom_task",
        events: ["task.run_*"],
        targets: [{ bridge_id: "bridge_slack_ops", canonical_route: "channel:ops" }],
      },
      "marketing"
    );

    await expectFetchRequest({
      method: "POST",
      path: "/api/notifications/presets?profile=marketing",
      body: {
        name: "custom_task",
        events: ["task.run_*"],
        targets: [{ bridge_id: "bridge_slack_ops", canonical_route: "channel:ops" }],
      },
    });

    mockJsonResponse({ preset: { ...presetFixture, enabled: true } });
    await updateNotificationPreset("task_terminal", { filter: "outcome >= warning" }, "marketing");
    await expectFetchRequest({
      callIndex: 1,
      method: "PUT",
      path: "/api/notifications/presets/task_terminal?profile=marketing",
      body: { filter: "outcome >= warning" },
    });
  });

  it("deletes a custom preset from the shared library", async () => {
    mockEmptyResponse({ status: 204 });
    await deleteNotificationPreset("custom_task");
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/notifications/presets/custom_task",
    });
  });

  it("sets effective enablement for one profile through the dedicated authority", async () => {
    mockJsonResponse({ name: "task_terminal", profile: "marketing", enabled: false });

    const result = await setNotificationPresetEnablement("task_terminal", {
      profile: "marketing",
      enabled: false,
    });

    expect(result).toEqual({ name: "task_terminal", profile: "marketing", enabled: false });

    await expectFetchRequest({
      method: "PUT",
      path: "/api/notifications/presets/task_terminal/enablement",
      body: { profile: "marketing", enabled: false },
    });
  });

  it("throws typed errors on failed preset reads", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listNotificationPresets()).rejects.toBeInstanceOf(NotificationsApiError);
    await expect(listNotificationPresets()).rejects.toThrow("Failed to load notification presets");
  });
});
