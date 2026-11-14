import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { COMPOZY_SERVERS, MCPServersByRuntimePage, type MCPPageProps } from "./mcp-page";
import {
  CLAUDE,
  CLAUDE_OFF,
  CODEX,
  CURSOR_MANY,
  DEFAULT_RUNTIMES,
  OPENCODE_UNREADABLE,
  PI,
} from "./runtime-fixtures";

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
  switch (state) {
    case "loading":
      return { ...page, loading: true };
    case "filter-codex":
      return { ...page, filter: "codex" };
    case "waiting":
      return { ...page, approvals: { sentry: "waiting" } };
    case "changed":
      return { ...page, approvals: { sentry: "changed" } };
    case "runtime-off":
      return { ...page, runtimes: [CLAUDE_OFF, CODEX, PI] };
    case "unreadable":
      return { ...page, runtimes: [CLAUDE, OPENCODE_UNREADABLE, CODEX] };
    case "folded":
      return { ...page, filter: "cursor", runtimes: [CURSOR_MANY, CODEX] };
    case "compozy-empty":
      return { ...page, compozy: [] };
    case "empty":
      return {
        compozy: [],
        runtimes: DEFAULT_RUNTIMES.map(group => ({ ...group, servers: [] })),
      };
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
    case "copied":
      return {
        ...page,
        compozy: [...COMPOZY_SERVERS, { ...COMPOZY_SERVERS[0]!, name: "exa", transport: "http" }],
        copied: "exa",
      };
    default:
      return page;
  }
}

export default function MCPServersByRuntimeDraft({ state }: PrototypeProps) {
  return <MCPServersByRuntimePage key={state} {...propsFor(state)} />;
}
