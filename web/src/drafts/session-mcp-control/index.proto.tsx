import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  ALL_READY,
  CODEX_SESSION,
  FAILED,
  LIMITED,
  MANY,
  NEEDS_SIGN_IN,
  PENDING,
  RELOAD_FAILED,
  RUNTIME_OFF,
  STALE,
  WAITING,
} from "./fixtures";
import { SessionMCPStage, type SessionStageProps } from "./stage";

export const meta = definePrototype({
  title: "Session MCP control",
  spec: "mcp-extension-hot-activation",
  surface: "S3 S4",
  route: "/sessions/sess-mcp",
  states: [
    { id: "ready", label: "S3 · All ready" },
    { id: "closed", label: "S3 · Trigger only" },
    { id: "pending", label: "S3 · Change applies on the next message" },
    { id: "sign-in", label: "S3 · Needs sign-in" },
    { id: "waiting", label: "S3 · Waiting for approval" },
    { id: "failed", label: "S3 · Couldn't connect" },
    { id: "reload-failed", label: "S3 · Couldn't start with the new servers" },
    { id: "limited", label: "S3 · Limited by the parent session" },
    { id: "runtime-off", label: "S3 · Claude Code servers off" },
    { id: "codex", label: "S3 · Codex session" },
    { id: "stale", label: "S3 · Status may be out of date" },
    { id: "loading", label: "S3 · Loading" },
    { id: "many", label: "S3 · 50 servers" },
    { id: "line-applied", label: "S4 · MCP servers updated" },
    { id: "line-replayed", label: "S4 · Continued from history" },
    { id: "line-failed", label: "S4 · Couldn't start, message not sent" },
  ],
});

function propsFor(state: string): SessionStageProps {
  switch (state) {
    case "closed":
      return { model: ALL_READY, open: false };
    case "pending":
      return { model: PENDING };
    case "sign-in":
      return { model: NEEDS_SIGN_IN };
    case "waiting":
      return { model: WAITING };
    case "failed":
      return { model: FAILED };
    case "reload-failed":
      return { model: RELOAD_FAILED };
    case "limited":
      return { model: LIMITED };
    case "runtime-off":
      return { model: RUNTIME_OFF };
    case "codex":
      return { model: CODEX_SESSION };
    case "stale":
      return { model: STALE };
    case "loading":
      return { model: ALL_READY, loading: true };
    case "many":
      return { model: MANY };
    case "line-applied":
      return { model: ALL_READY, open: false, line: "applied" };
    case "line-replayed":
      return { model: ALL_READY, open: false, line: "replayed" };
    case "line-failed":
      return { model: RELOAD_FAILED, open: false, line: "failed" };
    default:
      return { model: ALL_READY };
  }
}

export default function SessionMCPControlDraft({ state }: PrototypeProps) {
  return <SessionMCPStage key={state} {...propsFor(state)} />;
}
