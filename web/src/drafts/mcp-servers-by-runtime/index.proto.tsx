import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { mcpDefinitionKey } from "@/systems/settings/lib/mcp-management-target";
import {
  claudeOffRuntimeMCPFixture,
  claudeRuntimeMCPFixture,
  codexRuntimeMCPFixture,
  manyRuntimeMCPFixture,
  opencodeUnreadableRuntimeMCPFixture,
  piRuntimeMCPFixture,
  runtimeMCPInventoryFixture,
} from "@/systems/settings/mocks/runtime-mcp-fixtures";
import type { SettingsMCPServerEntry } from "@/systems/settings/types";

import { MCPServersByRuntimePage, type MCPPageProps } from "./mcp-page";
import { COMPOZY_SERVERS } from "./mcp-page-fixtures";

const DEFAULT_RUNTIMES = runtimeMCPInventoryFixture().runtimes;

/** The first project-scoped server, waiting for approval as a project file declared it. */
function withWaiting(servers: readonly SettingsMCPServerEntry[]): SettingsMCPServerEntry[] {
  let marked = false;
  return servers.map(server => {
    if (marked || server.scope !== "workspace") return server;
    marked = true;
    return {
      ...server,
      runtime_status: null,
      callable_in_sessions: false,
      approval: { required: true, state: "waiting", definition_digest: "sha256:3f9c" },
    };
  });
}

function waitingKey(servers: readonly SettingsMCPServerEntry[]): string {
  const waiting = servers.find(server => server.approval.state === "waiting");
  return waiting ? mcpDefinitionKey(waiting) : "";
}

export const meta = definePrototype({
  title: "MCP servers by runtime",
  spec: "mcp-extension-hot-activation",
  surface: "S1 S2",
  route: "/settings/mcp",
  states: [
    { id: "overview", label: "S1 · CompozyOS and every coding agent" },
    { id: "loading", label: "S1 · Loading" },
    { id: "filter-codex", label: "S1 · Filtered to Codex" },
    { id: "waiting", label: "S1 · Project server waiting for approval" },
    { id: "changed", label: "S1 · Changed since allowed" },
    { id: "runtime-off", label: "S1 · Claude Code servers off in CompozyOS sessions" },
    { id: "unreadable", label: "S1 · OpenCode file unreadable" },
    { id: "folded", label: "S1 · 24 servers folded" },
    { id: "compozy-empty", label: "S1 · No CompozyOS servers yet" },
    { id: "empty", label: "S1 · Nothing anywhere" },
    { id: "no-match", label: "S1 · Search with no match" },
    { id: "copy", label: "S2 · Copy to CompozyOS" },
    { id: "copy-name-taken", label: "S2 · Name already taken" },
    { id: "copy-source-changed", label: "S2 · Source changed" },
    { id: "copy-submitting", label: "S2 · Copying" },
    { id: "copied", label: "S2 · Copied, row highlighted" },
  ],
});

function propsFor(state: string): MCPPageProps {
  const page = { compozy: COMPOZY_SERVERS, runtimes: DEFAULT_RUNTIMES };
  const waiting = withWaiting(COMPOZY_SERVERS);
  switch (state) {
    case "loading":
      return { ...page, loading: true };
    case "filter-codex":
      return { ...page, filter: "codex" };
    case "waiting":
      return { ...page, compozy: waiting };
    case "changed":
      return {
        ...page,
        compozy: waiting,
        approvalNotes: { [waitingKey(waiting)]: { reason: "changed" } },
      };
    case "runtime-off":
      return {
        ...page,
        runtimes: [claudeOffRuntimeMCPFixture, codexRuntimeMCPFixture, piRuntimeMCPFixture],
      };
    case "unreadable":
      return {
        ...page,
        runtimes: [
          claudeRuntimeMCPFixture,
          opencodeUnreadableRuntimeMCPFixture,
          codexRuntimeMCPFixture,
        ],
      };
    case "folded":
      return {
        ...page,
        filter: "cursor",
        runtimes: [manyRuntimeMCPFixture(24), codexRuntimeMCPFixture],
      };
    case "compozy-empty":
      return { ...page, compozy: [] };
    case "empty":
      return { compozy: [], runtimes: [] };
    case "no-match":
      return { ...page, query: "kubernetes" };
    case "copy":
      return { ...page, copy: "default" };
    case "copy-name-taken":
      return { ...page, copy: "name-taken" };
    case "copy-source-changed":
      return { ...page, copy: "source-changed" };
    case "copy-submitting":
      return { ...page, copy: "submitting" };
    case "copied": {
      const exa: SettingsMCPServerEntry = {
        ...COMPOZY_SERVERS[0]!,
        name: "exa",
        transport: "http",
        scope: "user",
        workspace_id: undefined,
      };
      return { ...page, compozy: [...COMPOZY_SERVERS, exa], copied: exa };
    }
    default:
      return page;
  }
}

export default function MCPServersByRuntimeDraft({ state }: PrototypeProps) {
  return <MCPServersByRuntimePage key={state} {...propsFor(state)} />;
}
