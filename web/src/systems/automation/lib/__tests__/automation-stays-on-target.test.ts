import { describe, expect, it } from "vitest";

import type { AgentInstancePayload } from "@/systems/agent";

import {
  buildAutomationFormRequest,
  createAutomationFormDraft,
  setAutomationFormDoes,
  setAutomationFormStart,
} from "../automation-form-draft";
import { staysOnTargetModel } from "../automation-stays-on-target";
import { webhookUrl } from "../trigger-preview";

function instance(overrides: Partial<AgentInstancePayload>): AgentInstancePayload {
  return {
    id: "ain_1",
    agent_name: "triage",
    label: "Triage Lead",
    provider: "claude",
    resolved_appearance: {} as AgentInstancePayload["resolved_appearance"],
    profile_id: "p1",
    profile_name: "default",
    workspace_id: "ws_alpha",
    scope: "alpha",
    state: "active",
    activity: "idle",
    now: "",
    needs_attention: false,
    pause_reason: "" as AgentInstancePayload["pause_reason"],
    main_session_id: "sess_1",
    checks_enabled: true,
    last_wake_at: "",
    next_wake_at: "",
    quiet_streak: 0,
    failure_streak: 0,
    fatigue_limit: 3,
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

const GLOBAL = instance({ id: "ain_2", workspace_id: "", scope: "@user" });

// UT-061: the trigger editor's stays-on target and its missing-instance error.
describe("staysOnTargetModel", () => {
  it("resolves the instance in the trigger's own scope", () => {
    const model = staysOnTargetModel({
      instances: [instance({}), GLOBAL],
      agent: "triage",
      workspaceId: "ws_alpha",
    });
    expect(model.agents).toEqual([{ name: "triage", label: "Triage Lead" }]);
    expect(model.scopes.map(scope => scope.label)).toEqual(["Global", "alpha"]);
    expect(model.selectedScope).toBe("ws_alpha");
    expect(model.status).toEqual({ kind: "ready", label: "Triage Lead" });
  });

  it("tells agents that share a display name apart by their name", () => {
    const model = staysOnTargetModel({
      instances: [instance({}), instance({ id: "ain_3", agent_name: "triage-web" })],
      agent: "",
      workspaceId: "ws_alpha",
    });
    expect(model.agents.map(option => option.label)).toEqual([
      "Triage Lead (triage)",
      "Triage Lead (triage-web)",
    ]);
  });

  it("reports a missing instance when the agent does not run in that scope", () => {
    const model = staysOnTargetModel({
      instances: [GLOBAL],
      agent: "triage",
      workspaceId: "ws_alpha",
    });
    expect(model.selectedScope).toBeNull();
    expect(model.status).toEqual({ kind: "missing", agent: "triage", scope: "this project" });
  });

  it("says when events wait for a resume or no check will read them", () => {
    const paused = staysOnTargetModel({
      instances: [instance({ state: "paused" })],
      agent: "triage",
      workspaceId: "ws_alpha",
    });
    expect(paused.status.kind).toBe("paused");
    const unchecked = staysOnTargetModel({
      instances: [instance({ checks_enabled: false })],
      agent: "triage",
      workspaceId: "ws_alpha",
    });
    expect(unchecked.status.kind).toBe("no-checks");
  });
});

describe("stays-on trigger request", () => {
  it("saves an event trigger with the agent_instance target and its scope", () => {
    const draft = setAutomationFormDoes(
      { ...createAutomationFormDraft("ws_alpha", { start: "event" }), agent_name: "triage" },
      "stays-on",
      "ws_alpha"
    );
    const request = buildAutomationFormRequest({ ...draft, prompt: "Which deploy failed?" });
    expect(request.entity).toBe("trigger");
    expect(request.create).toMatchObject({
      target_kind: "agent_instance",
      target_instance_scope: "",
      agent_name: "triage",
    });
  });

  it("keeps a link's project when it tells a stays-on agent there", () => {
    const draft = setAutomationFormDoes(
      {
        ...createAutomationFormDraft("ws_alpha", { start: "webhook" }),
        agent_name: "triage",
        endpoint_slug: "deploy-failed",
        webhook_id: "wbh_1",
      },
      "stays-on",
      "ws_alpha"
    );
    const request = buildAutomationFormRequest({ ...draft, prompt: "Which deploy failed?" });
    expect(request.create).toMatchObject({
      event: "webhook",
      scope: "workspace",
      workspace_id: "ws_alpha",
      target_kind: "agent_instance",
    });
    expect(webhookUrl(request.create as Parameters<typeof webhookUrl>[0])).toBe(
      "/api/webhooks/workspaces/ws_alpha/deploy-failed--wbh_1"
    );
    // Any other link still lives in User.
    const plain = buildAutomationFormRequest(
      setAutomationFormDoes({ ...draft, prompt: "Summarize." }, "agent", "ws_alpha")
    );
    expect(plain.create).toMatchObject({ scope: "global" });
    expect(plain.create).not.toHaveProperty("workspace_id", "ws_alpha");
  });

  it("falls back to Ask an agent when the start becomes a schedule", () => {
    const draft = setAutomationFormDoes(
      createAutomationFormDraft("ws_alpha", { start: "event" }),
      "stays-on",
      "ws_alpha"
    );
    const scheduled = setAutomationFormStart(draft, "schedule");
    expect(scheduled.target_kind).toBe("agent");
    expect(scheduled.target_instance_scope).toBeUndefined();
  });
});
