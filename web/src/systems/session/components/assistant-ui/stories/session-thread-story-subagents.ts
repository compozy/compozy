import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import type { SubagentPayload } from "@/systems/session/adapters/subagent-api";
import { primarySessionFixture } from "@/systems/session/mocks";
import type { SessionPayload, TranscriptMessage } from "@/systems/session/types";

import { transcriptPayload } from "./session-thread-story-transcripts";

/*
 * Subagent scenes (`_uiux.md` S1, S2, S5–S8): the real `SessionThread` over MSW.
 * The transcript carries the daemon's `data-compozy-subagent` parts; the session
 * stream answers with `transcript_snapshot` then `subagents_snapshot`, so every
 * card, the waiting banner and the agents-running count read the production
 * roster path. Board values are fixtures (DESIGN-NOTES item 8).
 */

type TranscriptPart = NonNullable<TranscriptMessage["parts"]>[number];

const LOADED_AT = Date.now();
const secondsAgo = (seconds: number) => new Date(LOADED_AT - seconds * 1_000).toISOString();
const TURN = "turn-story-subagents";

export const subagentStoryIdleSession: SessionPayload = {
  ...primarySessionFixture,
  type: "user",
  archived_at: null,
  activity: null,
  badge: "idle",
  state: "active",
};

export const subagentStoryRunningSession: SessionPayload = {
  ...subagentStoryIdleSession,
  activity: {
    current_tool: "Bash",
    elapsed_ms: 161_000,
    elapsed_seconds: 161,
    idle_seconds: 0,
    iteration_current: 1,
    iteration_max: 1,
    turn_id: TURN,
    turn_started_at: secondsAgo(161),
  },
  badge: "running",
};

function tool(
  name: string,
  id: string,
  input: Record<string, unknown>,
  output: unknown,
  failed = false,
  extra: Record<string, unknown> = {}
): TranscriptPart {
  return {
    ...extra,
    type: `tool-${name}`,
    toolCallId: id,
    state: failed ? "output-error" : "output-available",
    turnId: TURN,
    input,
    ...(failed ? { errorText: String(output) } : { output }),
  } as unknown as TranscriptPart;
}

function card(subagentId: string, toolCallId: string, origin = "delegated"): TranscriptPart {
  return {
    type: "data-compozy-subagent",
    id: subagentId,
    turnId: TURN,
    data: { subagent_id: subagentId, tool_call_id: toolCallId, origin, turn_id: TURN },
  } as unknown as TranscriptPart;
}

function text(value: string, extra: Record<string, unknown> = {}): TranscriptPart {
  return { type: "text", text: value, state: "done", turnId: TURN, ...extra } as TranscriptPart;
}

const delegate = (index: number, title: string) =>
  tool(
    "compozy__subagent_delegate",
    `call-delegate-${index}`,
    { title, task: title, target: { provider: "codex" } },
    { subagent_id: `sub-story-${index}` }
  );

const user = (id: string, value: string): TranscriptMessage => ({
  id,
  role: "user",
  parts: [{ type: "text", text: value, state: "done" }],
});

/** S1/S2/S6: capabilities, three delegations as one group, and the parent's reply. */
export const subagentGroupTranscript: TranscriptMessage[] = [
  user(
    "story_sub_user",
    "Before we cut v2, check the webhook retries, look for N+1 queries in #812, and draft the release notes."
  ),
  {
    id: "story_sub_assistant",
    role: "assistant",
    parts: [
      text(
        "I'll run these in parallel. Each one gets its own subagent so the reviews don't share context."
      ),
      tool("compozy__subagent_capabilities", "call-capabilities", {}, { providers: [] }),
      delegate(1, "Audit payment webhooks for retry safety"),
      delegate(2, "Review PR #812 for N+1 queries"),
      delegate(3, "Draft release notes for v2"),
      card("sub-story-1", "call-delegate-1"),
      card("sub-story-2", "call-delegate-2"),
      card("sub-story-3", "call-delegate-3"),
      text("All three are running. I'll pick up their results as they finish."),
    ],
  },
];

/** S6: the subagent tool family as rows, grouped and failed. */
export const subagentToolsTranscript: TranscriptMessage[] = [
  user("story_tools_user", "Check on the reviewers and cancel the profiling run."),
  {
    id: "story_tools_assistant",
    role: "assistant",
    parts: [
      tool("compozy__subagent_capabilities", "call-cap-1", {}, { providers: [] }),
      tool("compozy__subagent_capabilities", "call-cap-2", {}, { providers: [] }),
      text("Both reviewers are available."),
      tool(
        "compozy__subagent_status",
        "call-status",
        { subagent_id: "sub-story-1" },
        { status: "running" }
      ),
      text("The webhook audit is still running."),
      tool(
        "compozy__subagent_cancel",
        "call-cancel",
        { subagent_id: "sub-story-4" },
        { status: "cancel_requested" }
      ),
      text("Canceled the profiling run."),
      tool(
        "compozy__subagent_delegate",
        "call-delegate-failed",
        { title: "Run the checkout e2e suite", target: { provider: "codex" } },
        "The codex runtime isn't installed in this workspace.",
        true
      ),
      text("Codex isn't installed here, so I couldn't hand off the e2e run."),
    ],
  },
];

/** S8: a provider-native Agent call whose inner work renders inside its card. */
export const subagentNativeTranscript: TranscriptMessage[] = [
  user("story_native_user", "Review the diff with a subagent."),
  {
    id: "story_native_assistant",
    role: "assistant",
    parts: [
      tool(
        "Agent",
        "toolu_story_agent",
        { description: "Review the diff (high effort)", prompt: "Review the diff" },
        "No issues found."
      ),
      card("sub-story-native", "toolu_story_agent", "provider_native"),
      text("Inspecting the diff.", { parentToolCallId: "toolu_story_agent" }),
      tool(
        "Read",
        "toolu_story_read",
        { file_path: "/workspace/README.md" },
        "Project notes",
        false,
        { parentToolCallId: "toolu_story_agent" }
      ),
      text("The reviewer found no issues."),
    ],
  },
];

export function subagentRow(
  id: string,
  title: string,
  overrides: Partial<SubagentPayload> & { elapsed?: number } = {}
): SubagentPayload {
  const { elapsed = 30, ...rest } = overrides;
  const live = !rest.status || ["queued", "running", "waiting"].includes(rest.status);
  return {
    subagent_id: id,
    workspace_id: subagentStoryIdleSession.workspace_id ?? "ws_alpha",
    parent_session_id: subagentStoryIdleSession.id,
    parent_turn_id: TURN,
    child_session_id: `sess-${id}`,
    origin: "delegated",
    provider_tool_call_id: null,
    title,
    role: "general",
    status: "running",
    work_state: live ? "working" : "result_available",
    runtime: {
      agent: "claude",
      provider: "claude",
      model: "Opus 5.5",
      reasoning_effort: "high",
      speed: "normal",
    },
    depth: 1,
    progress: "",
    result: null,
    result_preview: "",
    result_truncated: false,
    error: null,
    wait_timed_out: false,
    delivery: "none",
    created_at: secondsAgo(elapsed),
    started_at: secondsAgo(elapsed),
    settled_at: live ? null : secondsAgo(0),
    updated_at: secondsAgo(0),
    ...rest,
  };
}

const codex = {
  agent: "reviewer",
  provider: "codex",
  model: "gpt-5.6-sol",
  reasoning_effort: "medium",
  speed: "fast",
};

/** Two still working, one done: the group reads `1 working · 1 needs you · 1 done`. */
export const subagentGroupRoster: SubagentPayload[] = [
  subagentRow("sub-story-1", "Audit payment webhooks for retry safety", {
    elapsed: 161,
    progress: "Reading internal/payments/webhook.go",
  }),
  subagentRow("sub-story-2", "Review PR #812 for N+1 queries", {
    elapsed: 98,
    status: "waiting",
    runtime: codex,
  }),
  subagentRow("sub-story-3", "Draft release notes for v2", {
    elapsed: 52,
    status: "completed",
    // Its result waits to wake the parent: the banner glyph breathes (composer VC-01).
    delivery: "pending",
    result_preview: "Drafted notes: 6 user-facing changes, 2 fixes, 1 migration step.",
  }),
];

/** Live, so it stays out of the settled turn's fold and opens inline (VC-05 expanded). */
export const subagentNativeRoster: SubagentPayload[] = [
  subagentRow("sub-story-native", "Review the diff (high effort)", {
    origin: "provider_native",
    child_session_id: null,
    provider_tool_call_id: "toolu_story_agent",
    elapsed: 34,
    progress: "Read /workspace/README.md",
    runtime: {
      agent: "",
      provider: "claude",
      model: "sonnet-5.5",
      reasoning_effort: "",
      speed: "",
    },
  }),
];

const STREAM_ENCODER = new TextEncoder();

/** The parent's stream as the daemon opens it: transcript snapshot, then the roster. */
function subagentStoryStream(
  session: SessionPayload,
  transcript: TranscriptMessage[],
  roster: SubagentPayload[]
): Response {
  const frames = [
    [
      "transcript_snapshot",
      {
        ...transcriptPayload(transcript),
        reset: false,
        session_id: session.id,
        workspace_id: session.workspace_id,
      },
    ],
    ["subagents_snapshot", { session_id: session.id, subagents: roster }],
  ] as const;
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(
        STREAM_ENCODER.encode(
          frames
            .map(([event, data]) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`)
            .join("")
        )
      );
    },
  });
  return new Response(stream, {
    headers: {
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
      "Content-Type": "text/event-stream",
    },
    status: 200,
  });
}

export interface SubagentStoryScene {
  session: SessionPayload;
  transcript: TranscriptMessage[];
  roster: SubagentPayload[];
}

export function subagentStoryHandlers({
  session,
  transcript,
  roster,
}: SubagentStoryScene): HttpHandler[] {
  return [
    compozyApiMock.get("/api/sessions/{session_id}", () => HttpResponse.json({ session })),
    compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", () =>
      HttpResponse.json({ session })
    ),
    compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
      HttpResponse.json(transcriptPayload(transcript))
    ),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/stream",
      ({ response }) => response.untyped(subagentStoryStream(session, transcript, roster))
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/subagents/{subagent_id}/cancel",
      ({ params }) =>
        HttpResponse.json(
          { subagent_id: String(params.subagent_id), status: "cancel_requested" },
          { status: 202 }
        )
    ),
  ];
}
