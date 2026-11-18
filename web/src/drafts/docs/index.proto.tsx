import "../landing/landing.css";
import "./docs.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { LazyMotion, domMax } from "motion/react";

import { SiteFooter } from "../landing/sections-close";
import { DocsShell } from "./shell";
import type { DocsView } from "./view";

export const meta = definePrototype({
  title: "Docs redesign",
  spec: "landing-redesign",
  surface: "compozy.com/docs",
  states: [
    { id: "home", label: "Docs home" },
    { id: "tutorial", label: "Tutorial with steps" },
    { id: "concept", label: "Concept with diagram" },
    { id: "hub", label: "Section hub" },
    { id: "workflow", label: "Worked example" },
    { id: "reference", label: "Reference with wide tables" },
    { id: "config", label: "Longest page" },
    { id: "cli", label: "CLI command" },
    { id: "cli-group", label: "CLI command group" },
    { id: "api", label: "API resource" },
    { id: "sdk", label: "SDK overview" },
    { id: "sdk-client", label: "SDK client guide" },
    { id: "sdk-resource", label: "SDK resource" },
    { id: "sdk-go", label: "SDK resource in Go" },
    { id: "search", label: "Search" },
    { id: "short", label: "Short page" },
  ],
});

const CLOSED = { query: null, menu: false, lang: "ts" as const };

const START: Record<string, DocsView> = {
  home: { ...CLOSED, slug: "" },
  tutorial: { ...CLOSED, slug: "getting-started/quick-start" },
  concept: { ...CLOSED, slug: "tools" },
  hub: { ...CLOSED, slug: "sessions" },
  workflow: { ...CLOSED, slug: "examples/webhook-to-agent-run" },
  reference: { ...CLOSED, slug: "agents/providers" },
  config: { ...CLOSED, slug: "configuration/config-toml" },
  cli: { ...CLOSED, slug: "cli/session/prompt" },
  "cli-group": { ...CLOSED, slug: "cli/session" },
  api: { ...CLOSED, slug: "api/sessions" },
  sdk: { ...CLOSED, slug: "sdk" },
  "sdk-client": { ...CLOSED, slug: "sdk/prompt" },
  "sdk-resource": { ...CLOSED, slug: "sdk/sessions" },
  "sdk-go": { ...CLOSED, slug: "sdk/sessions", lang: "go" },
  search: { ...CLOSED, slug: "", query: "webhook" },
  short: { ...CLOSED, slug: "use-cases/handoff-between-agents" },
};

export default function Prototype({ state }: PrototypeProps) {
  return (
    <LazyMotion features={domMax}>
      <div id="landing" className="dark bg-[#0a0a0a] font-sans text-fg antialiased">
        <main>
          <DocsShell key={state} initialView={START[state] ?? { ...CLOSED, slug: "" }} />
        </main>
        <SiteFooter />
      </div>
    </LazyMotion>
  );
}
