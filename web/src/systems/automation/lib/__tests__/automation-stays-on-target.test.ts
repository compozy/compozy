import { describe, expect, it } from "vitest";

import type { AgentInstancePayload } from "@/systems/agent";

import {
  buildAutomationFormRequest,
  createAutomationFormDraft,
  setAutomationFormDoes,
  setAutomationFormStart,
} from "../automation-form-draft";
import { staysOnTargetModel } from "../automation-stays-on-target";

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
      isWebhook: false,
    });
    expect(model.agents).toEqual([{ name: "triage", label: "Triage Lead" }]);
    expect(model.scopes.map(scope => scope.label)).toEqual(["Global", "alpha"]);
    expect(model.selectedScope).toBe("ws_alpha");
    expect(model.status).toEqual({ kind: "ready", label: "Triage Lead" });
  });

  it("reports a missing instance when the agent does not run in that scope", () => {
    const model = staysOnTargetModel({
      instances: [GLOBAL],
      agent: "triage",
      workspaceId: "ws_alpha",
      isWebhook: false,
    });
    expect(model.selectedScope).toBeNull();
    expect(model.status).toEqual({ kind: "missing", agent: "triage", scope: "this project" });
  });

  it("limits a link start to Global instances", () => {
    const model = staysOnTargetModel({
      instances: [instance({}), GLOBAL],
      agent: "triage",
      workspaceId: "ws_alpha",
      isWebhook: true,
    });
    expect(model.selectedScope).toBe("");
    expect(
      model.scopes.find(scope => scope.workspaceId === "ws_alpha")?.disabledReason
    ).toBeTruthy();
  });

  it("says when events wait for a resume or no check will read them", () => {
    const paused = staysOnTargetModel({
      instances: [instance({ state: "paused" })],
      agent: "triage",
      workspaceId: "ws_alpha",
      isWebhook: false,
    });
    expect(paused.status.kind).toBe("paused");
    const unchecked = staysOnTargetModel({
      instances: [instance({ checks_enabled: false })],
      agent: "triage",
      workspaceId: "ws_alpha",
      isWebhook: false,
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
