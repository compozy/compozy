import "@/drafts/mcp-oauth-callback/callback.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { BrowserFrame } from "@/drafts/mcp-oauth-callback/browser-frame";
import { CallbackPage } from "@/drafts/mcp-oauth-callback/callback-page";
import { SCENES } from "@/drafts/mcp-oauth-callback/scenes";

export const meta = definePrototype({
  title: "MCP sign-in callback page",
  surface: "GET /api/mcp/oauth/callback",
  states: [
    { id: "connected", label: "Connected, one project" },
    { id: "connected-all-projects", label: "Connected, all projects" },
    { id: "connected-long", label: "Connected, long name and many scopes" },
    { id: "declined", label: "Declined on the provider" },
    { id: "expired", label: "Link expired or already used" },
    { id: "changed", label: "Server changed during sign-in" },
    { id: "unavailable", label: "Runtime not ready" },
    { id: "failed", label: "Failed, technical details" },
  ],
});

export default function Prototype({ state }: PrototypeProps) {
  return (
    <BrowserFrame>
      <CallbackPage key={state} scene={SCENES[state] ?? SCENES.connected} />
    </BrowserFrame>
  );
}
