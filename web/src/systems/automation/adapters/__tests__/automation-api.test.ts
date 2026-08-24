import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockEmptyResponse, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  AutomationApiError,
  createAutomationJob,
  createAutomationTrigger,
  deleteAutomationJob,
  deleteAutomationTrigger,
  getAutomationJob,
  getAutomationTrigger,
  listAutomationJobRuns,
  listAutomationJobs,
  listAutomationRuns,
  listAutomationTriggerRuns,
  listAutomationTriggers,
  triggerAutomationJob,
  updateAutomationJob,
  updateAutomationTrigger,
} from "@/systems/automation/adapters/automation-api";
import {
  acceptAutomationSuggestion,
  dismissAutomationSuggestion,
  listAutomationSuggestions,
} from "@/systems/automation/adapters/automation-suggestions-api";

const jobFixture = {
  id: "job_daily_review",
  name: "daily-review",
  agent_name: "reviewer",
  prompt: "Review recent changes.",
  scope: "workspace" as const,
  workspace_id: "ws_alpha",
  source: "dynamic" as const,
  enabled: true,
  schedule: { mode: "cron" as const, expr: "0 9 * * *" },
  retry: { strategy: "none" as const, max_retries: 3, base_delay: "2s" },
  fire_limit: { max: 12, window: "1h" },
  next_run: "2026-04-12T09:00:00Z",
  created_at: "2026-04-11T09:00:00Z",
  updated_at: "2026-04-11T09:05:00Z",
};

const triggerFixture = {
  id: "trg_push_review",
  name: "push-review",
  agent_name: "reviewer",
  prompt: "Review push event {{ .Data.branch }}.",
  event: "ext.github.push",
  filter: { "data.branch": "main" },
  scope: "workspace" as const,
  workspace_id: "ws_alpha",
  source: "dynamic" as const,
  enabled: true,
  retry: { strategy: "backoff" as const, max_retries: 4, base_delay: "5s" },
  fire_limit: { max: 12, window: "1h" },
  endpoint_slug: "push-review",
  webhook_id: "wbh_push_review",
  created_at: "2026-04-11T08:00:00Z",
  updated_at: "2026-04-11T08:10:00Z",
};

const runFixture = {
  id: "run_001",
  status: "running" as const,
  attempt: 1,
  job_id: "job_daily_review",
  session_id: "sess_001",
  started_at: "2026-04-11T10:00:00Z",
};

const suggestionFixture = {
  id: "suggestion_daily_review",
  workspace_id: "ws_alpha",
  source: "catalog",
  dedup_key: "daily-review",
  status: "pending",
  payload: { ...jobFixture, target_kind: "prompt" },
  created_at: "2026-04-11T09:00:00Z",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("listAutomationJobs", () => {
  it("forwards every stable job filter, package source, enabled state, and cursor", async () => {
    mockJsonResponse({
      jobs: [jobFixture],
      page: { has_more: true, limit: 10, next_cursor: "job-cursor-2", total: 12 },
    });

    const result = await listAutomationJobs({
      cursor: " job-cursor-1 ",
      enabled: false,
      loop: " release-loop ",
      q: " review ",
      scope: "workspace",
      workspace_id: "ws_alpha",
      profile: " marketing ",
      all_profiles: false,
      source: "package",
      limit: 10,
    });

    expect(result.jobs).toEqual([jobFixture]);
    expect(result.page.total).toBe(12);
    await expectFetchRequest({
      path: "/api/automation/jobs?scope=workspace&workspace_id=ws_alpha&source=package&enabled=false&q=review&cursor=job-cursor-1&limit=10&loop=release-loop&profile=marketing&all_profiles=false",
    });
  });

  it("passes abort signal to fetch", async () => {
    mockJsonResponse({ jobs: [] });

    const controller = new AbortController();
    await listAutomationJobs({ scope: "global" }, controller.signal);

    await expectFetchRequest({
      path: "/api/automation/jobs?scope=global",
      signal: controller.signal,
    });
  });

  it("throws AutomationApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listAutomationJobs()).rejects.toThrow(AutomationApiError);
    await expect(listAutomationJobs()).rejects.toThrow("Failed to fetch automation jobs: 500");
  });
});

describe("automation suggestions", () => {
  it("lists the complete status-filtered response envelope with an abort signal", async () => {
    const response = { suggestions: [suggestionFixture] };
    mockJsonResponse(response);
    const controller = new AbortController();

    const result = await listAutomationSuggestions("ws_alpha", "pending", controller.signal);

    expect(result).toEqual(response);
    await expectFetchRequest({
      path: "/api/workspaces/ws_alpha/automation/suggestions?status=pending",
      signal: controller.signal,
    });
  });

  it("accepts a suggestion and returns both the resolved suggestion and created job", async () => {
    const response = {
      suggestion: { ...suggestionFixture, status: "accepted" },
      job: jobFixture,
    };
    mockJsonResponse(response);

    await expect(
      acceptAutomationSuggestion("ws_alpha", "suggestion_daily_review")
    ).resolves.toEqual(response);
    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/automation/suggestions/suggestion_daily_review/accept",
    });
  });

  it("dismisses a suggestion without creating a job", async () => {
    const response = {
      suggestion: { ...suggestionFixture, status: "dismissed" },
    };
    mockJsonResponse(response);

    await expect(
      dismissAutomationSuggestion("ws_alpha", "suggestion_daily_review")
    ).resolves.toEqual(response);
    await expectFetchRequest({
      method: "POST",
      path: "/api/workspaces/ws_alpha/automation/suggestions/suggestion_daily_review/dismiss",
    });
  });

  it("reports a specific list failure", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(listAutomationSuggestions("ws_alpha", "pending")).rejects.toThrow(
      "Failed to fetch automation suggestions: 503"
    );
  });
});

describe("createAutomationJob", () => {
  it("posts the generated request body and returns the created job", async () => {
    mockJsonResponse({ job: jobFixture }, { status: 201 });

    const body = {
      name: "daily-review",
      agent_name: "reviewer",
      prompt: "Review recent changes.",
      scope: "workspace" as const,
      workspace_id: "ws_alpha",
      enabled: true,
      schedule: { mode: "cron" as const, expr: "0 9 * * *" },
      retry: { strategy: "none" as const, max_retries: 3, base_delay: "2s" },
      fire_limit: { max: 12, window: "1h" },
    };

    const result = await createAutomationJob(body);

    expect(result).toEqual(jobFixture);
    await expectFetchRequest({
      body,
      method: "POST",
      path: "/api/automation/jobs",
    });
  });

  it("serializes a normalized profile owner when creating a job", async () => {
    mockJsonResponse({ job: jobFixture }, { status: 201 });

    await createAutomationJob(
      {
        name: "daily-review",
        agent_name: "reviewer",
        prompt: "Review recent changes.",
        scope: "workspace",
        workspace_id: "ws_alpha",
        enabled: true,
        schedule: { mode: "cron", expr: "0 9 * * *" },
        retry: { strategy: "none", max_retries: 3, base_delay: "2s" },
        fire_limit: { max: 12, window: "1h" },
      },
      " growth "
    );

    await expectFetchRequest({
      method: "POST",
      path: "/api/automation/jobs?profile=growth",
    });
  });
});

describe("job detail endpoints", () => {
  it("gets one automation job by id", async () => {
    mockJsonResponse({ job: jobFixture });

    const result = await getAutomationJob("job_daily_review", { profile: "marketing" });

    expect(result).toEqual(jobFixture);
    await expectFetchRequest({
      path: "/api/automation/jobs/job_daily_review?profile=marketing",
    });
  });

  it("patches one job and returns the updated record", async () => {
    mockJsonResponse({ job: { ...jobFixture, enabled: false } });

    const result = await updateAutomationJob("job_daily_review", { enabled: false }, " marketing ");

    expect(result.enabled).toBe(false);
    await expectFetchRequest({
      body: { enabled: false },
      method: "PATCH",
      path: "/api/automation/jobs/job_daily_review?profile=marketing",
    });
  });

  it("deletes one job and supports abort signals", async () => {
    mockEmptyResponse({ status: 204 });

    const controller = new AbortController();
    await deleteAutomationJob("job_daily_review", "marketing", controller.signal);

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/automation/jobs/job_daily_review?profile=marketing",
      signal: controller.signal,
    });
  });

  it("throws a not-found error for missing jobs", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getAutomationJob("missing", { profile: "marketing" })).rejects.toThrow(
      "Automation job not found: missing"
    );
    await expect(deleteAutomationJob("missing", "marketing")).rejects.toThrow(
      "Automation job not found: missing"
    );
  });
});

describe("triggerAutomationJob", () => {
  it("posts to the trigger endpoint and returns the queued run", async () => {
    mockJsonResponse({ run: runFixture });

    const result = await triggerAutomationJob("job_daily_review", "marketing");

    expect(result).toEqual(runFixture);
    await expectFetchRequest({
      method: "POST",
      path: "/api/automation/jobs/job_daily_review/trigger?profile=marketing",
    });
  });
});

describe("listAutomationTriggers", () => {
  it("forwards every stable trigger filter, enabled state, event, and cursor", async () => {
    mockJsonResponse({
      page: { has_more: false, limit: 5, total: 1 },
      triggers: [triggerFixture],
    });

    const result = await listAutomationTriggers({
      cursor: " trigger-cursor-1 ",
      enabled: true,
      scope: "workspace",
      workspace_id: "ws_alpha",
      event: "ext.github.push",
      loop: " release-loop ",
      q: " main ",
      profile: " marketing ",
      all_profiles: false,
      source: "package",
      limit: 5,
    });

    expect(result.triggers).toEqual([triggerFixture]);
    expect(result.page.total).toBe(1);
    await expectFetchRequest({
      path: "/api/automation/triggers?scope=workspace&workspace_id=ws_alpha&source=package&enabled=true&event=ext.github.push&q=main&cursor=trigger-cursor-1&limit=5&loop=release-loop&profile=marketing&all_profiles=false",
    });
  });
});

describe("trigger detail endpoints", () => {
  it("gets one automation trigger by id", async () => {
    mockJsonResponse({ trigger: triggerFixture });

    const result = await getAutomationTrigger("trg_push_review", { all_profiles: true });

    expect(result).toEqual(triggerFixture);
    await expectFetchRequest({
      path: "/api/automation/triggers/trg_push_review?all_profiles=true",
    });
  });

  it("creates one trigger and returns the created record", async () => {
    mockJsonResponse({ trigger: triggerFixture }, { status: 201 });

    const body = {
      name: "push-review",
      agent_name: "reviewer",
      prompt: "Review push event {{ .Data.branch }}.",
      event: "webhook",
      filter: { "data.branch": "main" },
      scope: "workspace" as const,
      workspace_id: "ws_alpha",
      enabled: true,
      retry: { strategy: "backoff" as const, max_retries: 4, base_delay: "5s" },
      fire_limit: { max: 12, window: "1h" },
      endpoint_slug: "push-review",
      webhook_id: "wbh_push_review",
    };

    const result = await createAutomationTrigger(body);

    expect(result).toEqual(triggerFixture);
    await expectFetchRequest({
      body,
      method: "POST",
      path: "/api/automation/triggers",
    });
  });

  it("serializes a normalized profile owner when creating a trigger", async () => {
    mockJsonResponse({ trigger: triggerFixture }, { status: 201 });

    await createAutomationTrigger(
      {
        name: "push-review",
        agent_name: "reviewer",
        prompt: "Review push event {{ .Data.branch }}.",
        event: "webhook",
        filter: { "data.branch": "main" },
        scope: "workspace",
        workspace_id: "ws_alpha",
        enabled: true,
        retry: { strategy: "backoff", max_retries: 4, base_delay: "5s" },
        fire_limit: { max: 12, window: "1h" },
        endpoint_slug: "push-review",
        webhook_id: "wbh_push_review",
      },
      " growth "
    );

    await expectFetchRequest({
      method: "POST",
      path: "/api/automation/triggers?profile=growth",
    });
  });

  it("deletes one trigger", async () => {
    mockEmptyResponse({ status: 204 });

    await deleteAutomationTrigger("trg_push_review", "marketing");

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/automation/triggers/trg_push_review?profile=marketing",
    });
  });

  it("throws a not-found error for missing triggers", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getAutomationTrigger("missing", { profile: "marketing" })).rejects.toThrow(
      "Automation trigger not found: missing"
    );
    await expect(deleteAutomationTrigger("missing", "marketing")).rejects.toThrow(
      "Automation trigger not found: missing"
    );
  });
});

describe("updateAutomationTrigger", () => {
  it("patches one trigger and returns the updated record", async () => {
    mockJsonResponse({ trigger: { ...triggerFixture, enabled: false } });

    const result = await updateAutomationTrigger(
      "trg_push_review",
      {
        enabled: false,
        webhook_secret_value: "next-secret",
      },
      " marketing "
    );

    expect(result.enabled).toBe(false);
    await expectFetchRequest({
      body: { enabled: false, webhook_secret_value: "next-secret" },
      method: "PATCH",
      path: "/api/automation/triggers/trg_push_review?profile=marketing",
    });
  });
});

describe("run history endpoints", () => {
  it("maps job run history from /api/automation/jobs/:id/runs", async () => {
    mockJsonResponse({ runs: [runFixture] });

    const result = await listAutomationJobRuns("job_daily_review", {
      status: "running",
      limit: 3,
      profile: " marketing ",
      all_profiles: false,
    });

    expect(result).toEqual([runFixture]);
    await expectFetchRequest({
      path: "/api/automation/jobs/job_daily_review/runs?status=running&limit=3&profile=marketing&all_profiles=false",
    });
  });

  it("maps trigger run history from /api/automation/triggers/:id/runs", async () => {
    mockJsonResponse({
      runs: [{ ...runFixture, trigger_id: "trg_push_review", job_id: undefined }],
    });

    const result = await listAutomationTriggerRuns("trg_push_review", {
      status: "running",
      limit: 2,
      all_profiles: true,
    });

    expect(result[0]?.trigger_id).toBe("trg_push_review");
    await expectFetchRequest({
      path: "/api/automation/triggers/trg_push_review/runs?status=running&limit=2&all_profiles=true",
    });
  });

  it("maps global run history from /api/automation/runs and normalizes optional filters", async () => {
    mockJsonResponse({ runs: [runFixture] });

    const result = await listAutomationRuns({
      job_id: " job_daily_review ",
      trigger_id: " ",
      status: "running",
      since: " 2026-04-11T09:00:00Z ",
      until: "",
      limit: 10,
      profile: " marketing ",
    });

    expect(result).toEqual([runFixture]);
    await expectFetchRequest({
      path: "/api/automation/runs?job_id=job_daily_review&status=running&since=2026-04-11T09%3A00%3A00Z&limit=10&profile=marketing",
    });
  });
});

describe("profile-scoped automation mutations", () => {
  it.each([
    {
      name: "update job",
      execute: () => updateAutomationJob("job_daily_review", { enabled: false }, "   "),
    },
    { name: "delete job", execute: () => deleteAutomationJob("job_daily_review", "   ") },
    { name: "trigger job", execute: () => triggerAutomationJob("job_daily_review", "   ") },
    {
      name: "update trigger",
      execute: () => updateAutomationTrigger("trg_push_review", { enabled: false }, "   "),
    },
    {
      name: "delete trigger",
      execute: () => deleteAutomationTrigger("trg_push_review", "   "),
    },
  ])("rejects a missing owner before $name", async ({ execute }) => {
    await expect(execute()).rejects.toMatchObject({
      message: "Automation profile is required",
      status: 400,
    });
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });
});

describe("AutomationApiError", () => {
  it("stores the status code on the thrown error", () => {
    const error = new AutomationApiError("boom", 422);

    expect(error.name).toBe("AutomationApiError");
    expect(error.message).toBe("boom");
    expect(error.status).toBe(422);
  });
});
