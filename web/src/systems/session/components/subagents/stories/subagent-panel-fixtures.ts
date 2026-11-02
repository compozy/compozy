import type { SubagentStatus, SubagentView } from "../types";
import { NO_RUNTIME, subagentFixture } from "./subagent-story-fixtures";

interface NativeOptions {
  id: string;
  title: string;
  model: string;
  role: string;
  status: SubagentStatus;
  elapsed: number;
  progress?: string;
  tools?: number;
  tokens?: number;
  result?: string;
  error?: string;
}

function native(options: NativeOptions): SubagentView {
  const { model, tools, tokens, result, error, ...row } = options;
  return subagentFixture({
    ...row,
    origin: "provider_native",
    child_session_id: null,
    runtime: { ...NO_RUNTIME, agent: "general", provider: "claude", model },
    usage: tools === undefined ? undefined : { tool_uses: tools, total_tokens: tokens ?? 0 },
    result: result ?? null,
    result_preview: result ?? "",
    error: error ?? null,
  });
}

const MAP = native({
  id: "map",
  title: "Map always-on capabilities against spec-cycle",
  model: "sonnet",
  role: "Explore",
  status: "running",
  elapsed: 352,
  progress: "Running git grep -n always_on internal/session",
  tools: 41,
  tokens: 38_200,
});

const REVIEW = subagentFixture({
  id: "review",
  title: "Review session rebuild for data loss",
  status: "running",
  elapsed: 138,
  progress: "Reading internal/session/rebuild_replay.go",
  runtime: {
    ...NO_RUNTIME,
    agent: "reviewer",
    provider: "codex",
    model: "gpt-6-astra",
    reasoning_effort: "xhigh",
  },
});

const BROWSER = native({
  id: "browser",
  title: "Capture the Kanban in light and dark",
  model: "haiku",
  role: "general-purpose",
  status: "queued",
  elapsed: 0,
});

const E2E = native({
  id: "e2e",
  title: "Run the session rebuild end-to-end suite",
  model: "sonnet",
  role: "general-purpose",
  status: "failed",
  elapsed: 64,
  tools: 9,
  tokens: 6_400,
  error: "Playwright couldn't start: chromium_headless_shell-1243 is missing.",
});

const FINISHED = [
  native({
    id: "usage",
    title: "Measure spec-cycle usage across sessions",
    model: "opus",
    role: "general-purpose",
    status: "completed",
    elapsed: 236,
    tools: 28,
    tokens: 44_100,
    result: "The delivery half went unused in **31 of 34 runs**.",
  }),
  native({
    id: "issues",
    title: "Analyze issue orchestration sessions",
    model: "sonnet",
    role: "Explore",
    status: "completed",
    elapsed: 299,
    tools: 36,
    tokens: 29_300,
    result: "Orchestrators spend 70% of their tool calls polling CI.",
  }),
];

export const WORKING_FLEET = [MAP, REVIEW, BROWSER, ...FINISHED];
export const FAILED_FLEET = [E2E, ...WORKING_FLEET];

/** A big fan-out: the header carries the group's shape, the rows page. */
export const CROWDED_FLEET = Array.from({ length: 12 }, (_, index) =>
  native({
    id: `audit-${index}`,
    title: `Audit surface ${index + 1} for stale copy`,
    model: index % 3 === 2 ? "haiku" : "sonnet",
    role: "Explore",
    status: index < 7 ? "running" : index === 7 ? "failed" : "completed",
    elapsed: 70 + index * 63,
    progress: "Reading internal/api/core/handlers.go",
    tools: 6 + index * 2,
    tokens: 4_200 + index * 1_000,
    error: index === 7 ? "Stopped: the turn hit its token limit." : undefined,
  })
);
