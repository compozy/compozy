import { describe, expect, it } from "vitest";

import { createAutomationTriggerDraft } from "../automation-drafts";
import {
  availableDataFields,
  filterKeyOptions,
  getEventDef,
  listEventGroups,
} from "../trigger-catalog";
import {
  composeEventId,
  formatEventKind,
  parseEventSelection,
  sampleEventKind,
} from "../trigger-event-id";
import {
  buildTriggerPreview,
  retainValidFilters,
  webhookUrl,
  type JsonRow,
} from "../trigger-preview";
import { buildVariableChips, renderTemplate, tokenizeTemplate } from "../trigger-template";
import type { CreateAutomationTriggerRequest } from "../../types";

function draftFor(
  overrides: Partial<CreateAutomationTriggerRequest> = {}
): CreateAutomationTriggerRequest {
  return { ...createAutomationTriggerDraft("ws_checkout_api"), ...overrides };
}

describe("trigger-catalog", () => {
  it("orders groups canonically and only includes matching events when searched", () => {
    const groups = listEventGroups();
    expect(groups.map(bucket => bucket.group)).toEqual([
      "Session lifecycle",
      "Memory",
      "Hooks",
      "External",
      "Extensions",
    ]);

    const filtered = listEventGroups("webhook");
    expect(filtered).toHaveLength(1);
    expect(filtered[0].group).toBe("External");
    expect(filtered[0].events[0].id).toBe("webhook");
  });

  it("exposes open-payload data fields and envelope filter keys", () => {
    const stopped = getEventDef("session.stopped");
    expect(stopped).toBeDefined();
    expect(availableDataFields(stopped!)).toContain("data.stop_reason");

    const webhook = getEventDef("webhook")!;
    // open-payload events surface the sample's data keys as filter targets.
    expect(availableDataFields(webhook)).toContain("data.payload");

    expect(filterKeyOptions(stopped!).slice(0, 4)).toEqual([
      "kind",
      "scope",
      "source",
      "workspace_id",
    ]);
  });
});

describe("trigger-event-id", () => {
  it("round-trips fixed, hook, ext, and webhook event strings", () => {
    expect(parseEventSelection("session.stopped")).toMatchObject({
      catalogId: "session.stopped",
      family: "fixed",
    });

    const hook = parseEventSelection("hook.transform.completed");
    expect(hook).toMatchObject({
      catalogId: "hook.completed",
      family: "hook",
      hookName: "transform",
    });
    expect(composeEventId(hook)).toBe("hook.transform.completed");

    const ext = parseEventSelection("ext.release.deploy-started");
    expect(ext).toMatchObject({ family: "ext", extExt: "release", extEvent: "deploy-started" });
    expect(composeEventId(ext)).toBe("ext.release.deploy-started");

    const webhook = parseEventSelection("webhook");
    expect(webhook.family).toBe("webhook");
    expect(composeEventId(webhook)).toBe("webhook");
  });

  it("formats placeholders for empty parts but keeps concrete sample kinds", () => {
    const hook = parseEventSelection("hook..completed");
    expect(hook.hookName).toBe("");
    expect(formatEventKind(hook)).toBe("hook.<name>.completed");
    // sample kind falls back to the catalog example when the part is empty.
    expect(sampleEventKind(hook, "hook.transform.completed")).toBe("hook.transform.completed");
    expect(sampleEventKind({ ...hook, hookName: "deploy" }, "hook.transform.completed")).toBe(
      "hook.deploy.completed"
    );
  });

  it("treats unknown runtime events as usable custom events", () => {
    const custom = parseEventSelection("payments.chargeback.spike");
    expect(custom.catalogId).toBe("");
    expect(formatEventKind(custom, "payments.chargeback.spike")).toBe("payments.chargeback.spike");
  });
});

describe("trigger-template", () => {
  it("builds chips from the real envelope roots plus data fields", () => {
    const chips = buildVariableChips(["data.session_id"]);
    expect(chips).toEqual([
      "{{ .Kind }}",
      "{{ .Scope }}",
      "{{ .WorkspaceID }}",
      "{{ .Source }}",
      "{{ .Data.session_id }}",
    ]);
  });

  it("renders resolved variables and flags missing ones", () => {
    const env = {
      kind: "session.stopped",
      scope: "workspace" as const,
      workspace_id: "ws_checkout_api",
      source: "observer" as const,
      data: { session_id: "sess_9f2a1c" },
    };
    const tokens = renderTemplate(
      "Session {{ .Data.session_id }} on {{ .Kind }} ({{ .Data.missing }})",
      env
    );
    expect(tokens).toContainEqual({ id: "var:8", type: "var", value: "sess_9f2a1c" });
    expect(tokens).toContainEqual({ id: "var:34", type: "var", value: "session.stopped" });
    expect(tokens).toContainEqual({ id: "missing:47", type: "missing", name: "Data.missing" });
  });

  it("resolves the `index .Data` template form", () => {
    const env = {
      kind: "webhook",
      scope: "global" as const,
      workspace_id: "",
      source: "webhook" as const,
      data: { "weird.key": "value" },
    };
    const tokens = renderTemplate('{{ index .Data "weird.key" }}', env);
    expect(tokens).toEqual([{ id: "var:0", type: "var", value: "value" }]);
  });

  it("tokenizes the raw template for the tinted source view", () => {
    expect(tokenizeTemplate("hi {{ .Kind }}!")).toEqual([
      { id: "text:0", type: "text", value: "hi " },
      { id: "var:3", type: "var", value: "{{ .Kind }}" },
      { id: "text:14", type: "text", value: "!" },
    ]);
  });
});

describe("buildTriggerPreview", () => {
  it("summarizes the trigger and highlights filtered keys in the sample JSON", () => {
    const draft = draftFor({
      event: "session.stopped",
      scope: "workspace",
      workspace_id: "ws_checkout_api",
      agent_name: "summarizer",
      filter: { "data.stop_reason": "error" },
      prompt: "Session {{ .Data.session_id }} stopped: {{ .Data.stop_reason }}.",
    });

    const preview = buildTriggerPreview(draft, {
      workspaces: [{ id: "ws_checkout_api", name: "checkout-api" }],
    });

    const summaryText = preview.summary.map(segment => segment.text).join("");
    expect(summaryText).toBe(
      "When session.stopped happens in checkout-api, if stop_reason = error, run summarizer."
    );

    expect(preview.matchState).toBe("match");
    expect(preview.matchLabel).toBe("matches this sample");

    const highlighted = preview.json.filter(
      (row): row is Extract<JsonRow, { kind: "pair" }> => row.kind === "pair" && row.highlighted
    );
    expect(highlighted).toHaveLength(1);
    expect(highlighted[0]).toMatchObject({ keyName: "stop_reason", value: "error" });

    expect(preview.rendered).toContainEqual({ id: "var:8", type: "var", value: "sess_9f2a1c" });
    expect(preview.webhook).toBeNull();
  });

  it("reports a non-matching sample when the filter value differs", () => {
    const draft = draftFor({
      event: "session.stopped",
      filter: { "data.stop_reason": "completed" },
    });
    const preview = buildTriggerPreview(draft);
    expect(preview.matchState).toBe("nomatch");
    expect(preview.matchLabel).toBe("won't fire on this sample");
  });

  it("fires on every event with no conditions", () => {
    const preview = buildTriggerPreview(draftFor({ event: "session.created", filter: {} }));
    expect(preview.matchState).toBe("all");
    expect(preview.matchLabel).toBe("fires on every event");
  });

  it("builds the real webhook endpoint and curl for webhook triggers", () => {
    const draft = draftFor({
      event: "webhook",
      scope: "global",
      workspace_id: undefined,
      endpoint_slug: "ci-deploys",
      webhook_id: "wbh_a1b2c3",
    });
    const preview = buildTriggerPreview(draft);
    expect(preview.webhook?.url).toBe("/api/webhooks/global/ci-deploys--wbh_a1b2c3");
    const curlText =
      preview.webhook?.curl
        .flat()
        .map(segment => segment.text)
        .join("") ?? "";
    expect(curlText).toContain("X-AGH-Webhook-Signature: sha256=…");
    expect(curlText).toContain("X-AGH-Webhook-Timestamp: …");
    // summary uses "globally" for webhooks, never a workspace.
    expect(preview.summary.map(segment => segment.text).join("")).toContain("globally");
  });

  it("derives the reliability badge from retry, fire limit, and enabled state", () => {
    expect(
      buildTriggerPreview(
        draftFor({
          retry: { strategy: "backoff", max_retries: 2, base_delay: "5s" },
          fire_limit: { max: 4, window: "1h" },
          enabled: true,
        })
      ).reliabilityBadge
    ).toBe("Backoff ×2 · 4/1h · enabled");

    expect(
      buildTriggerPreview(
        draftFor({ retry: { strategy: "none", max_retries: 0, base_delay: "" }, enabled: false })
      ).reliabilityBadge
    ).toContain("No retry");
  });
});

describe("retainValidFilters", () => {
  it("drops keys that the new fixed event does not expose", () => {
    const stopped = getEventDef("session.stopped")!;
    const kept = retainValidFilters(
      { "data.stop_reason": "error", "data.unknown_path": "x", kind: "session.stopped" },
      stopped
    );
    expect(kept).toEqual({ "data.stop_reason": "error", kind: "session.stopped" });
  });

  it("keeps arbitrary data paths for open-payload events", () => {
    const webhook = getEventDef("webhook")!;
    const kept = retainValidFilters({ "data.anything": "1", scope: "global" }, webhook);
    expect(kept).toEqual({ "data.anything": "1", scope: "global" });
  });
});

describe("webhookUrl", () => {
  it("falls back to placeholders before slug and id are filled", () => {
    expect(webhookUrl(draftFor({ event: "webhook" }))).toBe("/api/webhooks/global/slug--wbh_id");
  });
});
