import { definePrototype, type PrototypeProps } from "@protohq/cli";

import {
  sessionMCPAllReadyFixture,
  sessionMCPCodexFixture,
  sessionMCPFailedFixture,
  sessionMCPLimitedFixture,
  sessionMCPManyFixture,
  sessionMCPNeedsSignInFixture,
  sessionMCPPendingFixture,
  sessionMCPReloadAppliedFixture,
  sessionMCPReloadFailedFixture,
  sessionMCPReloadFailedLineFixture,
  sessionMCPReloadReplayedFixture,
  sessionMCPRuntimeOffFixture,
  sessionMCPWaitingFixture,
} from "@/systems/session/mocks/session-mcp-fixtures";

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
      return { data: sessionMCPAllReadyFixture, open: false };
    case "pending":
      return { data: sessionMCPPendingFixture };
    case "sign-in":
      return { data: sessionMCPNeedsSignInFixture };
    case "waiting":
      return { data: sessionMCPWaitingFixture };
    case "failed":
      return { data: sessionMCPFailedFixture };
    case "reload-failed":
      return { data: sessionMCPReloadFailedFixture };
    case "limited":
      return { data: sessionMCPLimitedFixture };
    case "runtime-off":
      return { data: sessionMCPRuntimeOffFixture };
    case "codex":
      return { data: sessionMCPCodexFixture };
    case "stale":
      return { data: sessionMCPAllReadyFixture, stale: true };
    case "loading":
      return { data: sessionMCPAllReadyFixture, loading: true };
    case "many":
      return { data: sessionMCPManyFixture };
    case "line-applied":
      return {
        data: sessionMCPAllReadyFixture,
        open: false,
        reload: sessionMCPReloadAppliedFixture,
      };
    case "line-replayed":
      return {
        data: sessionMCPAllReadyFixture,
        open: false,
        reload: sessionMCPReloadReplayedFixture,
      };
    case "line-failed":
      return {
        data: sessionMCPReloadFailedFixture,
        open: false,
        reload: sessionMCPReloadFailedLineFixture,
      };
    default:
      return { data: sessionMCPAllReadyFixture };
  }
}

export default function SessionMCPControlDraft({ state }: PrototypeProps) {
  return <SessionMCPStage key={state} {...propsFor(state)} />;
}
