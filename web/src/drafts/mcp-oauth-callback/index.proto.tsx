import "@/drafts/mcp-oauth-callback/callback.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

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
    <div className="cb-stage">
      <div className="cb-browser">
        <div className="cb-browser__bar">
          <div className="cb-browser__lights" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
          <span className="cb-browser__url">
            http://<b>127.0.0.1:2123</b>/api/mcp/oauth/callback?code=…&amp;state=…
          </span>
        </div>
        <CallbackPage key={state} scene={SCENES[state] ?? SCENES.connected} />
      </div>
    </div>
  );
}
