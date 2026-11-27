import { definePrototype, type PrototypeProps } from "@protohq/cli";

import {
  allReadyFixture,
  codexFixture,
  failedFixture,
  limitedFixture,
  manyFixture,
  reloadFailedFixture,
  runtimeMixedFixture,
  runtimeOffFixture,
  signedInFixture,
  signInFixture,
  waitingFixture,
} from "./fixtures";
import { ComposerStage, type ComposerStageProps } from "./stage";

export const meta = definePrototype({
  title: "MCP redesign · Composer popover",
  route: "/sessions/sess-mcp",
  states: [
    { id: "proposed", label: "Proposed · Claude Code servers folded" },
    { id: "today", label: "Today · Shipped popover" },
    { id: "runtime-open", label: "Proposed · Claude Code servers opened" },
    { id: "sign-in-waiting", label: "Sign in · Waiting for the browser" },
    { id: "sign-in-starting", label: "Sign in · Opening the browser" },
    { id: "sign-in-paste", label: "Sign in · Browser on another computer" },
    { id: "sign-in-finishing", label: "Sign in · Finishing" },
    { id: "sign-in-connected", label: "Sign in · Connected" },
    { id: "sign-in-failed", label: "Sign in · Didn't finish" },
    { id: "sign-in-scopes", label: "Sign in · Asks for more access" },
    { id: "after-sign-in", label: "After sign-in · Joins on the next message" },
    { id: "allow", label: "Project server · Review before allowing" },
    { id: "issue", label: "Not working · Why" },
    { id: "reload-failed", label: "Couldn't start with the new servers" },
    { id: "limited", label: "Off in this session (parent limits)" },
    { id: "runtime-mixed", label: "Claude Code servers · One waits in Claude Code" },
    { id: "runtime-off", label: "Claude Code servers · Turned off" },
    { id: "all-ready", label: "All ready" },
    { id: "stale", label: "May be out of date" },
    { id: "loading", label: "Loading" },
    { id: "many", label: "50 servers" },
    { id: "codex", label: "Codex session" },
    { id: "closed", label: "Trigger only" },
  ],
});

const SIGN_IN = "branas-dashboard";

function propsFor(state: string): ComposerStageProps {
  switch (state) {
    case "today":
      return { today: true, data: signInFixture };
    case "runtime-open":
      return { data: signInFixture, runtimeOpen: true };
    case "sign-in-waiting":
      return {
        data: signInFixture,
        nested: { kind: "sign-in", server: SIGN_IN, phase: "waiting" },
      };
    case "sign-in-starting":
      return {
        data: signInFixture,
        nested: { kind: "sign-in", server: SIGN_IN, phase: "starting" },
      };
    case "sign-in-paste":
      return { data: signInFixture, nested: { kind: "sign-in", server: SIGN_IN, phase: "paste" } };
    case "sign-in-finishing":
      return {
        data: signInFixture,
        nested: { kind: "sign-in", server: SIGN_IN, phase: "finishing" },
      };
    case "sign-in-connected":
      return {
        data: signInFixture,
        nested: { kind: "sign-in", server: SIGN_IN, phase: "connected" },
      };
    case "sign-in-failed":
      return {
        data: signInFixture,
        nested: {
          kind: "sign-in",
          server: SIGN_IN,
          phase: "failed",
          error: "The sign-in link expired before you finished.",
        },
      };
    case "sign-in-scopes":
      return {
        data: signInFixture,
        nested: {
          kind: "sign-in",
          server: SIGN_IN,
          phase: "scopes",
          scopes: [
            { id: "finance:read", label: "Read finance reports" },
            { id: "crm:write", label: "Create and edit CRM deals" },
          ],
        },
      };
    case "after-sign-in":
      return { data: signedInFixture };
    case "allow":
      return { data: waitingFixture, nested: { kind: "allow", server: "postgres-local" } };
    case "issue":
      return { data: failedFixture, nested: { kind: "issue", server: "pagerduty" } };
    case "reload-failed":
      return { data: reloadFailedFixture };
    case "limited":
      return { data: limitedFixture };
    case "runtime-mixed":
      return { data: runtimeMixedFixture, runtimeOpen: true };
    case "runtime-off":
      return { data: runtimeOffFixture, runtimeOpen: true };
    case "all-ready":
      return { data: allReadyFixture };
    case "stale":
      return { data: allReadyFixture, stale: true };
    case "loading":
      return { data: null, loading: true };
    case "many":
      return { data: manyFixture };
    case "codex":
      return { data: codexFixture };
    case "closed":
      return { data: signInFixture, open: false };
    default:
      return { data: signInFixture };
  }
}

export default function MCPComposerRedesignDraft({ state }: PrototypeProps) {
  return <ComposerStage key={state} {...propsFor(state)} />;
}
